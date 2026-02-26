package main

import (
	"fmt"
	"os"
	"slices"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/components"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/pkg/browser"

	"cosmossdk.io/math"

	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
)

// Some constants
var (
	// list of ICF wallets
	icfWallets = []string{
		// Source https://github.com/gnolang/bounties/issues/18#issuecomment-1034700230
		"cosmos1z8mzakma7vnaajysmtkwt4wgjqr2m84tzvyfkz",
		"cosmos1unc788q8md2jymsns24eyhua58palg5kc7cstv",
		// The 2 addresses above have been emptied in favour of the following 2
		"cosmos1sufkm72dw7ua9crpfhhp0dqpyuggtlhdse98e7",
		"cosmos1z6czaavlk6kjd48rpf58kqqw9ssad2uaxnazgl",
		// From other investigations
		"cosmos17u903qxqc6dzn3chvmc9zzp9fl4xja0pwggfj7",
	}
)

type airdrop struct {
	// params hold the distribution parameters that resulted in this airdrop
	params distriParams
	// addresses contains the airdrop amount per address.
	addresses       map[string]math.Int
	addressesDetail []addrAmtDetail
	// nonVotersMultiplier ensures that non-voters don't hold more than 1/3 of
	// the supply
	nonVotersMultiplier math.LegacyDec
	// $ATOM distribution
	atom distrib
	// $ATONE distribution
	atone distrib
	// Amount of $ATOM slashed for the ICF
	icfSlash math.LegacyDec
	// Amount minted for CP
	communityPool math.LegacyDec
	// Amount minted for reserved address
	reservedAddr math.LegacyDec
}

type addrAmtDetail struct {
	Address      string    `json:"address"`
	YesDetail    amtDetail `json:"yesDetail"`
	NoDetail     amtDetail `json:"noDetail"`
	NWVDetail    amtDetail `json:"nwvDetail"`
	AbsDetail    amtDetail `json:"absDetail"`
	DnvDetail    amtDetail `json:"dnvDetail"`
	LiquidDetail amtDetail `json:"liquidDetail"`
	Total        math.LegacyDec   `json:"total"`
}

type amtDetail struct {
	AtomAmt    math.LegacyDec `json:"atomAmt"`
	Multiplier math.LegacyDec `json:"multiplier"`
	BonusMalus math.LegacyDec `json:"bonusMalus"`
	Factor     math.LegacyDec `json:"factor"`
	AtoneAmt   math.LegacyDec `json:"atoneAmt"`
}

type distrib struct {
	// total supply of the distrib
	supply math.LegacyDec
	// votes holds the part of the distrib per vote.
	votes voteMap
	// unstaked is part of the distrib for unstaked amounts.
	unstaked math.LegacyDec
}

type distriParams struct {
	yesVotesMultiplier math.LegacyDec
	noVotesMultiplier  math.LegacyDec
	bonus              math.LegacyDec
	malus              math.LegacyDec
	supplyFactor       math.LegacyDec
	supplyMintFactor   math.LegacyDec
}

func (d distriParams) String() string {
	return fmt.Sprintf("Yes x%.1f / No x%.1f",
		d.yesVotesMultiplier.MustFloat64(), d.noVotesMultiplier.MustFloat64())
}

func defaultDistriParams() distriParams {
	return distriParams{
		yesVotesMultiplier: math.LegacyOneDec(),                    // Y get x1
		noVotesMultiplier:  math.LegacyNewDec(9),                   // N & NWV get x9
		bonus:              math.LegacyNewDecWithPrec(103, 2),      // 3% bonus
		malus:              math.LegacyNewDecWithPrec(97, 2),       // -3% malus
		supplyFactor:       math.LegacyNewDecWithPrec(1, 1),        // Decrease final supply by a factor of 10
		supplyMintFactor:   math.LegacyOneDec().Quo(math.LegacyNewDec(9)), // 1/9 of the total supply is minted for the CP and a reserved address
	}
}

func (d distrib) votePercentages() map[govtypes.VoteOption]math.LegacyDec {
	percs := make(map[govtypes.VoteOption]math.LegacyDec)
	for k, v := range d.votes {
		percs[k] = v.Quo(d.supply)
	}
	return percs
}

func distribution(accounts []Account, params distriParams, prefix string) (airdrop, error) {
	airdrop := airdrop{
		params:    params,
		addresses: make(map[string]math.Int),
		icfSlash:  math.LegacyZeroDec(),
		atom: distrib{
			supply:   math.LegacyZeroDec(),
			votes:    newVoteMap(),
			unstaked: math.LegacyZeroDec(),
		},
		atone: distrib{
			supply:   math.LegacyZeroDec(),
			votes:    newVoteMap(),
			unstaked: math.LegacyZeroDec(),
		},
	}
	for _, acc := range accounts {
		var (
			voteWeights = acc.voteWeights()

			// Detail of vote distribution for $ATOM
			yesAtomAmt        = voteWeights[govtypes.OptionYes].Mul(acc.StakedAmount)
			noAtomAmt         = voteWeights[govtypes.OptionNo].Mul(acc.StakedAmount)
			noWithVetoAtomAmt = voteWeights[govtypes.OptionNoWithVeto].Mul(acc.StakedAmount)
			abstainAtomAmt    = voteWeights[govtypes.OptionAbstain].Mul(acc.StakedAmount)
			noVoteAtomAmt     = voteWeights[govtypes.OptionEmpty].Mul(acc.StakedAmount)
		)
		// increment $ATOM votes
		airdrop.atom.votes.add(govtypes.OptionYes, yesAtomAmt)
		airdrop.atom.votes.add(govtypes.OptionNo, noAtomAmt)
		airdrop.atom.votes.add(govtypes.OptionNoWithVeto, noWithVetoAtomAmt)
		airdrop.atom.votes.add(govtypes.OptionAbstain, abstainAtomAmt)
		airdrop.atom.votes.add(govtypes.OptionEmpty, noVoteAtomAmt)
		// increment $ATOM supply
		airdrop.atom.supply = airdrop.atom.supply.Add(acc.StakedAmount.Add(acc.LiquidAmount))
		airdrop.atom.unstaked = airdrop.atom.unstaked.Add(acc.LiquidAmount)
	}

	// Compute nonVotersMultiplier to have non-voters <= 33%
	var (
		yesAtoneTotalAmt     = airdrop.atom.votes[govtypes.OptionYes].Mul(params.yesVotesMultiplier)
		noAtoneTotalAmt      = airdrop.atom.votes[govtypes.OptionNo].Add(airdrop.atom.votes[govtypes.OptionNoWithVeto]).Mul(params.noVotesMultiplier)
		noVotersAtomTotalAmt = airdrop.atom.votes[govtypes.OptionAbstain].Add(airdrop.atom.votes[govtypes.OptionEmpty]).Add(airdrop.atom.unstaked)
		targetNonVotersPerc  = math.LegacyNewDecWithPrec(33, 2)
	)
	// Formula is:
	// nonVotersMultiplier = (t x (yesAtone + noAtone)) / ((1 - t) x nonVoterAtom)
	// where t is the targetNonVotersPerc
	airdrop.nonVotersMultiplier = targetNonVotersPerc.Mul(yesAtoneTotalAmt.Add(noAtoneTotalAmt)).
		Quo((math.LegacyOneDec().Sub(targetNonVotersPerc)).Mul(noVotersAtomTotalAmt))

	for _, acc := range accounts {
		if slices.Contains(icfWallets, acc.Address) {
			// Slash ICF
			airdrop.icfSlash = airdrop.icfSlash.Add(acc.LiquidAmount).Add(acc.StakedAmount)
			continue
		}

		var (
			voteWeights       = acc.voteWeights()
			yesAtomAmt        = voteWeights[govtypes.OptionYes].Mul(acc.StakedAmount)
			noAtomAmt         = voteWeights[govtypes.OptionNo].Mul(acc.StakedAmount)
			noWithVetoAtomAmt = voteWeights[govtypes.OptionNoWithVeto].Mul(acc.StakedAmount)
			abstainAtomAmt    = voteWeights[govtypes.OptionAbstain].Mul(acc.StakedAmount)
			noVoteAtomAmt     = voteWeights[govtypes.OptionEmpty].Mul(acc.StakedAmount)
			// Apply airdrop multipliers:
			// Yes:         x yesVotesMultiplier
			// No:         	x noVotesMultiplier
			// NoWithVeto: 	x noVotesMultiplier x bonus
			// Abstain:    	x nonVotersMultiplier
			// Didn't vote: x nonVotersMultiplier x malus
			yesAirdropAmt        = yesAtomAmt.Mul(params.yesVotesMultiplier).Mul(params.supplyFactor)
			noAirdropAmt         = noAtomAmt.Mul(params.noVotesMultiplier).Mul(params.supplyFactor)
			noWithVetoAirdropAmt = noWithVetoAtomAmt.Mul(params.noVotesMultiplier).Mul(params.bonus).Mul(params.supplyFactor)
			abstainAirdropAmt    = abstainAtomAmt.Mul(airdrop.nonVotersMultiplier).Mul(params.supplyFactor)
			noVoteAirdropAmt     = noVoteAtomAmt.Mul(airdrop.nonVotersMultiplier).Mul(params.malus).Mul(params.supplyFactor)

			// Liquid amount gets the same multiplier as those who didn't vote.
			liquidMultiplier = airdrop.nonVotersMultiplier.Mul(params.malus)

			// total airdrop for this account
			liquidAirdropAmt = acc.LiquidAmount.Mul(liquidMultiplier).Mul(params.supplyFactor)
			stakedAirdropAmt = yesAirdropAmt.Add(noAirdropAmt).Add(noWithVetoAirdropAmt).
						Add(abstainAirdropAmt).Add(noVoteAirdropAmt)
			airdropAmt = liquidAirdropAmt.Add(stakedAirdropAmt)
		)
		// increment airdrop votes
		airdrop.atone.votes.add(govtypes.OptionYes, yesAirdropAmt)
		airdrop.atone.votes.add(govtypes.OptionNo, noAirdropAmt)
		airdrop.atone.votes.add(govtypes.OptionNoWithVeto, noWithVetoAirdropAmt)
		airdrop.atone.votes.add(govtypes.OptionAbstain, abstainAirdropAmt)
		airdrop.atone.votes.add(govtypes.OptionEmpty, noVoteAirdropAmt)
		// increment airdrop supply
		airdrop.atone.supply = airdrop.atone.supply.Add(airdropAmt)
		airdrop.atone.unstaked = airdrop.atone.unstaked.Add(liquidAirdropAmt)
		// add address and amount (skipping 0 balance)
		if amtInt := airdropAmt.RoundInt(); !amtInt.IsZero() {
			addr := acc.Address
			if prefix != "" {
				// Derive address from "cosmos" to prefix parameter
				var err error
				addr, err = convertBech32(acc.Address, "cosmos", prefix)
				if err != nil {
					return airdrop, err
				}
			}
			// Fill with "cosmos" prefixed address
			airdrop.addresses[addr] = amtInt
			ad := addrAmtDetail{
				Address: addr,
				YesDetail: amtDetail{
					AtomAmt:    yesAtomAmt,
					Multiplier: params.yesVotesMultiplier,
					BonusMalus: math.LegacyOneDec(),
					Factor:     params.supplyFactor,
					AtoneAmt:   yesAirdropAmt,
				},
				NoDetail: amtDetail{
					AtomAmt:    noAtomAmt,
					Multiplier: params.noVotesMultiplier,
					BonusMalus: math.LegacyOneDec(),
					Factor:     params.supplyFactor,
					AtoneAmt:   noAirdropAmt,
				},
				NWVDetail: amtDetail{
					AtomAmt:    noWithVetoAtomAmt,
					Multiplier: params.noVotesMultiplier,
					BonusMalus: params.bonus,
					Factor:     params.supplyFactor,
					AtoneAmt:   noWithVetoAirdropAmt,
				},
				AbsDetail: amtDetail{
					AtomAmt:    abstainAtomAmt,
					Multiplier: airdrop.nonVotersMultiplier,
					BonusMalus: math.LegacyOneDec(),
					Factor:     params.supplyFactor,
					AtoneAmt:   abstainAirdropAmt,
				},
				DnvDetail: amtDetail{
					AtomAmt:    noVoteAtomAmt,
					Multiplier: airdrop.nonVotersMultiplier,
					BonusMalus: params.malus,
					Factor:     params.supplyFactor,
					AtoneAmt:   noVoteAirdropAmt,
				},
				LiquidDetail: amtDetail{
					AtomAmt:    acc.LiquidAmount,
					Multiplier: airdrop.nonVotersMultiplier,
					BonusMalus: params.malus,
					Factor:     params.supplyFactor,
					AtoneAmt:   liquidAirdropAmt,
				},
				Total: airdropAmt,
			}
			airdrop.addressesDetail = append(airdrop.addressesDetail, ad)
			amt := yesAirdropAmt.Add(noAirdropAmt).Add(noWithVetoAirdropAmt).Add(abstainAirdropAmt).Add(noVoteAirdropAmt).Add(liquidAirdropAmt)
			if !amt.Equal(airdropAmt) {
				panic(fmt.Sprintf("WRONG %+v\n", ad))
			}
		}
	}
	// Compute minted part
	minted := airdrop.atone.supply.Mul(params.supplyMintFactor)
	airdrop.communityPool = minted.Quo(math.LegacyNewDec(2))
	airdrop.reservedAddr = minted.Quo(math.LegacyNewDec(2))
	return airdrop, nil
}

// convenient type for manipulating vote counts.
type voteMap map[govtypes.VoteOption]math.LegacyDec

var (
	allVoteOptions = []govtypes.VoteOption{
		govtypes.OptionEmpty,
		govtypes.OptionYes,
		govtypes.OptionAbstain,
		govtypes.OptionNo,
		govtypes.OptionNoWithVeto,
	}
	activeVoteOptions = []govtypes.VoteOption{
		govtypes.OptionYes,
		govtypes.OptionNo,
		govtypes.OptionNoWithVeto,
	}
)

func newVoteMap() voteMap {
	m := make(map[govtypes.VoteOption]math.LegacyDec)
	for _, v := range allVoteOptions {
		m[v] = math.LegacyZeroDec()
	}
	return m
}

func (m voteMap) add(v govtypes.VoteOption, d math.LegacyDec) {
	m[v] = m[v].Add(d)
}

func printAirdropsStats(chartMode bool, airdrops []airdrop) error {
	if chartMode {
		f, err := os.CreateTemp("", "chart*.html")
		if err != nil {
			return err
		}
		defer f.Close()
		page := components.NewPage()
		page.PageTitle = "$ATONE distributions"
		page.AddCharts(
			newBarChart(airdrops),
			newPieChart("$ATOM distribution", airdrops[0].atom),
		)
		for _, airdrop := range airdrops {
			page.AddCharts(
				newPieChart(fmt.Sprintf("$ATONE distribution %s", airdrop.params), airdrop.atone),
			)
		}
		page.Render(f)
		fmt.Printf("Charts rendered in %s\n", f.Name())
		browser.OpenFile(f.Name())
		return nil
	}

	printDistrib := func(d distrib) {
		table := newMarkdownTable("", "TOTAL", "DID NOT VOTE", "YES", "NO", "NOWITHVETO", "ABSTAIN", "NOT STAKED")
		table.Append([]string{
			"Distributed",
			humand(d.supply),
			humand(d.votes[govtypes.OptionEmpty]),
			humand(d.votes[govtypes.OptionYes]),
			humand(d.votes[govtypes.OptionNo]),
			humand(d.votes[govtypes.OptionNoWithVeto]),
			humand(d.votes[govtypes.OptionAbstain]),
			humand(d.unstaked),
		})
		votePercs := d.votePercentages()
		table.Append([]string{
			"Percentage over total",
			"",
			humanPercentI(votePercs[govtypes.OptionEmpty]),
			humanPercentI(votePercs[govtypes.OptionYes]),
			humanPercentI(votePercs[govtypes.OptionNo]),
			humanPercentI(votePercs[govtypes.OptionNoWithVeto]),
			humanPercentI(votePercs[govtypes.OptionAbstain]),
			humanPercentI(d.unstaked.Quo(d.supply)),
		})
		table.Render()
		fmt.Println()
	}
	fmt.Println("$ATOM distribution")
	printDistrib(airdrops[0].atom)
	for _, airdrop := range airdrops {
		fmt.Printf("$ATONE distribution (params: %s) (ratio: x%.3f, nonVotersMultiplier: %.3f, icfSlash: %s $ATOM)\n",
			airdrop.params,
			airdrop.atone.supply.Quo(airdrop.atom.supply).MustFloat64(),
			airdrop.nonVotersMultiplier.MustFloat64(),
			humand(airdrop.icfSlash),
		)
		printDistrib(airdrop.atone)
		fmt.Printf(
			"ATONE TOTAL SUPPLY = DISTRIBUTED(%s) + COMMUNITY_POOL(%s) + RESERVED_ADDRESS(%s) = %s\n",
			humand(airdrop.atone.supply), humand(airdrop.communityPool), humand(airdrop.reservedAddr),
			humand(airdrop.atone.supply.Add(airdrop.communityPool).Add(airdrop.reservedAddr)),
		)
	}
	return nil
}

func newBarChart(airdrops []airdrop) *charts.Bar {
	bar := charts.NewBar()
	bar.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{Title: "Votes distribution"}),
		charts.WithLegendOpts(opts.Legend{Show: true, Right: "right", Orient: "vertical"}),
		charts.WithTooltipOpts(opts.Tooltip{
			Show:      true,
			Formatter: opts.FuncOpts("function(params){ return params.value.toFixed(2)+'%'}"),
		}),
	)

	bar.SetXAxis([]string{"Yes", "No", "NWV", "Abstain", "DNV", "Unstaked"})
	generateData := func(d distrib) []opts.BarData {
		var (
			votePercs  = d.votePercentages()
			data       = make([]opts.BarData, 6)
			oneHundred = math.LegacyNewDec(100)
		)
		data[0] = opts.BarData{
			Name:  "Yes",
			Value: votePercs[govtypes.OptionYes].Mul(oneHundred).MustFloat64(),
		}
		data[1] = opts.BarData{
			Name:  "No",
			Value: votePercs[govtypes.OptionNo].Mul(oneHundred).MustFloat64(),
		}
		data[2] = opts.BarData{
			Name:  "NWV",
			Value: votePercs[govtypes.OptionNoWithVeto].Mul(oneHundred).MustFloat64(),
		}
		data[3] = opts.BarData{
			Name:  "Abstain",
			Value: votePercs[govtypes.OptionAbstain].Mul(oneHundred).MustFloat64(),
		}
		data[4] = opts.BarData{
			Name:  "DNV",
			Value: votePercs[govtypes.OptionEmpty].Mul(oneHundred).MustFloat64(),
		}
		data[5] = opts.BarData{
			Name:  "Unstaked",
			Value: d.unstaked.Quo(d.supply).Mul(oneHundred).MustFloat64(),
		}
		return data
	}
	bar.AddSeries("$ATOM", generateData(airdrops[0].atom))
	for _, airdrop := range airdrops {
		bar.AddSeries(fmt.Sprintf("$ATONE %s", airdrop.params), generateData(airdrop.atone))
	}
	return bar
}

func newPieChart(title string, d distrib) *charts.Pie {
	pie := charts.NewPie()
	pie.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{
			Title: title,
		}),
		charts.WithLegendOpts(opts.Legend{
			Show: false,
		}),
		charts.WithTooltipOpts(opts.Tooltip{
			Show:      true,
			Formatter: opts.FuncOpts("function(params){ return params.name+': '+params.value.toFixed(2)+'%'}"),
		}),
	)
	var (
		data       = make([]opts.PieData, 6)
		dataSum    = make([]opts.PieData, 3)
		votePercs  = d.votePercentages()
		oneHundred = math.LegacyNewDec(100)
	)
	data[0] = opts.PieData{
		Name:      "Yes",
		ItemStyle: &opts.ItemStyle{Color: "#ff8b87"},
		Value:     votePercs[govtypes.OptionYes].Mul(oneHundred).MustFloat64(),
	}
	dataSum[0] = opts.PieData{
		Name:      "Yes",
		ItemStyle: &opts.ItemStyle{Color: "#ff8b87"},
		Value:     data[0].Value,
	}
	data[1] = opts.PieData{
		Name:      "No",
		ItemStyle: &opts.ItemStyle{Color: "#9FDFBF"},
		Value:     votePercs[govtypes.OptionNo].Mul(oneHundred).MustFloat64(),
	}
	data[2] = opts.PieData{
		Name:      "NWV",
		ItemStyle: &opts.ItemStyle{Color: "#88d8b0"},
		Value:     votePercs[govtypes.OptionNoWithVeto].Mul(oneHundred).MustFloat64(),
	}
	dataSum[1] = opts.PieData{
		Name:      "No+NWV",
		ItemStyle: &opts.ItemStyle{Color: "#6cac8c"},
		Value:     data[1].Value.(float64) + data[2].Value.(float64),
	}
	data[3] = opts.PieData{
		Name:      "Abstain",
		ItemStyle: &opts.ItemStyle{Color: "#eac086"},
		Value:     votePercs[govtypes.OptionAbstain].Mul(oneHundred).MustFloat64(),
	}
	data[4] = opts.PieData{
		Name:      "DNV",
		ItemStyle: &opts.ItemStyle{Color: "#ffcd94"},
		Value:     votePercs[govtypes.OptionEmpty].Mul(oneHundred).MustFloat64(),
	}
	data[5] = opts.PieData{
		Name:      "Unstaked",
		ItemStyle: &opts.ItemStyle{Color: "#ffe0bd"},
		Value:     d.unstaked.Quo(d.supply).Mul(oneHundred).MustFloat64(),
	}
	dataSum[2] = opts.PieData{
		Name:      "Non voters",
		ItemStyle: &opts.ItemStyle{Color: "#ffad60"},
		Value:     data[3].Value.(float64) + data[4].Value.(float64) + data[5].Value.(float64),
	}
	pie.AddSeries("pie", data,
		charts.WithLabelOpts(opts.Label{
			Show:      true,
			Formatter: opts.FuncOpts("function(params){ return params.name+': '+params.value.toFixed(2)+'%'}"),
		}),
		charts.WithPieChartOpts(opts.PieChart{
			Radius: []string{"45%", "80%"},
		}),
	)
	pie.AddSeries("pie2", dataSum,
		charts.WithLabelOpts(opts.Label{
			Show:      false,
			Formatter: opts.FuncOpts("function(params){ return params.name+': '+params.value.toFixed(2)+'%'}"),
		}),
		charts.WithPieChartOpts(opts.PieChart{
			Radius: []string{"0%", "46%"},
		}),
	)
	return pie
}
