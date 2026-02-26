package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/math"

	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
)

func TestDistribution(t *testing.T) {
	var (
		voteYes = govtypes.WeightedVoteOptions{{
			Option: govtypes.OptionYes,
			Weight: math.LegacyNewDec(1),
		}}
		voteAbstain = govtypes.WeightedVoteOptions{{
			Option: govtypes.OptionAbstain,
			Weight: math.LegacyNewDec(1),
		}}
		voteNo = govtypes.WeightedVoteOptions{{
			Option: govtypes.OptionNo,
			Weight: math.LegacyNewDec(1),
		}}
		voteNoWithVeto = govtypes.WeightedVoteOptions{{
			Option: govtypes.OptionNoWithVeto,
			Weight: math.LegacyNewDec(1),
		}}
		noVotesMultiplier = defaultDistriParams().noVotesMultiplier
		bonus             = defaultDistriParams().bonus
		malus             = defaultDistriParams().malus
	)

	tests := []struct {
		name              string
		accounts          []Account
		expectedAddresses func(math.LegacyDec) map[string]math.LegacyDec
		expectedTotal     int64
		expectedUnstaked  int64
		expectedVotes     map[govtypes.VoteOption]int64
	}{
		{
			name: "direct votes",
			accounts: []Account{
				{
					Address:      "yes",
					LiquidAmount: math.LegacyNewDec(10),
					StakedAmount: math.LegacyNewDec(20),
					Vote:         voteYes,
				},
				{
					Address:      "abstain",
					LiquidAmount: math.LegacyNewDec(10),
					StakedAmount: math.LegacyNewDec(20),
					Vote:         voteAbstain,
				},
				{
					Address:      "no",
					LiquidAmount: math.LegacyNewDec(10),
					StakedAmount: math.LegacyNewDec(20),
					Vote:         voteNo,
				},
				{
					Address:      "noWithVeto",
					LiquidAmount: math.LegacyNewDec(10),
					StakedAmount: math.LegacyNewDec(20),
					Vote:         voteNoWithVeto,
				},
				{
					Address:      "didntVote",
					LiquidAmount: math.LegacyNewDec(10),
					StakedAmount: math.LegacyNewDec(20),
					Delegations: []Delegation{{
						Amount: math.LegacyNewDec(20),
					}},
				},
			},
			expectedAddresses: func(nonVotersMult math.LegacyDec) map[string]math.LegacyDec {
				return map[string]math.LegacyDec{
					"yes":        math.LegacyNewDec(1).Mul(nonVotersMult.Mul(malus)).Add(math.LegacyNewDec(2)),
					"abstain":    math.LegacyNewDec(1).Mul(nonVotersMult.Mul(malus)).Add(math.LegacyNewDec(2).Mul(nonVotersMult)),
					"no":         math.LegacyNewDec(1).Mul(nonVotersMult.Mul(malus)).Add(math.LegacyNewDec(2).Mul(noVotesMultiplier)),
					"noWithVeto": math.LegacyNewDec(1).Mul(nonVotersMult.Mul(malus)).Add(math.LegacyNewDec(2).Mul(noVotesMultiplier).Mul(bonus)),
					"didntVote":  math.LegacyNewDec(1).Mul(nonVotersMult.Mul(malus)).Add(math.LegacyNewDec(2).Mul(nonVotersMult).Mul(malus)),
				}
			},
			expectedTotal:    57,
			expectedUnstaked: 10,
			expectedVotes: map[govtypes.VoteOption]int64{
				govtypes.OptionEmpty:      4,
				govtypes.OptionYes:        2,
				govtypes.OptionAbstain:    4,
				govtypes.OptionNo:         18,
				govtypes.OptionNoWithVeto: 19,
			},
		},
		{
			name: "direct votes with small bags",
			accounts: []Account{
				{
					Address:      "yes",
					LiquidAmount: math.LegacyNewDec(1),
					StakedAmount: math.LegacyNewDec(2),
					Vote:         voteYes,
				},
				{
					Address:      "abstain",
					LiquidAmount: math.LegacyNewDec(1),
					StakedAmount: math.LegacyNewDec(2),
					Vote:         voteAbstain,
				},
				{
					Address:      "no",
					LiquidAmount: math.LegacyNewDec(1),
					StakedAmount: math.LegacyNewDec(2),
					Vote:         voteNo,
				},
				{
					Address:      "noWithVeto",
					LiquidAmount: math.LegacyNewDec(1),
					StakedAmount: math.LegacyNewDec(2),
					Vote:         voteNoWithVeto,
				},
				{
					Address:      "didntVote",
					LiquidAmount: math.LegacyNewDec(1),
					StakedAmount: math.LegacyNewDec(2),
					Delegations: []Delegation{{
						Amount: math.LegacyNewDec(2),
					}},
				},
			},
			expectedAddresses: func(nonVotersMult math.LegacyDec) map[string]math.LegacyDec {
				return map[string]math.LegacyDec{
					"no":         math.LegacyNewDec(1).Mul(nonVotersMult.Mul(malus)).Add(math.LegacyNewDec(2).Mul(noVotesMultiplier)).QuoInt64(10),
					"noWithVeto": math.LegacyNewDec(1).Mul(nonVotersMult.Mul(malus)).Add(math.LegacyNewDec(2).Mul(noVotesMultiplier).Mul(bonus)).QuoInt64(10),
					"abstain":    math.LegacyNewDec(1),
					"didntVote":  math.LegacyNewDec(1),
				}
			},
			expectedTotal:    6,
			expectedUnstaked: 1,
			expectedVotes: map[govtypes.VoteOption]int64{
				govtypes.OptionEmpty:      0,
				govtypes.OptionYes:        0,
				govtypes.OptionAbstain:    0,
				govtypes.OptionNo:         2,
				govtypes.OptionNoWithVeto: 2,
			},
		},
		{
			name: "direct weighted votes",
			accounts: []Account{
				{
					Address:      "directWeightVote",
					LiquidAmount: math.LegacyNewDec(10),
					StakedAmount: math.LegacyNewDec(180),
					Vote: govtypes.WeightedVoteOptions{
						{
							Option: govtypes.OptionYes,
							Weight: math.LegacyNewDecWithPrec(1, 1),
						},
						{
							Option: govtypes.OptionAbstain,
							Weight: math.LegacyNewDecWithPrec(2, 1),
						},
						{
							Option: govtypes.OptionNo,
							Weight: math.LegacyNewDecWithPrec(3, 1),
						},
						{
							Option: govtypes.OptionNoWithVeto,
							Weight: math.LegacyNewDecWithPrec(4, 1),
						},
					},
				},
			},
			expectedAddresses: func(nonVotersMult math.LegacyDec) map[string]math.LegacyDec {
				return map[string]math.LegacyDec{
					"directWeightVote":
					// liquid amount
					math.LegacyNewDec(1).Mul(nonVotersMult.Mul(malus)).
						// voted yes
						Add(math.LegacyNewDec(18).Mul(math.LegacyNewDecWithPrec(1, 1))).
						// voted abstain
						Add(math.LegacyNewDec(18).Mul(math.LegacyNewDecWithPrec(2, 1)).Mul(nonVotersMult)).
						// voted no
						Add(math.LegacyNewDec(18).Mul(math.LegacyNewDecWithPrec(3, 1)).Mul(noVotesMultiplier)).
						// voted noWithVeto
						Add(math.LegacyNewDec(18).Mul(math.LegacyNewDecWithPrec(4, 1)).Mul(noVotesMultiplier).Mul(bonus)),
				}
			},
			expectedTotal:    174,
			expectedUnstaked: 12,
			expectedVotes: map[govtypes.VoteOption]int64{
				govtypes.OptionEmpty:      0,
				govtypes.OptionYes:        2,
				govtypes.OptionAbstain:    44,
				govtypes.OptionNo:         49,
				govtypes.OptionNoWithVeto: 67,
			},
		},
		{
			name: "indirect votes",
			accounts: []Account{
				{
					Address:      "indirectVote",
					LiquidAmount: math.LegacyNewDec(10),
					StakedAmount: math.LegacyNewDec(200),
					Vote:         nil,
					Delegations: []Delegation{
						// one deleg didn't vote
						{
							Amount: math.LegacyNewDec(20),
							Vote:   nil,
						},
						// one deleg voted yes
						{
							Amount: math.LegacyNewDec(30),
							Vote:   voteYes,
						},
						// one deleg voted abstain
						{
							Amount: math.LegacyNewDec(40),
							Vote:   voteAbstain,
						},
						// one deleg voted no
						{
							Amount: math.LegacyNewDec(50),
							Vote:   voteNo,
						},
						// one deleg voted noWithVeto
						{
							Amount: math.LegacyNewDec(60),
							Vote:   voteNoWithVeto,
						},
					},
				},
			},
			expectedAddresses: func(nonVotersMult math.LegacyDec) map[string]math.LegacyDec {
				return map[string]math.LegacyDec{
					"indirectVote":
					// liquid amount
					math.LegacyNewDec(1).Mul(nonVotersMult.Mul(malus)).
						// from deleg who didn't vote
						Add(math.LegacyNewDec(2).Mul(nonVotersMult).Mul(malus)).
						// from deleg who voted yes
						Add(math.LegacyNewDec(3)).
						// from deleg who voted abstain
						Add(math.LegacyNewDec(4).Mul(nonVotersMult)).
						// from deleg who voted no
						Add(math.LegacyNewDec(5).Mul(noVotesMultiplier)).
						// from deleg who voted noWithVeto
						Add(math.LegacyNewDec(6).Mul(noVotesMultiplier).Mul(bonus)),
				}
			},
			expectedTotal:    153,
			expectedUnstaked: 7,
			expectedVotes: map[govtypes.VoteOption]int64{
				govtypes.OptionEmpty:      14,
				govtypes.OptionYes:        3,
				govtypes.OptionAbstain:    29,
				govtypes.OptionNo:         45,
				govtypes.OptionNoWithVeto: 56,
			},
		},
		{
			name: "indirect weighted votes",
			accounts: []Account{
				{
					Address:      "directWeightVote",
					LiquidAmount: math.LegacyNewDec(10),
					StakedAmount: math.LegacyNewDec(330),
					Vote:         nil,
					Delegations: []Delegation{
						// one deleg used a weighted vote
						{
							Amount: math.LegacyNewDec(180),
							Vote: govtypes.WeightedVoteOptions{
								{
									Option: govtypes.OptionYes,
									Weight: math.LegacyNewDecWithPrec(1, 1),
								},
								{
									Option: govtypes.OptionAbstain,
									Weight: math.LegacyNewDecWithPrec(2, 1),
								},
								{
									Option: govtypes.OptionNo,
									Weight: math.LegacyNewDecWithPrec(3, 1),
								},
								{
									Option: govtypes.OptionNoWithVeto,
									Weight: math.LegacyNewDecWithPrec(4, 1),
								},
							},
						},
						// one other deleg used a weighted vote
						{
							Amount: math.LegacyNewDec(100),
							Vote: govtypes.WeightedVoteOptions{
								{
									Option: govtypes.OptionYes,
									Weight: math.LegacyNewDecWithPrec(4, 1),
								},
								{
									Option: govtypes.OptionAbstain,
									Weight: math.LegacyNewDecWithPrec(6, 1),
								},
							},
						},
						// one deleg voted no
						{
							Amount: math.LegacyNewDec(20),
							Vote:   voteNo,
						},
						// one deleg didn't vote
						{
							Amount: math.LegacyNewDec(30),
							Vote:   nil,
						},
					},
				},
			},
			expectedAddresses: func(nonVotersMult math.LegacyDec) map[string]math.LegacyDec {
				return map[string]math.LegacyDec{
					"directWeightVote":
					// liquid amount
					math.LegacyNewDec(1).Mul(nonVotersMult.Mul(malus)).
						// voted yes
						Add(math.LegacyNewDec(18).Mul(math.LegacyNewDecWithPrec(1, 1))).
						Add(math.LegacyNewDec(10).Mul(math.LegacyNewDecWithPrec(4, 1))).
						// voted abstain
						Add(math.LegacyNewDec(18).Mul(math.LegacyNewDecWithPrec(2, 1)).Mul(nonVotersMult)).
						Add(math.LegacyNewDec(10).Mul(math.LegacyNewDecWithPrec(6, 1)).Mul(nonVotersMult)).
						// voted no
						Add(math.LegacyNewDec(18).Mul(math.LegacyNewDecWithPrec(3, 1)).Mul(noVotesMultiplier)).
						Add(math.LegacyNewDec(2).Mul(noVotesMultiplier)).
						// voted noWithVeto
						Add(math.LegacyNewDec(18).Mul(math.LegacyNewDecWithPrec(4, 1)).Mul(noVotesMultiplier).Mul(bonus)).
						// didn't vote
						Add(math.LegacyNewDec(3).Mul(nonVotersMult.Mul(malus))),
				}
			},
			expectedTotal:    206,
			expectedUnstaked: 5,
			expectedVotes: map[govtypes.VoteOption]int64{
				govtypes.OptionEmpty:      14,
				govtypes.OptionYes:        6,
				govtypes.OptionAbstain:    48,
				govtypes.OptionNo:         67,
				govtypes.OptionNoWithVeto: 67,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			assert := assert.New(t)

			airdrop, err := distribution(tt.accounts, defaultDistriParams(), "")

			require.NoError(err)
			expectedRes := tt.expectedAddresses(airdrop.nonVotersMultiplier)
			assert.Equal(len(expectedRes), len(airdrop.addresses), "unexpected number of res")
			for k, v := range airdrop.addresses {
				ev, ok := expectedRes[k]
				if assert.True(ok, "unexpected address '%s' balance '%s'", k, v) {
					assert.Equal(ev.RoundInt64(), v.Int64(), "unexpected airdrop amount for address '%s'", k)
				}
			}
			assert.Equal(tt.expectedTotal, airdrop.atone.supply.RoundInt64(), "unexpected airdrop.total")
			assert.Equal(tt.expectedUnstaked, airdrop.atone.unstaked.RoundInt64(), "unexpected airdrop.unstaked")
			for _, v := range allVoteOptions {
				assert.Equal(tt.expectedVotes[v], airdrop.atone.votes[v].RoundInt64(), "unexpected airdrop.votes[%s]", v)
			}
		})
	}
}
