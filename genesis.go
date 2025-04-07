package main

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"slices"
	"strings"

	tmjson "github.com/cometbft/cometbft/libs/json"
	tmtypes "github.com/cometbft/cometbft/types"

	govtypes "github.com/atomone-hub/atomone/x/gov/types/v1"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
)

const constitutionLink = "https://raw.githubusercontent.com/atomone-hub/genesis/af652e0bc2bf1579350648770bf1f7b2d51d4884/CONSTITUTION.md"

// writeGenesis reads airdrop and fills the related modules accordingly in the
// genesisFile.
//
// Note about JSON encoding: the genesisDoc, the appState and the modules
// genesis use different encoding primitives (it would too simple otherwise!):
// - genesisDoc uses tmjson "github.com/cometbft/cometbft/libs/json"
// - appState uses standard "encoding/json"
// - modules genesis use protoJSON (represented as cdc)
func writeGenesis(genesisFile string, airdrop airdrop) error {
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
	var authGen authtypes.GenesisState
	if err := cdc.UnmarshalJSON(appState["auth"], &authGen); err != nil {
		return fmt.Errorf("umarshal auth genesis: %w", err)
	}
	var bankGen banktypes.GenesisState
	if err := cdc.UnmarshalJSON(appState["bank"], &bankGen); err != nil {
		return fmt.Errorf("umarshal bank genesis: %w", err)
	}
	var distrGen distrtypes.GenesisState
	if err := cdc.UnmarshalJSON(appState["distribution"], &distrGen); err != nil {
		return fmt.Errorf("umarshal distribution genesis: %w", err)
	}
	var govGen govtypes.GenesisState
	if err := cdc.UnmarshalJSON(appState["gov"], &govGen); err != nil {
		return fmt.Errorf("umarshal gov genesis: %w", err)
	}

	// Reset supply, balances and accounts
	bankGen.Supply = sdk.NewCoins()
	bankGen.Balances = nil
	authGen.Accounts = nil
	// Add airdrop.addresses to balances and accounts
	const ticker = "atone"
	for _, addr := range slices.Sorted(maps.Keys(airdrop.addresses)) {
		// update bank genesis
		amt := airdrop.addresses[addr]
		coins := sdk.NewCoins(sdk.NewCoin("u"+ticker, amt))
		bankGen.Balances = append(bankGen.Balances, banktypes.Balance{
			Address: addr,
			Coins:   coins,
		})
		bankGen.Supply = bankGen.Supply.Add(coins...)

		// update auth genesis
		acc := &authtypes.BaseAccount{Address: addr}
		any, err := codectypes.NewAnyWithValue(acc)
		if err != nil {
			return fmt.Errorf("newAny from base account: %w", err)
		}
		authGen.Accounts = append(authGen.Accounts, any)
	}
	// Add reserved address
	// hex:    0x000000000000000000000000000000000000bda0
	// bech32: atone1qqqqqqqqqqqqqqqqqqqqqqqqqqqqp0dqtalx52
	reservedAddrBz := []byte("\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\xbd\xa0")
	reservedAddrCoins := sdk.NewCoins(sdk.NewCoin("u"+ticker, airdrop.reservedAddr.RoundInt()))
	reservedAddr := sdk.MustBech32ifyAddressBytes("atone", reservedAddrBz)
	bankGen.Balances = append(bankGen.Balances, banktypes.Balance{
		Address: reservedAddr,
		Coins:   reservedAddrCoins,
	})
	bankGen.Supply = bankGen.Supply.Add(reservedAddrCoins...)
	// add auth reserved address
	reservedAcc := &authtypes.BaseAccount{Address: reservedAddr}
	any, err := codectypes.NewAnyWithValue(reservedAcc)
	if err != nil {
		return fmt.Errorf("newAny from base account: %w", err)
	}
	authGen.Accounts = append(authGen.Accounts, any)

	// setup community pool
	communityPoolCoins := sdk.NewCoins(sdk.NewCoin("u"+ticker, airdrop.communityPool.RoundInt()))
	distrGen.FeePool = distrtypes.FeePool{
		CommunityPool: sdk.NewDecCoinsFromCoins(communityPoolCoins...),
	}
	// same amount must be distributed to the distribution module account
	distrModuleAddr := sdk.MustBech32ifyAddressBytes("atone", authtypes.NewModuleAddress(distrtypes.ModuleName))
	bankGen.Balances = append(bankGen.Balances, banktypes.Balance{
		Address: distrModuleAddr,
		Coins:   communityPoolCoins,
	})
	bankGen.Supply = bankGen.Supply.Add(communityPoolCoins...)

	// setup bank params and denoms
	bankGen.Params = banktypes.Params{
		DefaultSendEnabled: true,
		SendEnabled:        []*banktypes.SendEnabled{},
	}
	bankGen.DenomMetadata = []banktypes.Metadata{
		{
			Display:     ticker,
			Symbol:      strings.ToUpper(ticker),
			Base:        "u" + ticker,
			Name:        "AtomOne Atone",
			Description: "The native staking token of AtomOne Hub",
			DenomUnits: []*banktypes.DenomUnit{
				{
					Aliases:  []string{"micro" + ticker},
					Denom:    "u" + ticker,
					Exponent: 0,
				},
				{
					Aliases:  []string{"milli" + ticker},
					Denom:    "m" + ticker,
					Exponent: 3,
				},
				{
					Aliases:  []string{ticker},
					Denom:    ticker,
					Exponent: 6,
				},
			},
		},
	}

	// Update constitution
	resp, err := http.Get(constitutionLink)
	if err != nil {
		return err
	}
	bz, err = io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	govGen.Constitution = string(bz)

	//-----------------------------------------
	// Update the  genesis
	appState["bank"], err = cdc.MarshalJSON(&bankGen)
	if err != nil {
		return fmt.Errorf("marshal bank genesis: %w", err)
	}
	appState["distribution"], err = cdc.MarshalJSON(&distrGen)
	if err != nil {
		return fmt.Errorf("marshal distribution genesis: %w", err)
	}
	appState["gov"], err = cdc.MarshalJSON(&govGen)
	if err != nil {
		return fmt.Errorf("marshal gov genesis: %w", err)
	}
	appState["auth"], err = cdc.MarshalJSON(&authGen)
	if err != nil {
		return fmt.Errorf("marshal auth genesis: %w", err)
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

// test code for address generation
// addr := "cosmos1uhqq8atwfm79amnmrk5d3ze6f7arkknjma522p"
// addr = "cosmos1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqrdqvfzfpm"
// addr = "atone1qqqqqqqqqqqqqqqqqqqqqqqqqqqqqrdqzf7whr"
// var a uint64 = 0x0000000000000000000000000000000000000da0
// binary.Write(a, binary.LittleEndian, a)
// res := make([]byte, 20)
// bz := strconv.Itoa(a)
// fmt.Printf("%x\n", bz)
// binary.LittleEndian.PutUint64(res, a)
// fmt.Printf("%x\n", res)
// aton, _ := convertBech32(addr, "cosmos", "atone")
// fmt.Println(aton)

// sdkAddr, err := sdk.GetFromBech32(addr, "atone")
// if err != nil {
// panic(err)
// }
// fmt.Println(addr, len(sdkAddr), hex.EncodeToString(sdkAddr))
// fmt.Printf("%X\n", sdkAddr)
// os.Exit(1)
// resAddr := []byte("\xe5\xc0\x03\xf5\x6e\x4e\xfc\x5e\xee\x7b\x1d\xa8\xd8\x8b\x3a\x4f\xba\x3b\x5a\x72")
// resAddr := []byte("\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x0d\xa0")
// addr2 := sdk.MustBech32ifyAddressBytes("cosmos", resAddr)
// fmt.Println(addr2)
