package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func TestComputeDistribution(t *testing.T) {
	var (
		accAddrs = createAccountAddrs(2)
		accAddr1 = accAddrs[0]
		// voterAddr2 = accAddrs[1]
		valAddrs    = createValidatorAddrs(2)
		valAddr1    = valAddrs[0]
		valAccAddr1 = sdk.AccAddress(valAddr1)
		// valAddr2   = valAddrs[1]
		noChangeBalanceFactor = map[govtypes.VoteOption]func(sdk.Dec) sdk.Dec{
			// XXX these are basic raw examples of airdrop/slash functions
			govtypes.OptionYes:        func(d sdk.Dec) sdk.Dec { return d },
			govtypes.OptionAbstain:    func(d sdk.Dec) sdk.Dec { return d },
			govtypes.OptionNo:         func(d sdk.Dec) sdk.Dec { return d },
			govtypes.OptionNoWithVeto: func(d sdk.Dec) sdk.Dec { return d },
		}
	)
	tests := []struct {
		name                 string
		delegsByAddr         map[string][]stakingtypes.Delegation
		votesByAddr          map[string]govtypes.Vote
		valsByAddr           map[string]govtypes.ValidatorGovInfo
		balanceFactors       map[govtypes.VoteOption]func(sdk.Dec) sdk.Dec
		expectedDistribution []banktypes.Balance
	}{
		{
			name:                 "no delegation",
			expectedDistribution: []banktypes.Balance{},
		},
		{
			name: "one delegation without validator",
			delegsByAddr: map[string][]stakingtypes.Delegation{
				accAddr1.String(): {
					{
						DelegatorAddress: accAddr1.String(),
						ValidatorAddress: valAddr1.String(),
						Shares:           sdk.NewDec(1000),
					},
				},
			},
			expectedDistribution: []banktypes.Balance{},
		},
		{
			name: "one delegation with validator didn't vote",
			delegsByAddr: map[string][]stakingtypes.Delegation{
				accAddr1.String(): {
					{
						DelegatorAddress: accAddr1.String(),
						ValidatorAddress: valAddr1.String(),
						Shares:           sdk.NewDec(1000),
					},
				},
			},
			valsByAddr: map[string]govtypes.ValidatorGovInfo{
				valAddr1.String(): {
					Address:         valAddr1,
					BondedTokens:    sdk.NewInt(1000000),
					DelegatorShares: sdk.NewDec(1000000),
				},
			},
			expectedDistribution: []banktypes.Balance{},
		},
		{
			name: "one delegation with validator: inherit vote",
			delegsByAddr: map[string][]stakingtypes.Delegation{
				accAddr1.String(): {
					{
						DelegatorAddress: accAddr1.String(),
						ValidatorAddress: valAddr1.String(),
						Shares:           sdk.NewDec(1000),
					},
				},
			},
			valsByAddr: map[string]govtypes.ValidatorGovInfo{
				valAddr1.String(): {
					Address:         valAddr1,
					BondedTokens:    sdk.NewInt(1000000),
					DelegatorShares: sdk.NewDec(1000000),
				},
			},
			votesByAddr: map[string]govtypes.Vote{
				valAccAddr1.String(): {
					Options: []govtypes.WeightedVoteOption{{
						Option: govtypes.OptionYes,
						Weight: sdk.NewDec(1),
					}},
				},
			},
			balanceFactors: noChangeBalanceFactor,
			expectedDistribution: []banktypes.Balance{
				{
					Address: accAddr1.String(),
					Coins:   sdk.NewCoins(sdk.NewInt64Coin("u"+ticker, 1000)),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			balances := computeDistribution(tt.delegsByAddr, tt.votesByAddr, tt.valsByAddr, tt.balanceFactors)

			assert.Equal(t, tt.expectedDistribution, balances)
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
