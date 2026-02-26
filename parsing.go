package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cosmossdk.io/math"

	"github.com/cosmos/gogoproto/jsonpb"
	h "github.com/dustin/go-humanize"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	proposaltypes "github.com/cosmos/cosmos-sdk/x/params/types/proposal"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	icatypes "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/types"
)

var (
	registry    = codectypes.NewInterfaceRegistry()
	marshaler   jsonpb.Marshaler
	unmarshaler jsonpb.Unmarshaler
	cdc         codec.Codec
)

func init() {
	cryptocodec.RegisterInterfaces(registry)
	govtypes.RegisterInterfaces(registry)
	sdk.RegisterInterfaces(registry)
	proposaltypes.RegisterInterfaces(registry)
	authtypes.RegisterInterfaces(registry)
	vestingtypes.RegisterInterfaces(registry)
	icatypes.RegisterInterfaces(registry)
	marshaler = jsonpb.Marshaler{AnyResolver: registry}
	unmarshaler = jsonpb.Unmarshaler{AnyResolver: registry}
	// FIXME: replace marshaler and unmarshaler by cdc?
	cdc = codec.NewProtoCodec(registry)
}

const M = 1_000_000 // 1 million

func human(i math.Int) string {
	M := math.NewInt(M)
	return h.Comma(i.Quo(M).Int64())
}

func humani(i int64) string {
	return h.Comma(i / M)
}

func humand(d math.LegacyDec) string {
	M := math.LegacyNewDec(1_000_000)
	return h.Comma(d.Quo(M).RoundInt64())
}

func humanPercentI(d math.LegacyDec) string {
	return fmt.Sprintf("%d%%", d.Mul(math.LegacyNewDec(100)).RoundInt64())
}

func humanPercent(d math.LegacyDec) string {
	return fmt.Sprintf("%.2f %%", d.Mul(math.LegacyNewDec(100)).MustFloat64())
}

// convertBech32 derive addr from src to dst bech32 prefix.
func convertBech32(addr, src, dst string) (string, error) {
	sdkAddr, err := sdk.GetFromBech32(addr, src)
	if err != nil {
		return "", fmt.Errorf("GetFromBech32 '%s': %v", addr, err)
	}
	addr, err = sdk.Bech32ifyAddressBytes(dst, sdkAddr)
	if err != nil {
		return "", fmt.Errorf("Bech32ifyAddressBytes '%s': %v", addr, err)
	}
	return addr, nil
}

func parseAccounts(path string) ([]Account, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s file, run `%s accounts` to generate it: %w", path, os.Args[0], err)
	}
	defer f.Close()
	var accounts []Account
	if err := json.NewDecoder(f).Decode(&accounts); err != nil {
		return nil, fmt.Errorf("cannot json decode accounts from file %s: %w", path, err)
	}
	return accounts, nil
}

func parseAccountTypesPerAddr(path string) (map[string]string, error) {
	f, err := os.Open(filepath.Join(path, "auth_genesis.json"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var genesis authtypes.GenesisState
	err = unmarshaler.Unmarshal(f, &genesis)
	if err != nil {
		return nil, err
	}
	accountTypesByAddr := make(map[string]string)
	for i, any := range genesis.Accounts {
		var acc authtypes.GenesisAccount
		registry.UnpackAny(any, &acc)
		accountTypesByAddr[acc.GetAddress().String()] = genesis.Accounts[i].GetTypeUrl()
	}
	fmt.Printf("%s accounts\n", h.Comma(int64(len(accountTypesByAddr))))
	return accountTypesByAddr, nil
}

func analyzeVestingAccounts(path string) error {
	f, err := os.Open(filepath.Join(path, "auth_genesis.json"))
	if err != nil {
		return err
	}
	defer f.Close()
	var genesis authtypes.GenesisState
	err = unmarshaler.Unmarshal(f, &genesis)
	if err != nil {
		return err
	}
	var (
		now             = time.Now()
		numVesting      int
		numStillVesting int
		totalVesting    sdk.Coins
		blocktime             = time.Unix(1700946028, 0) // time of prop848 2023-11-25 22:00:28 +0100 CET
		highCapInt      int64 = 10000000000
		highCap               = sdk.NewCoins(sdk.NewInt64Coin("uatom", highCapInt))
		numHighCap      int
	)
	for i, any := range genesis.Accounts {
		var acc authtypes.GenesisAccount
		registry.UnpackAny(any, &acc)
		if strings.Contains(genesis.Accounts[i].GetTypeUrl(), "Vesting") {
			numVesting++
			switch v := acc.(type) {
			case *vestingtypes.ContinuousVestingAccount:
				d := time.Unix(v.EndTime, 0)
				if d.After(now) {
					numStillVesting++
					vestingAmt := v.GetVestingCoins(blocktime)
					totalVesting = totalVesting.Add(vestingAmt...)
					if vestingAmt.IsAllGT(highCap) {
						numHighCap++
						fmt.Println("CONT VEST", acc.GetAddress().String(), d, vestingAmt)
					}
				}
			case *vestingtypes.DelayedVestingAccount:
				d := time.Unix(v.EndTime, 0)
				if d.After(now) {
					numStillVesting++
					totalVesting = totalVesting.Add(v.OriginalVesting...)
					if v.OriginalVesting.IsAllGT(highCap) {
						numHighCap++
						fmt.Println("DEL VEST", acc.GetAddress().String(), d, v.OriginalVesting)
					}
				}
			}
		}
	}
	fmt.Printf("%d/%d valid vesting accounts, total of %s freed\n", numStillVesting, numVesting, totalVesting)
	fmt.Printf("%d vesting account(s) with more than %s vesting\n", numHighCap, highCap)
	return nil
}

func parseVotesByAddr(path string) (map[string]govtypes.WeightedVoteOptions, error) {
	f, err := os.Open(filepath.Join(path, "votes.json"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	// XXX workaround to unmarshal votes because proto doesn't support top-level array
	dec := json.NewDecoder(f)
	_, err = dec.Token()
	if err != nil {
		return nil, err
	}
	votesByAddr := make(map[string]govtypes.WeightedVoteOptions)
	for dec.More() {
		var vote govtypes.Vote
		err := unmarshaler.UnmarshalNext(dec, &vote)
		if err != nil {
			return nil, err
		}
		votesByAddr[vote.Voter] = vote.Options
	}
	fmt.Printf("%s votes\n", h.Comma(int64(len(votesByAddr))))
	return votesByAddr, nil
}

func parseDelegationsByAddr(path string) (map[string][]stakingtypes.Delegation, error) {
	f, err := os.Open(filepath.Join(path, "delegations.json"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var delegs []stakingtypes.Delegation
	err = json.NewDecoder(f).Decode(&delegs)
	if err != nil {
		return nil, err
	}
	delegsByAddr := make(map[string][]stakingtypes.Delegation)
	for _, d := range delegs {
		delegsByAddr[d.DelegatorAddress] = append(delegsByAddr[d.DelegatorAddress], d)
	}
	fmt.Printf("%s delegations for %s delegators\n", h.Comma(int64(len(delegs))),
		h.Comma(int64(len(delegsByAddr))))
	return delegsByAddr, nil
}

func parseValidatorsByAddr(path string, votesByAddr map[string]govtypes.WeightedVoteOptions) (map[string]govtypes.ValidatorGovInfo, error) {
	f, err := os.Open(filepath.Join(path, "active_validators.json"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	// XXX workaround to unmarshal validators because proto doesn't support top-level array
	dec := json.NewDecoder(f)
	_, err = dec.Token()
	if err != nil {
		return nil, err
	}
	valsByAddr := make(map[string]govtypes.ValidatorGovInfo)
	for dec.More() {
		var val stakingtypes.Validator
		err := unmarshaler.UnmarshalNext(dec, &val)
		if err != nil {
			return nil, err
		}

		valAddr, err := sdk.ValAddressFromBech32(val.OperatorAddress)
		if err != nil {
			panic(err)
		}
		accAddr := sdk.AccAddress(valAddr.Bytes()).String()
		valsByAddr[val.OperatorAddress] = govtypes.NewValidatorGovInfo(
			valAddr,
			val.GetBondedTokens(),
			val.GetDelegatorShares(),
			math.LegacyZeroDec(),
			votesByAddr[accAddr],
		)
	}
	fmt.Printf("%d validators\n", len(valsByAddr))
	return valsByAddr, nil
}

func parseProp(path string) govtypes.Proposal {
	f, err := os.Open(filepath.Join(path, "prop.json"))
	if err != nil {
		panic(err)
	}
	defer f.Close()
	var prop govtypes.Proposal
	err = unmarshaler.Unmarshal(f, &prop)
	if err != nil {
		panic(err)
	}
	return prop
}

func parseBalancesByAddr(path, denom string) (map[string]sdk.Coins, error) {
	f, err := os.Open(filepath.Join(path, "balances.json"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var balances []banktypes.Balance
	err = json.NewDecoder(f).Decode(&balances)
	if err != nil {
		return nil, err
	}
	balancesByAddr := make(map[string]sdk.Coins)
	for _, b := range balances {
		if denom == "" {
			balancesByAddr[b.Address] = b.Coins
		} else {
			ok, c := b.Coins.Find(denom)
			if ok {
				balancesByAddr[b.Address] = sdk.NewCoins(c)
			}
		}
	}
	fmt.Printf("%s account balances\n", h.Comma(int64(len(balancesByAddr))))
	return balancesByAddr, nil
}
