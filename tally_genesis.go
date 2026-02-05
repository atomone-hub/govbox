package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/atomone-hub/atomone/cmd/atomoned/cmd"
	"github.com/atomone-hub/atomone/x/gov"
	govkeeper "github.com/atomone-hub/atomone/x/gov/keeper"
	govtypes "github.com/atomone-hub/atomone/x/gov/types"
	govv1types "github.com/atomone-hub/atomone/x/gov/types/v1"

	dbm "github.com/cometbft/cometbft-db"
	tmjson "github.com/cometbft/cometbft/libs/json"
	"github.com/cometbft/cometbft/libs/log"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	tmtypes "github.com/cometbft/cometbft/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/store"
	storetypes "github.com/cosmos/cosmos-sdk/store/types"
	"github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func tallyGenesis(goCtx context.Context, genesisFile, nodeAddr string, nodeConsPubkey string, numVals, numDels, numGovs int) error {
	cmd.InitSDKConfig()
	var (
		addrs     = sims.CreateRandomAccounts(numVals + numDels + numGovs)
		valAddrs  = sims.ConvertAddrsToValAddrs(addrs[:numVals])
		delAddrs  = addrs[numVals : numVals+numDels]
		govAddrs  = addrs[numVals+numDels:]
		permAddrs = map[string][]string{
			banktypes.ModuleName:           {authtypes.Minter},
			stakingtypes.BondedPoolName:    {authtypes.Burner, authtypes.Staking},
			stakingtypes.NotBondedPoolName: {authtypes.Burner, authtypes.Staking},
			govtypes.ModuleName:            {},
		}
		keys = sdk.NewKVStoreKeys(
			authtypes.StoreKey,
			banktypes.StoreKey,
			stakingtypes.StoreKey,
			govtypes.StoreKey,
		)
		minDeposit = sdk.NewCoins(sdk.NewInt64Coin("uatone", 512_000_000))
	)
	// unmarshal node pubkey
	var nodePK cryptotypes.PubKey
	err := cdc.UnmarshalInterfaceJSON([]byte(nodeConsPubkey), &nodePK)
	if err != nil {
		panic(err)
	}
	// Set validator node to the first validator
	addrs[0] = sdk.MustAccAddressFromBech32(nodeAddr)
	valAddrs[0] = sdk.ValAddress(addrs[0])

	// create keepers and msgServers
	ctx := newContext(goCtx, keys)
	govAcct := authtypes.NewEmptyModuleAccount(govtypes.ModuleName)
	govAddr := govAcct.GetAddress().String()
	ak := authkeeper.NewAccountKeeper(cdc, keys[authtypes.StoreKey], authtypes.ProtoBaseAccount, permAddrs, "atone", govAddr)
	ak.InitGenesis(ctx, *authtypes.DefaultGenesisState())
	bk := bankkeeper.NewBaseKeeper(cdc, keys[banktypes.StoreKey], ak, nil, govAddr)
	bk.InitGenesis(ctx, banktypes.DefaultGenesisState())
	sk := stakingkeeper.NewKeeper(cdc, keys[stakingtypes.StoreKey], ak, bk, govAddr)
	stakingMsgServer := stakingkeeper.NewMsgServerImpl(sk)
	stakingGenesis := stakingtypes.DefaultGenesisState()
	stakingGenesis.Params.BondDenom = "uatone"
	sk.InitGenesis(ctx, stakingGenesis)
	gk := govkeeper.NewKeeper(cdc, keys[govtypes.StoreKey], ak, bk, sk, nil, govtypes.DefaultConfig(), govAddr)
	govGenesis := govv1types.DefaultGenesisState()
	govGenesis.Params.MinDeposit = minDeposit
	gov.InitGenesis(ctx, ak, bk, gk, govGenesis)
	govMsgServer := govkeeper.NewMsgServerImpl(gk)

	// fill all address bank balances with addrAmt
	var (
		addrAmt     = sdk.NewInt(1_000_000_000_000)
		addrBalance = sdk.NewCoins(sdk.NewCoin("uatone", addrAmt))
		// mint amt * number of addresses
		totalAddrAmt = addrAmt.Mul(sdk.NewInt(int64(len(addrs))))
		// mint 3 x totalAddrAmt for the node so it can run the chain alone
		nodeAmt       = totalAddrAmt.Mul(sdk.NewInt(3))
		nodeBalance   = sdk.NewCoins(sdk.NewCoin("uatone", nodeAmt))
		supplyAmt     = totalAddrAmt.Add(nodeAmt)
		supplyBalance = sdk.NewCoins(sdk.NewCoin("uatone", supplyAmt))
	)
	err = bk.MintCoins(ctx, banktypes.ModuleName, supplyBalance)
	if err != nil {
		panic(err)
	}
	// send amt to each account
	for i, a := range addrs {
		var amt sdk.Coins
		if i == 0 {
			// first address is the node
			amt = nodeBalance
		} else {
			amt = addrBalance
		}
		err := bk.SendCoinsFromModuleToAccount(ctx, banktypes.ModuleName, a, amt)
		if err != nil {
			panic(err)
		}
	}
	// create validators
	for i, a := range valAddrs {
		valOpAddr := sdk.ValAddress(a)
		description := stakingtypes.NewDescription(fmt.Sprintf("val%d", i), "", "", "", "")
		commissionRates := stakingtypes.CommissionRates{
			Rate:          sdk.MustNewDecFromStr("0.1"),
			MaxRate:       sdk.MustNewDecFromStr("0.2"),
			MaxChangeRate: sdk.MustNewDecFromStr("0.01"),
		}
		var (
			pk             cryptotypes.PubKey
			selfDelegation sdk.Coin
		)
		if i == 0 {
			// first validator is our operator, use the pk from parameters
			pk = nodePK
			// use full node balance for self delegation so it got >67% of voting power
			selfDelegation = sdk.NewCoin("uatone", nodeAmt)
		} else {
			// other validators get a random pk
			pk = ed25519.GenPrivKey().PubKey()
			selfDelegation = sdk.NewInt64Coin("uatone", 1_000_000)
		}
		msg, err := stakingtypes.NewMsgCreateValidator(valOpAddr, pk,
			selfDelegation, description, commissionRates,
			sdk.NewInt(1))
		if err != nil {
			panic(err)
		}
		_, err = stakingMsgServer.CreateValidator(ctx, msg)
		if err != nil {
			panic(err)
		}
	}
	// bond validators
	_, err = sk.ApplyAndReturnValidatorSetUpdates(ctx)
	if err != nil {
		panic(err)
	}

	// delegate to validators
	var (
		valIdx                = 0
		numDelegsPerDelegator = 5
		delAmt                = sdk.NewInt64Coin("uatone", 900_000_000_000/int64(numDelegsPerDelegator)) // deleg 90% of balance
	)
	// first do governors delegation to validators
	// NOTE: for performance reason, it is better to first create and delegate to
	// governors rather than doing all the validator delegations, because
	// DelegateGovernor has bad perf when delegations already exists.
	for _, a := range govAddrs {
		for j := 0; j < numDelegsPerDelegator; j++ {
			msg := stakingtypes.NewMsgDelegate(a, sdk.ValAddress(valAddrs[valIdx]), delAmt)
			_, err := stakingMsgServer.Delegate(ctx, msg)
			if err != nil {
				panic(err)
			}
			valIdx++ // next delegation to next validator
			if valIdx >= numVals {
				valIdx = 0
			}
		}
	}

	// Note: Governor functionality was removed in atomone v3.x
	_ = govAddrs // unused but kept for potential future use

	// second all the other validator delegations
	for _, a := range delAddrs {
		for j := 0; j < numDelegsPerDelegator; j++ {
			msg := stakingtypes.NewMsgDelegate(a, sdk.ValAddress(valAddrs[valIdx]), delAmt)
			_, err := stakingMsgServer.Delegate(ctx, msg)
			if err != nil {
				panic(err)
			}
			valIdx++ // next delegation to next validator
			if valIdx >= numVals {
				valIdx = 0
			}
		}
	}

	// create proposal
	msg, err := govv1types.NewMsgSubmitProposal(nil, minDeposit, addrs[1].String(), "", "my prop", "")
	if err != nil {
		panic(err)
	}
	_, err = govMsgServer.SubmitProposal(ctx, msg)
	if err != nil {
		panic(err)
	}
	// vote on proposal
	for i, a := range append(delAddrs, govAddrs...) {
		vote := govv1types.VoteOption(i%3) + 1
		msg := govv1types.NewMsgVote(a, 1, vote, "")
		_, err := govMsgServer.Vote(ctx, msg)
		if err != nil {
			panic(err)
		}
	}
	{
		// bankGenesis := bk.ExportGenesis(ctx)
		// bz, _ := cdc.MarshalJSON(bankGenesis)
		// printJSON("BANK", bz)

		// stakingGenesis = sk.ExportGenesis(ctx)
		// bz, _ := cdc.MarshalJSON(stakingGenesis)
		// printJSON("STAKING", bz)

		// govGenesis := gov.ExportGenesis(ctx, gk)
		// bz, _ := cdc.MarshalJSON(govGenesis)
		// printJSON("GOV", bz)
	}

	// Read input genesis and update it
	bz, err := os.ReadFile(genesisFile)
	if err != nil {
		return fmt.Errorf("readfile %s: %w", genesisFile, err)
	}
	var genesisState tmtypes.GenesisDoc
	if err := tmjson.Unmarshal(bz, &genesisState); err != nil {
		return fmt.Errorf("unmarshal genesis doc: %w", err)
	}
	var appState map[string]json.RawMessage
	if err := json.Unmarshal(genesisState.AppState, &appState); err != nil {
		return fmt.Errorf("unmarshal appstate: %w", err)
	}
	appState["bank"], err = cdc.MarshalJSON(bk.ExportGenesis(ctx))
	if err != nil {
		return fmt.Errorf("marshal bank genesis: %w", err)
	}
	appState["gov"], err = cdc.MarshalJSON(gov.ExportGenesis(ctx, gk))
	if err != nil {
		return fmt.Errorf("marshal gov genesis: %w", err)
	}
	appState["auth"], err = cdc.MarshalJSON(ak.ExportGenesis(ctx))
	if err != nil {
		return fmt.Errorf("marshal auth genesis: %w", err)
	}
	appState["staking"], err = cdc.MarshalJSON(sk.ExportGenesis(ctx))
	if err != nil {
		return fmt.Errorf("marshal staking genesis: %w", err)
	}
	genesisState.AppState, err = json.MarshalIndent(appState, "", "  ")
	if err != nil {
		return err
	}
	bz, err = tmjson.MarshalIndent(genesisState, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(bz))
	return nil
}

func newContext(ctx context.Context, keys map[string]*storetypes.KVStoreKey) sdk.Context {
	db := dbm.NewMemDB() // TODO try a disk store to better represent real usage
	cms := store.NewCommitMultiStore(db)
	for _, v := range keys {
		cms.MountStoreWithDB(v, storetypes.StoreTypeIAVL, db)
	}
	err := cms.LoadLatestVersion()
	if err != nil {
		panic(err)
	}
	return sdk.NewContext(cms, tmproto.Header{}, false, log.NewNopLogger()).
		WithContext(ctx).WithBlockTime(time.Now())
}


func printJSON(name string, bz []byte) {
	var m map[string]any
	err := json.Unmarshal(bz, &m)
	if err != nil {
		panic(err)
	}
	bz, _ = json.MarshalIndent(m, "", " ")
	fmt.Println(name, string(bz))
	fmt.Println()
}
