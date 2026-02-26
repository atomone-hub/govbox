package main

import (
	"encoding/json"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func TestGetAccounts(t *testing.T) {
	var (
		accAddrs = createAccountAddrs(2)
		accAddr1 = accAddrs[0].String()
		accAddr2 = accAddrs[1].String()
		valAddrs = createValidatorAddrs(2)
		// Validator1
		valAddr1       = valAddrs[0]
		valAddr1Str    = valAddr1.String()
		valAccAddr1Str = sdk.AccAddress(valAddrs[0]).String()
		// Validator2
		valAddr2       = valAddrs[1]
		valAddr2Str    = valAddr2.String()
		valAccAddr2Str = sdk.AccAddress(valAddrs[1]).String()
		// Handy funcs
		newVal = func(addr sdk.ValAddress, bonded, shares int64, vote govtypes.WeightedVoteOptions) govtypes.ValidatorGovInfo {
			return govtypes.ValidatorGovInfo{
				Address:             addr,
				BondedTokens:        math.NewInt(bonded),
				DelegatorShares:     math.LegacyNewDec(shares),
				DelegatorDeductions: math.LegacyZeroDec(),
				Vote:                vote,
			}
		}
		newDeleg = func(delAddr, valAddr string, shares int64) stakingtypes.Delegation {
			return stakingtypes.Delegation{
				DelegatorAddress: delAddr,
				ValidatorAddress: valAddr,
				Shares:           math.LegacyNewDec(shares),
			}
		}
		// Some votes
		noVote  govtypes.WeightedVoteOptions
		voteYes = govtypes.WeightedVoteOptions{{
			Option: govtypes.OptionYes,
			Weight: math.LegacyNewDec(1),
		}}
		voteNo = govtypes.WeightedVoteOptions{{
			Option: govtypes.OptionNo,
			Weight: math.LegacyNewDec(1),
		}}
		voteAbstain = govtypes.WeightedVoteOptions{{
			Option: govtypes.OptionAbstain,
			Weight: math.LegacyNewDec(1),
		}}
		// Some initial balances
		balancesByAddr = map[string]sdk.Coins{
			accAddr1:       sdk.NewCoins(sdk.NewInt64Coin("uatom", 100)),
			accAddr2:       sdk.NewCoins(sdk.NewInt64Coin("uatom", 200)),
			valAccAddr1Str: sdk.NewCoins(sdk.NewInt64Coin("uatom", 300)),
			valAccAddr2Str: sdk.NewCoins(sdk.NewInt64Coin("uatom", 400)),
		}
		// Account types
		accountTypesByAddr = map[string]string{
			accAddr1:       "accAddr1Type",
			accAddr2:       "accAddr2Type",
			valAccAddr1Str: "valAccAddr1Type",
			valAccAddr2Str: "valAccAddr2Type",
		}
	)
	tests := []struct {
		name             string
		delegsByAddr     map[string][]stakingtypes.Delegation
		votesByAddr      map[string]govtypes.WeightedVoteOptions
		valsByAddr       map[string]govtypes.ValidatorGovInfo
		expectedAccounts []Account
	}{
		{
			name: "no delegation",
			expectedAccounts: []Account{
				{
					Address:      accAddr1,
					Type:         "accAddr1Type",
					LiquidAmount: math.LegacyNewDec(100),
					StakedAmount: math.LegacyZeroDec(),
				},
				{
					Address:      accAddr2,
					Type:         "accAddr2Type",
					LiquidAmount: math.LegacyNewDec(200),
					StakedAmount: math.LegacyZeroDec(),
				},
				{
					Address:      valAccAddr1Str,
					Type:         "valAccAddr1Type",
					LiquidAmount: math.LegacyNewDec(300),
					StakedAmount: math.LegacyZeroDec(),
				},
				{
					Address:      valAccAddr2Str,
					Type:         "valAccAddr2Type",
					LiquidAmount: math.LegacyNewDec(400),
					StakedAmount: math.LegacyZeroDec(),
				},
			},
		},
		{
			name: "one delegation: inactive validator",
			delegsByAddr: map[string][]stakingtypes.Delegation{
				accAddr1: {
					newDeleg(accAddr1, valAddr1Str, 1000),
				},
			},
			expectedAccounts: []Account{
				{
					Address:      accAddr1,
					Type:         "accAddr1Type",
					LiquidAmount: math.LegacyNewDec(100),
					StakedAmount: math.LegacyZeroDec(),
				},
				{
					Address:      accAddr2,
					Type:         "accAddr2Type",
					LiquidAmount: math.LegacyNewDec(200),
					StakedAmount: math.LegacyZeroDec(),
				},
				{
					Address:      valAccAddr1Str,
					Type:         "valAccAddr1Type",
					LiquidAmount: math.LegacyNewDec(300),
					StakedAmount: math.LegacyZeroDec(),
				},
				{
					Address:      valAccAddr2Str,
					Type:         "valAccAddr2Type",
					LiquidAmount: math.LegacyNewDec(400),
					StakedAmount: math.LegacyZeroDec(),
				},
			},
		},
		{
			name: "one delegation: nobody voted",
			delegsByAddr: map[string][]stakingtypes.Delegation{
				accAddr1: {
					newDeleg(accAddr1, valAddr1Str, 1000),
				},
				valAccAddr1Str: { // self-delegation
					newDeleg(valAccAddr1Str, valAddr1Str, 1000000-1000),
				},
			},
			valsByAddr: map[string]govtypes.ValidatorGovInfo{
				valAddr1Str: newVal(valAddr1, 1000000, 1000000, noVote),
			},
			expectedAccounts: []Account{
				{
					Address:      accAddr1,
					Type:         "accAddr1Type",
					LiquidAmount: math.LegacyNewDec(100),
					StakedAmount: math.LegacyNewDec(1000),
					Delegations: []Delegation{{
						ValidatorAddress: valAddr1Str,
						Amount:           math.LegacyNewDec(1000),
					}},
				},
				{
					Address:      valAccAddr1Str,
					Type:         "valAccAddr1Type",
					LiquidAmount: math.LegacyNewDec(300),
					StakedAmount: math.LegacyNewDec(1000000 - 1000),
					Delegations: []Delegation{{
						ValidatorAddress: valAddr1Str,
						Amount:           math.LegacyNewDec(1000000 - 1000),
					}},
				},
				{
					Address:      accAddr2,
					Type:         "accAddr2Type",
					LiquidAmount: math.LegacyNewDec(200),
					StakedAmount: math.LegacyZeroDec(),
				},
				{
					Address:      valAccAddr2Str,
					Type:         "valAccAddr2Type",
					LiquidAmount: math.LegacyNewDec(400),
					StakedAmount: math.LegacyZeroDec(),
				},
			},
		},
		{
			name: "one delegation: inherit validator vote",
			delegsByAddr: map[string][]stakingtypes.Delegation{
				accAddr1: {
					newDeleg(accAddr1, valAddr1Str, 1000),
				},
				valAccAddr1Str: { // self-delegation
					newDeleg(valAccAddr1Str, valAddr1Str, 1000000-1000),
				},
			},
			valsByAddr: map[string]govtypes.ValidatorGovInfo{
				valAddr1Str: newVal(valAddr1, 1000000, 1000000, voteNo),
			},
			votesByAddr: map[string]govtypes.WeightedVoteOptions{
				valAccAddr1Str: voteNo,
			},
			expectedAccounts: []Account{
				{
					Address:      accAddr1,
					Type:         "accAddr1Type",
					LiquidAmount: math.LegacyNewDec(100),
					StakedAmount: math.LegacyNewDec(1000),
					Delegations: []Delegation{{
						ValidatorAddress: valAddr1Str,
						Amount:           math.LegacyNewDec(1000),
						Vote:             voteNo,
					}},
				},
				{
					Address:      valAccAddr1Str,
					Type:         "valAccAddr1Type",
					LiquidAmount: math.LegacyNewDec(300),
					StakedAmount: math.LegacyNewDec(1000000 - 1000),
					Vote:         voteNo,
					Delegations: []Delegation{{
						ValidatorAddress: valAddr1Str,
						Amount:           math.LegacyNewDec(1000000 - 1000),
						Vote:             voteNo,
					}},
				},
				{
					Address:      accAddr2,
					Type:         "accAddr2Type",
					LiquidAmount: math.LegacyNewDec(200),
					StakedAmount: math.LegacyZeroDec(),
				},
				{
					Address:      valAccAddr2Str,
					Type:         "valAccAddr2Type",
					LiquidAmount: math.LegacyNewDec(400),
					StakedAmount: math.LegacyZeroDec(),
				},
			},
		},
		{
			name: "one delegation: inherit validator vote (no self-delegation)",
			delegsByAddr: map[string][]stakingtypes.Delegation{
				accAddr1: {
					newDeleg(accAddr1, valAddr1Str, 1000),
				},
			},
			valsByAddr: map[string]govtypes.ValidatorGovInfo{
				valAddr1Str: newVal(valAddr1, 1000, 1000, voteNo),
			},
			votesByAddr: map[string]govtypes.WeightedVoteOptions{
				valAccAddr1Str: voteNo,
			},
			expectedAccounts: []Account{
				{
					Address:      accAddr1,
					Type:         "accAddr1Type",
					LiquidAmount: math.LegacyNewDec(100),
					StakedAmount: math.LegacyNewDec(1000),
					Delegations: []Delegation{{
						ValidatorAddress: valAddr1Str,
						Amount:           math.LegacyNewDec(1000),
						Vote:             voteNo,
					}},
				},
				{
					Address:      accAddr2,
					Type:         "accAddr2Type",
					LiquidAmount: math.LegacyNewDec(200),
					StakedAmount: math.LegacyZeroDec(),
				},
				{
					Address:      valAccAddr1Str,
					Type:         "valAccAddr1Type",
					LiquidAmount: math.LegacyNewDec(300),
					StakedAmount: math.LegacyZeroDec(),
				},
				{
					Address:      valAccAddr2Str,
					Type:         "valAccAddr2Type",
					LiquidAmount: math.LegacyNewDec(400),
					StakedAmount: math.LegacyZeroDec(),
				},
			},
		},
		{
			name: "one delegation: voted",
			delegsByAddr: map[string][]stakingtypes.Delegation{
				accAddr1: {
					newDeleg(accAddr1, valAddr1Str, 1000),
				},
				valAccAddr1Str: { // self-delegation
					newDeleg(valAccAddr1Str, valAddr1Str, 1000000-1000),
				},
			},
			valsByAddr: map[string]govtypes.ValidatorGovInfo{
				valAddr1Str: newVal(valAddr1, 1000000, 1000000, voteNo),
			},
			votesByAddr: map[string]govtypes.WeightedVoteOptions{
				accAddr1:       voteYes,
				valAccAddr1Str: voteNo,
			},
			expectedAccounts: []Account{
				{
					Address:      accAddr1,
					Type:         "accAddr1Type",
					LiquidAmount: math.LegacyNewDec(100),
					StakedAmount: math.LegacyNewDec(1000),
					Vote:         voteYes,
					Delegations: []Delegation{{
						ValidatorAddress: valAddr1Str,
						Amount:           math.LegacyNewDec(1000),
						Vote:             voteNo,
					}},
				},
				{
					Address:      valAccAddr1Str,
					Type:         "valAccAddr1Type",
					LiquidAmount: math.LegacyNewDec(300),
					StakedAmount: math.LegacyNewDec(1000000 - 1000),
					Vote:         voteNo,
					Delegations: []Delegation{{
						ValidatorAddress: valAddr1Str,
						Amount:           math.LegacyNewDec(1000000 - 1000),
						Vote:             voteNo,
					}},
				},
				{
					Address:      accAddr2,
					Type:         "accAddr2Type",
					LiquidAmount: math.LegacyNewDec(200),
					StakedAmount: math.LegacyZeroDec(),
				},
				{
					Address:      valAccAddr2Str,
					Type:         "valAccAddr2Type",
					LiquidAmount: math.LegacyNewDec(400),
					StakedAmount: math.LegacyZeroDec(),
				},
			},
		},
		{
			name: "one account with multiple delegations: inherit validator votes",
			delegsByAddr: map[string][]stakingtypes.Delegation{
				accAddr1: {
					newDeleg(accAddr1, valAddr1Str, 1000),
					newDeleg(accAddr1, valAddr2Str, 2000),
				},
				valAccAddr1Str: { // self-delegation
					newDeleg(valAccAddr1Str, valAddr1Str, 1000000-1000),
				},
				valAccAddr2Str: { // self-delegation
					newDeleg(valAccAddr2Str, valAddr2Str, 2000000-2000),
				},
			},
			votesByAddr: map[string]govtypes.WeightedVoteOptions{
				valAccAddr1Str: voteNo,
				valAccAddr2Str: voteYes,
			},
			valsByAddr: map[string]govtypes.ValidatorGovInfo{
				valAddr1Str: newVal(valAddr1, 1000000, 1000000, voteNo),
				valAddr2Str: newVal(valAddr2, 2000000, 2000000, voteYes),
			},
			expectedAccounts: []Account{
				{
					Address:      accAddr1,
					Type:         "accAddr1Type",
					LiquidAmount: math.LegacyNewDec(100),
					StakedAmount: math.LegacyNewDec(3000),
					Delegations: []Delegation{
						{
							ValidatorAddress: valAddr1Str,
							Amount:           math.LegacyNewDec(1000),
							Vote:             voteNo,
						},
						{
							ValidatorAddress: valAddr2Str,
							Amount:           math.LegacyNewDec(2000),
							Vote:             voteYes,
						},
					},
				},
				{
					Address:      valAccAddr1Str,
					Type:         "valAccAddr1Type",
					LiquidAmount: math.LegacyNewDec(300),
					StakedAmount: math.LegacyNewDec(1000000 - 1000),
					Vote:         voteNo,
					Delegations: []Delegation{{
						ValidatorAddress: valAddr1Str,
						Amount:           math.LegacyNewDec(1000000 - 1000),
						Vote:             voteNo,
					}},
				},
				{
					Address:      valAccAddr2Str,
					Type:         "valAccAddr2Type",
					LiquidAmount: math.LegacyNewDec(400),
					StakedAmount: math.LegacyNewDec(2000000 - 2000),
					Vote:         voteYes,
					Delegations: []Delegation{{
						ValidatorAddress: valAddr2Str,
						Amount:           math.LegacyNewDec(2000000 - 2000),
						Vote:             voteYes,
					}},
				},
				{
					Address:      accAddr2,
					Type:         "accAddr2Type",
					LiquidAmount: math.LegacyNewDec(200),
					StakedAmount: math.LegacyZeroDec(),
				},
			},
		},
		{
			name: "one account with multiple delegations: voted",
			delegsByAddr: map[string][]stakingtypes.Delegation{
				accAddr1: {
					newDeleg(accAddr1, valAddr1Str, 1000),
					newDeleg(accAddr1, valAddr2Str, 2000),
				},
				valAccAddr1Str: { // self-delegation
					newDeleg(valAccAddr1Str, valAddr1Str, 1000000-1000),
				},
				valAccAddr2Str: { // self-delegation
					newDeleg(valAccAddr2Str, valAddr2Str, 2000000-2000),
				},
			},
			valsByAddr: map[string]govtypes.ValidatorGovInfo{
				valAddr1Str: newVal(valAddr1, 1000000, 1000000, voteNo),
				valAddr2Str: newVal(valAddr2, 2000000, 2000000, voteYes),
			},
			votesByAddr: map[string]govtypes.WeightedVoteOptions{
				accAddr1:       voteAbstain,
				valAccAddr1Str: voteNo,
				valAccAddr2Str: voteYes,
			},
			expectedAccounts: []Account{
				{
					Address:      accAddr1,
					Type:         "accAddr1Type",
					LiquidAmount: math.LegacyNewDec(100),
					StakedAmount: math.LegacyNewDec(3000),
					Vote:         voteAbstain,
					Delegations: []Delegation{
						{
							ValidatorAddress: valAddr1Str,
							Amount:           math.LegacyNewDec(1000),
							Vote:             voteNo,
						},
						{
							ValidatorAddress: valAddr2Str,
							Amount:           math.LegacyNewDec(2000),
							Vote:             voteYes,
						},
					},
				},
				{
					Address:      valAccAddr1Str,
					Type:         "valAccAddr1Type",
					LiquidAmount: math.LegacyNewDec(300),
					StakedAmount: math.LegacyNewDec(1000000 - 1000),
					Vote:         voteNo,
					Delegations: []Delegation{{
						ValidatorAddress: valAddr1Str,
						Amount:           math.LegacyNewDec(1000000 - 1000),
						Vote:             voteNo,
					}},
				},
				{
					Address:      valAccAddr2Str,
					Type:         "valAccAddr2Type",
					LiquidAmount: math.LegacyNewDec(400),
					StakedAmount: math.LegacyNewDec(2000000 - 2000),
					Vote:         voteYes,
					Delegations: []Delegation{{
						ValidatorAddress: valAddr2Str,
						Amount:           math.LegacyNewDec(2000000 - 2000),
						Vote:             voteYes,
					}},
				},
				{
					Address:      accAddr2,
					Type:         "accAddr2Type",
					LiquidAmount: math.LegacyNewDec(200),
					StakedAmount: math.LegacyZeroDec(),
				},
			},
		},
		{
			name: "multiple accounts with multiple delegations",
			delegsByAddr: map[string][]stakingtypes.Delegation{
				accAddr1: {
					newDeleg(accAddr1, valAddr1Str, 1000),
					newDeleg(accAddr1, valAddr2Str, 2000),
				},
				accAddr2: {
					newDeleg(accAddr2, valAddr1Str, 3000),
					newDeleg(accAddr2, valAddr2Str, 4000),
				},
				valAccAddr1Str: { // self-delegation
					newDeleg(valAccAddr1Str, valAddr1Str, 1000000-1000-3000),
				},
				// No self-delegation for valAccAddr2Str
			},
			valsByAddr: map[string]govtypes.ValidatorGovInfo{
				valAddr1Str: newVal(valAddr1, 1000000, 1000000, voteNo),
				valAddr2Str: newVal(valAddr2, 2000+4000, 2000+4000, voteYes),
			},
			votesByAddr: map[string]govtypes.WeightedVoteOptions{
				accAddr1:       voteAbstain,
				valAccAddr1Str: voteNo,
				valAccAddr2Str: voteYes,
			},
			expectedAccounts: []Account{
				{
					Address:      accAddr1,
					Type:         "accAddr1Type",
					LiquidAmount: math.LegacyNewDec(100),
					StakedAmount: math.LegacyNewDec(1000 + 2000),
					Vote:         voteAbstain,
					Delegations: []Delegation{
						{
							ValidatorAddress: valAddr1Str,
							Amount:           math.LegacyNewDec(1000),
							Vote:             voteNo,
						},
						{
							ValidatorAddress: valAddr2Str,
							Amount:           math.LegacyNewDec(2000),
							Vote:             voteYes,
						},
					},
				},
				{
					Address:      accAddr2,
					Type:         "accAddr2Type",
					LiquidAmount: math.LegacyNewDec(200),
					StakedAmount: math.LegacyNewDec(3000 + 4000),
					Delegations: []Delegation{
						{
							ValidatorAddress: valAddr1Str,
							Amount:           math.LegacyNewDec(3000),
							Vote:             voteNo,
						},
						{
							ValidatorAddress: valAddr2Str,
							Amount:           math.LegacyNewDec(4000),
							Vote:             voteYes,
						},
					},
				},
				{
					Address:      valAccAddr1Str,
					Type:         "valAccAddr1Type",
					LiquidAmount: math.LegacyNewDec(300),
					StakedAmount: math.LegacyNewDec(1000000 - 1000 - 3000),
					Vote:         voteNo,
					Delegations: []Delegation{{
						ValidatorAddress: valAddr1Str,
						Amount:           math.LegacyNewDec(1000000 - 1000 - 3000),
						Vote:             voteNo,
					}},
				},
				{
					Address:      valAccAddr2Str,
					Type:         "valAccAddr2Type",
					LiquidAmount: math.LegacyNewDec(400),
					StakedAmount: math.LegacyZeroDec(),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			require := require.New(t)

			accounts := getAccounts(tt.delegsByAddr, tt.votesByAddr, tt.valsByAddr, balancesByAddr, accountTypesByAddr)

			// order is not determistic, sort to have it
			sort.Slice(accounts, func(i, j int) bool {
				return accounts[i].Address > accounts[j].Address
			})
			sort.Slice(tt.expectedAccounts, func(i, j int) bool {
				return tt.expectedAccounts[i].Address > tt.expectedAccounts[j].Address
			})
			exBz, err := json.Marshal(tt.expectedAccounts)
			require.NoError(err)
			bz, err := json.Marshal(accounts)
			require.NoError(err)
			// Invoked assert.JSONEq because it has a more readable output
			assert.JSONEq(string(exBz), string(bz))
		})
	}
}

func createAccountAddrs(accNum int) []sdk.AccAddress {
	addrs := make([]sdk.AccAddress, accNum)
	for i := 0; i < accNum; i++ {
		pk := ed25519.GenPrivKey().PubKey()
		addrs[i] = sdk.AccAddress(pk.Address())
	}
	return addrs
}

func createValidatorAddrs(addrNum int) []sdk.ValAddress {
	addrs := make([]sdk.ValAddress, addrNum)
	for i := 0; i < addrNum; i++ {
		pk := ed25519.GenPrivKey().PubKey()
		addrs[i] = sdk.ValAddress(pk.Address())
	}
	return addrs
}
