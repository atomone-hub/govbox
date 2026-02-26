package main

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sort"

	h "github.com/dustin/go-humanize"

	tmjson "github.com/cometbft/cometbft/libs/json"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

func autoStaking(genesisPath string) error {
	bz, err := os.ReadFile(genesisPath)
	if err != nil {
		return fmt.Errorf("readfile %s: %w", genesisPath, err)
	}
	var genesisState map[string]json.RawMessage
	if err := tmjson.Unmarshal(bz, &genesisState); err != nil {
		return fmt.Errorf("tmjson.Unmarshal: %w", err)
	}
	var appState map[string]json.RawMessage
	if err := tmjson.Unmarshal(genesisState["app_state"], &appState); err != nil {
		return fmt.Errorf("tmjson.Unmarshal appstate: %w", err)
	}
	// get all balances
	var bankGenesisState banktypes.GenesisState
	err = tmjson.Unmarshal(appState["bank"], &bankGenesisState)
	if err != nil {
		return fmt.Errorf("unmarshal auth: %w", err)
	}
	var (
		minTokens           = math.NewInt(25_000_000)
		supply              = math.ZeroInt()
		totalStake          = math.ZeroInt()
		numStakes           = 0
		validatorLen        = 30
		validators          = make([]int64, validatorLen)
		stakeds             int64
		stakes              = make(map[int]int)
		stakeSplitCondition = math.NewInt(1_000_000_000_000)

		// This algorithm splits the stake into parts and stake those parts one by
		// one into the validator that has the less stake.
		basicAlgo = func(balIdx int, stake math.Int) {
			// to prevent staking multiple times over the same validator
			// adjust split amount for the whale account
			splitStake := math.NewInt(1)
			switch {
			case stake.LT(math.NewInt(500_000_000)):
				splitStake = stake.QuoRaw(5)
			case stake.LT(math.NewInt(10_000_000_000)):
				splitStake = stake.QuoRaw(10)
			default:
				splitStake = stake.QuoRaw(20)
			}

			for ; stake.GTE(sdk.DefaultPowerReduction); stake = stake.Sub(splitStake) {
				// find validator which has the less stake
				valIdx := slices.Index(validators, slices.Min(validators))

				staked := math.MinInt(stake, splitStake).Int64()
				stakeds += staked
				validators[valIdx] += staked
				stakes[balIdx]++
				// if balIdx == 0 || balIdx == 663 || balIdx == 1 || balIdx == 662 {
				// fmt.Println(balIdx, "stake", humani(staked), "valIdx", valIdx)
				// }
			}
		}

		// staking distrib from terra
		// https://github.com/terra-money/core/blob/release/v2.0/app/app.go#L841
		valIdx    = 0
		terraAlgo = func(balIdx int, stake math.Int) {
			// to prevent staking multiple times over the same validator
			// adjust split amount for the whale account
			splitStake := stakeSplitCondition
			if stake.GT(stakeSplitCondition.MulRaw(int64(validatorLen))) {
				splitStake = stake.QuoRaw(int64(validatorLen))
			}
			// if a vesting account has more staking token than `stakeSplitCondition`,
			// split staking balance to distribute staking power evenly
			// Ex) 2_200_000_000_000
			// stake 1_000_000_000_000 to val1
			// stake 1_000_000_000_000 to val2
			// stake 200_000_000_000 to val3
			for ; stake.GTE(sdk.DefaultPowerReduction); stake = stake.Sub(splitStake) {
				staked := math.MinInt(stake, splitStake).Int64()
				stakeds += staked
				validators[valIdx%validatorLen] += staked
				stakes[balIdx]++

				// increase index only when staking happened
				valIdx++
			}
		}
	)
	bals := bankGenesisState.Balances
	sort.Slice(bals, func(i, j int) bool {
		return bals[i].Coins.IsAllGT(bals[j].Coins)
	})
	for i := 0; i < 4; i++ {
		fmt.Println("BAL", i, human(bals[i].Coins.AmountOf("ugovgen")))
	}

	_, _ = terraAlgo, basicAlgo
	for balIdx, bal := range bals {
		tokens := bal.Coins.AmountOf("ugovgen")
		supply = supply.Add(tokens)
		if tokens.LTE(minTokens) {
			// Don't stake when tokens < minToken
			continue
		}
		numStakes++
		// take 50%
		stake := tokens.QuoRaw(2)
		totalStake = totalStake.Add(stake)

		basicAlgo(balIdx, stake)
	}
	// for k, v := range stakes {
	// if v > 5 {
	// fmt.Println("STAKE", k, v)
	// }
	// }
	for i := 0; i < validatorLen; i++ {
		fmt.Println("VAL", i, humani(validators[i]))
	}
	fmt.Println("count", h.Comma(int64(numStakes)), h.Comma(int64(len(bankGenesisState.Balances))))
	fmt.Println("amount", human(totalStake), humani(stakeds), human(supply))
	fmt.Println("staking ratio", totalStake.ToLegacyDec().Quo(supply.ToLegacyDec()))
	return nil
}
