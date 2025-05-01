package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var rootCmd = &ffcli.Command{
	ShortUsage: "govbox <subcommand> <path>",
	ShortHelp:  "Set of commands for GovGen proposals.",
	Subcommands: []*ffcli.Command{
		tallyCmd(), accountsCmd(), genesisCmd(), autoStakingCmd(),
		distributionCmd(), top20Cmd(), propJSONCmd(),
		signTxCmd(), vestingCmd(),
		tallyGenesisCmd(), shrinkVotesCmd(), gnoAirdropCmd(),
	},
	Exec: func(ctx context.Context, args []string) error {
		return flag.ErrHelp
	},
}

func main() {
	err := rootCmd.ParseAndRun(context.Background(), os.Args[1:])
	if err != nil && err != flag.ErrHelp {
		log.Fatal(err)
	}
}

func shrinkVotesCmd() *ffcli.Command {
	return &ffcli.Command{
		Name:       "shrink-votes",
		ShortUsage: "govbox shrink-votes <genesis.json> <high>",
		ShortHelp:  "Outputs a genesis where only the first <high> votes are kept from <genesis.json>",
		Exec: func(ctx context.Context, args []string) error {
			if len(args) != 2 {
				return flag.ErrHelp
			}
			high, err := strconv.Atoi(args[1])
			if err != nil {
				return err
			}
			return shrinkVotes(ctx, args[0], high)
		},
	}
}

func tallyGenesisCmd() *ffcli.Command {
	fs := flag.NewFlagSet("tallyGenesis", flag.ContinueOnError)
	numVals := fs.Int("numVals", 1, "number of validators")
	numDels := fs.Int("numDels", 0, "number of delegators")
	numGovs := fs.Int("numGovs", 0, "number of governors")
	nodeAddr := fs.String("nodeAddr", "", "bech32 address of the validator node that will run the genesis")
	nodeConsPubkey := fs.String("nodeConsPubkey", "", "consensus pubkey of the validator node that will run the genesis")
	return &ffcli.Command{
		Name:       "tally-genesis",
		ShortUsage: "govbox tally-genesis <genesis.json>",
		ShortHelp: `Outputs a genesis with validators, delegators, governors, delegations, votes and one proposal.
Used to evaluate the performance of the governance tally.`,
		FlagSet: fs,
		Exec: func(ctx context.Context, args []string) error {
			if err := fs.Parse(args); err != nil {
				return err
			}
			if fs.NArg() != 1 {
				return flag.ErrHelp
			}
			if *numVals < 1 {
				return fmt.Errorf("numVals must be greater than 0")
			}
			if *nodeAddr == "" {
				return fmt.Errorf("nodeAddr flag must be provided")
			}
			if *nodeConsPubkey == "" {
				return fmt.Errorf("nodeConsPubkey flag must be provided")
			}
			return tallyGenesis(ctx, fs.Arg(0), *nodeAddr, *nodeConsPubkey, *numVals, *numDels, *numGovs)
		},
	}
}

func tallyCmd() *ffcli.Command {
	return &ffcli.Command{
		Name:       "tally",
		ShortUsage: "govbox tally <path>",
		ShortHelp:  "Print the comparison between the tally result and the tally computed from <path>",
		Exec: func(ctx context.Context, args []string) error {
			if len(args) == 0 {
				return flag.ErrHelp
			}
			datapath := args[0]
			votesByAddr, err := parseVotesByAddr(datapath)
			if err != nil {
				return err
			}
			valsByAddr, err := parseValidatorsByAddr(datapath, votesByAddr)
			if err != nil {
				return err
			}
			delegsByAddr, err := parseDelegationsByAddr(datapath)
			if err != nil {
				return err
			}
			results, totalVotingPower := tally(votesByAddr, valsByAddr, delegsByAddr)
			printTallyResults(results, totalVotingPower, parseProp(datapath))
			return nil
		},
	}
}

func accountsCmd() *ffcli.Command {
	return &ffcli.Command{
		Name:       "accounts",
		ShortUsage: "govbox accounts <path>",
		ShortHelp:  "Consolidate the data in <path> into a single file <path>/accounts.json",
		Exec: func(ctx context.Context, args []string) error {
			if len(args) == 0 {
				return flag.ErrHelp
			}
			var (
				datapath     = args[0]
				accountsFile = filepath.Join(datapath, "accounts.json")
			)
			votesByAddr, err := parseVotesByAddr(datapath)
			if err != nil {
				return err
			}
			valsByAddr, err := parseValidatorsByAddr(datapath, votesByAddr)
			if err != nil {
				return err
			}
			delegsByAddr, err := parseDelegationsByAddr(datapath)
			if err != nil {
				return err
			}
			balancesByAddr, err := parseBalancesByAddr(datapath, "uatom")
			if err != nil {
				return err
			}
			accountTypesByAddr, err := parseAccountTypesPerAddr(datapath)
			if err != nil {
				return err
			}

			accounts := getAccounts(delegsByAddr, votesByAddr, valsByAddr, balancesByAddr, accountTypesByAddr)

			bz, err := json.MarshalIndent(accounts, "", "  ")
			if err != nil {
				return err
			}
			if err := os.WriteFile(accountsFile, bz, 0o666); err != nil {
				return err
			}
			fmt.Printf("%s file created.\n", accountsFile)

			return nil
		},
	}
}

func genesisCmd() *ffcli.Command {
	return &ffcli.Command{
		Name:       "genesis",
		ShortUsage: "govbox genesis <genesis.json> <path>",
		ShortHelp:  "Outputs an updated version of <genesis.json> with the airdrop",
		Exec: func(ctx context.Context, args []string) error {
			if len(args) != 2 {
				return flag.ErrHelp
			}
			var (
				genesisFile  = args[0]
				datapath     = args[1]
				accountsFile = filepath.Join(datapath, "accounts.json")
			)
			accounts, err := parseAccounts(accountsFile)
			if err != nil {
				return err
			}
			airdrop, err := distribution(accounts, defaultDistriParams(), "atone")
			if err != nil {
				return err
			}
			return writeGenesis(genesisFile, airdrop)
		},
	}
}

func autoStakingCmd() *ffcli.Command {
	return &ffcli.Command{
		Name:       "autostaking",
		ShortUsage: "govbox autostaking <path>",
		ShortHelp:  "Experimental command to evaluate auto-staking algorithms",
		LongHelp:   `Final implementation in GovGen commit https://github.com/atomone-hub/govgen/commit/3c40c31`,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) == 0 {
				return flag.ErrHelp
			}
			datapath := args[0]
			return autoStaking(filepath.Join(datapath, "genesis-govgen.json"))
		},
	}
}

func distributionCmd() *ffcli.Command {
	fs := flag.NewFlagSet("distribution", flag.ContinueOnError)
	chartMode := fs.Bool("chart", false, "Outputs a chart instead of Markdown tables")
	yesMultipliers := fs.String("yesMultipliers", "1", "List of possible comma-seperated Yes multipliers")
	noMultipliers := fs.String("noMultipliers", "9", "List of possible comma-separated No multipliers")
	prefix := fs.String("prefix", "", "Cosmos address prefix (by default it is unchanged: \"cosmos\")")

	cmd := &ffcli.Command{
		Name:       "distribution",
		ShortUsage: "govbox distribution <path>",
		ShortHelp:  "Convert <path>/accounts.json into <path>/airdrop.json",
		LongHelp:   `Generate the ATONE distribution described in GovGen PROP 001`,
		FlagSet:    fs,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) == 0 {
				return flag.ErrHelp
			}
			fs.Parse(args)
			// Build distribution parameters from yes and no multipliers
			var distriParamss []distriParams
			for _, y := range strings.Split(*yesMultipliers, ",") {
				for _, n := range strings.Split(*noMultipliers, ",") {
					distriParams := defaultDistriParams()
					distriParams.yesVotesMultiplier = sdk.MustNewDecFromStr(y)
					distriParams.noVotesMultiplier = sdk.MustNewDecFromStr(n)
					distriParamss = append(distriParamss, distriParams)
				}
			}
			var (
				datapath          = fs.Arg(0)
				accountsFile      = filepath.Join(datapath, "accounts.json")
				airdropFile       = filepath.Join(datapath, "airdrop.json")
				airdropDetailFile = filepath.Join(datapath, "airdrop_detail.csv")
				airdrops          []airdrop
			)
			accounts, err := parseAccounts(accountsFile)
			if err != nil {
				return err
			}
			for _, params := range distriParamss {
				airdrop, err := distribution(accounts, params, *prefix)
				if err != nil {
					return err
				}
				airdrops = append(airdrops, airdrop)
			}
			if err := printAirdropsStats(*chartMode, airdrops); err != nil {
				return err
			}
			if len(airdrops) == 1 {
				// Write airdrop.json only if a single distriParamss
				bz, err := json.MarshalIndent(airdrops[0].addresses, "", "  ")
				if err != nil {
					return err
				}
				if err := os.WriteFile(airdropFile, bz, 0o666); err != nil {
					return err
				}
				fmt.Printf("⚠ '%s' has been created/updated, don't forget to update S3 ⚠\n", airdropFile)

				f, err := os.Create(airdropDetailFile)
				if err != nil {
					return err
				}
				defer f.Close()
				w := csv.NewWriter(f)
				w.Write([]string{
					"address", "factor",
					"yesAtomAmt", "yesMultiplier", "yesBonusMalus", "yesAtoneAmt",
					"noAtomAmt", "noMultiplier", "noBonusMalus", "noAtoneAmt",
					"nwvAtomAmt", "nwvMultiplier", "nwvBonusMalus", "nwvAtoneAmt",
					"absAtomAmt", "absMultiplier", "absBonusMalus", "absAtoneAmt",
					"dnvAtomAmt", "dnvMultiplier", "dnvBonusMalus", "dnvAtoneAmt",
					"liquidAtomAmt", "liquidMultiplier", "liquidBonusMalus", "liquidAtoneAmt",
					"totalAtoneAmt",
				})
				for _, v := range airdrops[0].addressesDetail {
					w.Write([]string{
						v.Address, v.YesDetail.Factor.String(),
						v.YesDetail.AtomAmt.String(), v.YesDetail.Multiplier.String(), v.YesDetail.BonusMalus.String(), v.YesDetail.AtoneAmt.String(),
						v.NoDetail.AtomAmt.String(), v.NoDetail.Multiplier.String(), v.NoDetail.BonusMalus.String(), v.NoDetail.AtoneAmt.String(),
						v.NWVDetail.AtomAmt.String(), v.NWVDetail.Multiplier.String(), v.NWVDetail.BonusMalus.String(), v.NWVDetail.AtoneAmt.String(),
						v.AbsDetail.AtomAmt.String(), v.AbsDetail.Multiplier.String(), v.AbsDetail.BonusMalus.String(), v.AbsDetail.AtoneAmt.String(),
						v.DnvDetail.AtomAmt.String(), v.DnvDetail.Multiplier.String(), v.DnvDetail.BonusMalus.String(), v.DnvDetail.AtoneAmt.String(),
						v.LiquidDetail.AtomAmt.String(), v.LiquidDetail.Multiplier.String(), v.LiquidDetail.BonusMalus.String(), v.LiquidDetail.AtoneAmt.String(),
						v.Total.String(),
					})
				}
				w.Flush()
				fmt.Printf("⚠ '%s' has been created/updated, don't forget to update S3 ⚠\n", airdropDetailFile)
			}
			return nil
		},
	}
	return cmd
}

func top20Cmd() *ffcli.Command {
	return &ffcli.Command{
		Name:       "top20",
		ShortUsage: "govbox top20 <path>",
		ShortHelp:  "Prints the top richest addresses of <path>/airdrop.json",
		Exec: func(ctx context.Context, args []string) error {
			var (
				datapath   = args[0]
				knownAddrs = map[string]string{
					"cosmos14lultfckehtszvzw4ehu0apvsr77afvyhgqhwh": "Dokia",
					"cosmos1p3ucd3ptpw902fluyjzhq3ffgq4ntddac9sa3s": "Binance?",
					"cosmos1nm0rrq86ucezaf8uj35pq9fpwr5r82cl8sc7p5": "Kraken",
					"cosmos1zr7aswwzskhav7w57vwpaqsafuh5uj7nv8a964": "SG1?",
					"cosmos1f70nsqtq0wcd0kymq79ca2p0k5napnm6yqc94x": "ChorusOne?",
					"cosmos1wlh0f94r6c4y5nwsqlxd2384jmxlljstame50p": "CosmosStation?",
				}
			)

			f, err := os.Open(filepath.Join(datapath, "airdrop.json"))
			if err != nil {
				return err
			}
			defer f.Close()
			var addresses map[string]sdk.Int
			err = json.NewDecoder(f).Decode(&addresses)
			if err != nil {
				return err
			}
			addrs := slices.Collect(maps.Keys(addresses))
			sort.Slice(addrs, func(i, j int) bool {
				return addresses[addrs[i]].GT(addresses[addrs[j]])
			})
			var (
				top20    = make([]string, 20)
				totalAmt = sdk.NewInt(0)
			)
			for i, addr := range addrs {
				if i < 20 {
					top20[i] = addr
				}
				totalAmt = totalAmt.Add(addresses[addr])
			}
			table := newMarkdownTable("Position", "Address", "ID", "$ATONE", "Supply %")
			for i, addr := range top20 {
				amt := addresses[addr]
				table.Append([]string{
					fmt.Sprint(i + 1),
					fmt.Sprintf("[%[1]s](https://www.mintscan.io/cosmos/address/%[1]s)", addr),
					knownAddrs[addr],
					human(amt),
					humanPercent(amt.ToLegacyDec().Quo(totalAmt.ToLegacyDec())),
				})
			}
			table.Render()
			return nil
		},
	}
}

func propJSONCmd() *ffcli.Command {
	fs := flag.NewFlagSet("propJSON", flag.ContinueOnError)
	deposit := fs.String("deposit", "512000000uatone", "Proposal deposit")
	return &ffcli.Command{
		Name:       "propJSON",
		ShortUsage: "govbox propJSON <path/to/proposal.md>",
		ShortHelp:  "Prints the JSON format compatible with the submit-proposal CLI gov module",
		FlagSet:    fs,
		Exec: func(ctx context.Context, args []string) error {
			if err := fs.Parse(args); err != nil {
				return err
			}
			if fs.NArg() != 1 {
				return flag.ErrHelp
			}
			bz, err := os.ReadFile(fs.Arg(0))
			if err != nil {
				return err
			}
			if len(string(bz)) > 10000 {
				return fmt.Errorf("Description has more than 10000 characters (%d)", len(string(bz)))
			}
			// Fetch title from markdown
			title := strings.SplitN(string(bz), "\n", 2)[0]
			title = title[2:] // Remove the '# ' prefix

			data := map[string]any{
				"title":    title,
				"summary":  string(bz),
				"deposit":  *deposit,
				"metadata": "ipfs://CID",
			}
			bz, err = json.MarshalIndent(data, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(bz))
			return nil
		},
	}
}

func signTxCmd() *ffcli.Command {
	fs := flag.NewFlagSet("signTx", flag.ContinueOnError)
	return &ffcli.Command{
		Name:       "signTx",
		ShortUsage: "govbox signTx <path/to/tx.json>",
		ShortHelp:  "Outputs signed transactions",
		Exec: func(ctx context.Context, args []string) error {
			if err := fs.Parse(args); err != nil {
				return err
			}
			if fs.NArg() != 1 {
				return flag.ErrHelp
			}
			return signTx(fs.Arg(0))
		},
	}
}

func vestingCmd() *ffcli.Command {
	return &ffcli.Command{
		Name:       "vesting",
		ShortUsage: "govbox vesting <path>",
		ShortHelp:  "Report vesting accounts analysis",
		Exec: func(ctx context.Context, args []string) error {
			if len(args) == 0 {
				return flag.ErrHelp
			}
			datapath := args[0]
			err := analyzeVestingAccounts(datapath)
			if err != nil {
				return err
			}
			return nil
		},
	}
}

func gnoAirdropCmd() *ffcli.Command {
	fs := flag.NewFlagSet("gno-airdrop", flag.ContinueOnError)
	return &ffcli.Command{
		Name:    "gno-airdrop",
		FlagSet: fs,
		Exec: func(ctx context.Context, args []string) error {
			if err := fs.Parse(args); err != nil {
				return err
			}
			if fs.NArg() != 1 {
				return flag.ErrHelp
			}
			f, err := os.Open(fs.Arg(0))
			if err != nil {
				return err
			}
			defer f.Close()
			// scan file line by line
			scanner := bufio.NewScanner(f)
			supply := sdk.ZeroInt()
			type entry struct {
				Addr  string
				Alloc sdk.Coin
			}
			var a []entry
			for scanner.Scan() {
				line := scanner.Text()
				items := strings.Split(line, "=")
				addr := items[0]
				alloc, err := sdk.ParseCoinNormalized(items[1])
				if err != nil {
					panic(err)
				}
				supply = supply.Add(alloc.Amount)
				a = append(a, entry{addr, alloc})
			}

			expSupply := sdk.MustNewDecFromStr("700000000000000")
			fact := expSupply.Quo(supply.ToLegacyDec())
			newSup := sdk.ZeroInt()
			for _, e := range a {
				newAmt := e.Alloc.Amount.ToLegacyDec().Mul(fact)
				newAlloc := sdk.NewCoin(e.Alloc.Denom, newAmt.RoundInt())
				fmt.Printf("%s=%s\n", e.Addr, newAlloc)
				newSup = newSup.Add(newAlloc.Amount)
			}
			fmt.Println("SUPPLY", supply)
			fmt.Println("FACT", fact)
			fmt.Println("SUPPLY", newSup)

			return nil
		},
	}
}
