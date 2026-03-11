package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/components"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/pkg/browser"

	"cosmossdk.io/log"
	"cosmossdk.io/math"

	"github.com/cosmos/gogoproto/proto"

	rpcclient "github.com/cometbft/cometbft/rpc/client"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"

	dynamicfeetypes "github.com/atomone-hub/atomone/x/dynamicfee/types"
)

// BlockGasData holds gas data for a single block
type BlockGasData struct {
	Height       int64     `json:"height"`
	TotalGas     int64     `json:"total_gas"`
	GasPrice     float64   `json:"gas_price"`
	LearningRate float64   `json:"learning_rate,omitempty"`
	TxCount      int       `json:"tx_count"`
	Timestamp    time.Time `json:"timestamp"`
}

// GasData holds cached block data
type GasData struct {
	Params *dynamicfeetypes.Params `json:"params,omitempty"`
	Blocks []*BlockGasData         `json:"blocks"`
}

const gasMonitorCacheFile = "data/gasmonitor_cache.json"

// ============================================================
// gasmonitor command — fetch gas data from RPC and display chart
// ============================================================

func gasMonitorCmd() *ffcli.Command {
	fs := flag.NewFlagSet("gasmonitor", flag.ContinueOnError)
	rpcEndpoint := fs.String("rpc", "https://rpc.atomone-archive.citizenweb3.com", "RPC endpoint URL")
	startBlock := fs.Int64("start", 0, "Start block height (0 = latest - numBlocks)")
	numBlocks := fs.Int("num", 100, "Number of blocks to fetch")
	untilStable := fs.Bool("until-stable", false, "Keep fetching until gas stabilizes below 1,000,000")
	noCache := fs.Bool("no-cache", false, "Disable cache")
	outputFile := fs.String("output-file", "", "Write chart to this file instead of a temp file")

	return &ffcli.Command{
		Name:       "gasmonitor",
		ShortUsage: "govbox gasmonitor [flags]",
		ShortHelp:  "Fetch gas data from RPC and display a chart",
		FlagSet:    fs,
		Exec: func(ctx context.Context, args []string) error {
			if err := fs.Parse(args); err != nil {
				return err
			}
			return runGasMonitor(ctx, *rpcEndpoint, *startBlock, *numBlocks, *untilStable, *noCache, *outputFile)
		},
	}
}

func runGasMonitor(ctx context.Context, rpcEndpoint string, startBlock int64, numBlocks int, untilStable, noCache bool, outputFile string) error {
	client, err := rpchttp.New(rpcEndpoint, "/websocket")
	if err != nil {
		return fmt.Errorf("failed to create RPC client: %w", err)
	}

	if startBlock == 0 {
		status, err := client.Status(ctx)
		if err != nil {
			return fmt.Errorf("failed to get node status: %w", err)
		}
		startBlock = status.SyncInfo.LatestBlockHeight - int64(numBlocks)
		if startBlock < 1 {
			startBlock = 1
		}
	}

	endBlock := startBlock + int64(numBlocks) - 1

	if untilStable {
		fmt.Printf("Fetching blocks from %d until gas stabilizes (below 1,000,000) from %s\n", startBlock, rpcEndpoint)
	} else {
		fmt.Printf("Fetching blocks %d to %d from %s\n", startBlock, endBlock, rpcEndpoint)
	}

	cache := loadCache(gasMonitorCacheFile, noCache)

	// Build index for fast lookup by height
	cacheIndex := make(map[int64]*BlockGasData, len(cache.Blocks))
	for _, b := range cache.Blocks {
		cacheIndex[b.Height] = b
	}

	blocksData := make([]*BlockGasData, 0, numBlocks)
	fetchCount := 0

	const stableGasThreshold int64 = 1_000_000
	const consecutiveStableBlocks = 10
	stableCount := 0

	for h := startBlock; ; h++ {
		if !untilStable && h > endBlock {
			break
		}

		if data, ok := cacheIndex[h]; ok {
			blocksData = append(blocksData, data)
			if untilStable {
				if data.TotalGas < stableGasThreshold {
					stableCount++
					if stableCount >= consecutiveStableBlocks {
						fmt.Printf("Gas stabilized below %d for %d consecutive blocks\n", stableGasThreshold, consecutiveStableBlocks)
						break
					}
				} else {
					stableCount = 0
				}
			}
			continue
		}

		data, err := fetchBlockGasData(ctx, client, h)
		if err != nil {
			return fmt.Errorf("Warning: failed to fetch block %d: %v\n", h, err)
		}

		blocksData = append(blocksData, data)
		cache.Blocks = append(cache.Blocks, data)
		fetchCount++

		if fetchCount%10 == 0 {
			fmt.Printf("Fetched %d blocks...\n", fetchCount)
		}

		if untilStable {
			if data.TotalGas < stableGasThreshold {
				stableCount++
				if stableCount >= consecutiveStableBlocks {
					fmt.Printf("Gas stabilized below %d for %d consecutive blocks\n", stableGasThreshold, consecutiveStableBlocks)
					break
				}
			} else {
				stableCount = 0
			}
		}
	}

	fmt.Printf("Fetched %d blocks from RPC, %d from cache\n", fetchCount, len(blocksData)-fetchCount)

	// Fetch params once from the first monitored block if not cached
	if cache.Params == nil && len(blocksData) > 0 {
		params, err := fetchDynamicfeeParams(ctx, client, blocksData[0].Height)
		if err != nil {
			fmt.Printf("Warning: failed to fetch dynamicfee params: %v\n", err)
		} else {
			cache.Params = params
			fmt.Printf("Fetched dynamicfee params at block %d\n", blocksData[0].Height)
		}
	}

	if fetchCount > 0 || cache.Params != nil {
		if err := saveCache(gasMonitorCacheFile, cache); err != nil {
			fmt.Printf("Warning: failed to save cache: %v\n", err)
		}
	}

	result := &GasData{
		Params: cache.Params,
		Blocks: blocksData,
	}
	return generateGasChart(result, outputFile)
}

func fetchBlockGasData(ctx context.Context, client *rpchttp.HTTP, height int64) (*BlockGasData, error) {
	blockResults, err := client.BlockResults(ctx, &height)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch block results: %w", err)
	}

	block, err := client.Block(ctx, &height)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch block header: %w", err)
	}

	var totalGas int64
	for _, txResult := range blockResults.TxsResults {
		totalGas += txResult.GasUsed
	}

	gasPrice, learningRate, err := fetchDynamicfeeState(ctx, client, height)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch dynamicfee state: %w", err)
	}

	return &BlockGasData{
		Height:       height,
		TotalGas:     totalGas,
		GasPrice:     gasPrice,
		LearningRate: learningRate,
		TxCount:      len(blockResults.TxsResults),
		Timestamp:    block.Block.Header.Time,
	}, nil
}

func fetchDynamicfeeState(ctx context.Context, client *rpchttp.HTTP, height int64) (gasPrice float64, learningRate float64, err error) {
	resp, err := client.ABCIQueryWithOptions(ctx,
		"/atomone.dynamicfee.v1.Query/State",
		nil,
		rpcclient.ABCIQueryOptions{Height: height},
	)
	if err != nil {
		return 0, 0, fmt.Errorf("ABCI query failed: %w", err)
	}
	if resp.Response.Code != 0 {
		return 0, 0, fmt.Errorf("ABCI query error: %s", resp.Response.Log)
	}

	var stateResp dynamicfeetypes.StateResponse
	if err := proto.Unmarshal(resp.Response.Value, &stateResp); err != nil {
		return 0, 0, fmt.Errorf("failed to unmarshal StateResponse: %w", err)
	}

	gp, _ := stateResp.State.BaseGasPrice.Float64()
	lr, _ := stateResp.State.LearningRate.Float64()
	return gp, lr, nil
}

func fetchDynamicfeeParams(ctx context.Context, client *rpchttp.HTTP, height int64) (*dynamicfeetypes.Params, error) {
	resp, err := client.ABCIQueryWithOptions(ctx,
		"/atomone.dynamicfee.v1.Query/Params",
		nil,
		rpcclient.ABCIQueryOptions{Height: height},
	)
	if err != nil {
		return nil, fmt.Errorf("ABCI query failed: %w", err)
	}
	if resp.Response.Code != 0 {
		return nil, fmt.Errorf("ABCI query error: %s", resp.Response.Log)
	}

	var paramsResp dynamicfeetypes.ParamsResponse
	if err := proto.Unmarshal(resp.Response.Value, &paramsResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ParamsResponse: %w", err)
	}

	return &paramsResp.Params, nil
}

// ============================================================
// gassim command — simulate AIMD EIP-1559 and generate chart
// ============================================================

func gasSimCmd() *ffcli.Command {
	fs := flag.NewFlagSet("gassim", flag.ContinueOnError)
	inputFile := fs.String("input-file", "", "Input JSON file (GasData format with total_gas data)")
	outputFile := fs.String("output-file", "", "Output HTML chart file")

	// AIMD parameters — defaults come from the input file's params section.
	// Only flags explicitly set on the command line override the input file values.
	alpha := fs.Float64("alpha", 0, "Alpha: additive LR increase")
	beta := fs.Float64("beta", 0, "Beta: multiplicative LR decrease")
	gamma := fs.Float64("gamma", 0, "Gamma: equilibrium band threshold")
	minLR := fs.Float64("min-lr", 0, "Minimum learning rate")
	maxLR := fs.Float64("max-lr", 0, "Maximum learning rate")
	window := fs.Uint64("window", 0, "Sliding window size (blocks)")
	targetUtil := fs.Float64("target-util", 0, "Target block utilization")
	maxBlockGas := fs.Uint64("max-block-gas", 0, "Maximum block gas")
	minGasPrice := fs.Float64("min-gas-price", 0, "Minimum base gas price")

	return &ffcli.Command{
		Name:       "gassim",
		ShortUsage: "govbox gassim -input-file <path> [-output-file <path>] [flags]",
		ShortHelp:  "Simulate dynamic fee evolution and generate a chart",
		FlagSet:    fs,
		Exec: func(ctx context.Context, args []string) error {
			if err := fs.Parse(args); err != nil {
				return err
			}
			if *inputFile == "" {
				return fmt.Errorf("-input-file is required")
			}

			// Load input file to get cached params as defaults
			data, err := os.ReadFile(*inputFile)
			if err != nil {
				return fmt.Errorf("failed to read input file: %w", err)
			}
			var cache GasData
			if err := json.Unmarshal(data, &cache); err != nil {
				return fmt.Errorf("failed to parse input file: %w", err)
			}

			if cache.Params == nil {
				return fmt.Errorf("input file has no params section; run gasmonitor first to populate it")
			}

			// Start from cached params, then override with explicitly-set flags
			params := *cache.Params
			setFlags := make(map[string]bool)
			fs.Visit(func(f *flag.Flag) { setFlags[f.Name] = true })

			if setFlags["alpha"] {
				params.Alpha = math.LegacyMustNewDecFromStr(fmt.Sprintf("%g", *alpha))
			}
			if setFlags["beta"] {
				params.Beta = math.LegacyMustNewDecFromStr(fmt.Sprintf("%g", *beta))
			}
			if setFlags["gamma"] {
				params.Gamma = math.LegacyMustNewDecFromStr(fmt.Sprintf("%g", *gamma))
			}
			if setFlags["min-lr"] {
				params.MinLearningRate = math.LegacyMustNewDecFromStr(fmt.Sprintf("%g", *minLR))
			}
			if setFlags["max-lr"] {
				params.MaxLearningRate = math.LegacyMustNewDecFromStr(fmt.Sprintf("%g", *maxLR))
			}
			if setFlags["window"] {
				params.Window = *window
			}
			if setFlags["target-util"] {
				params.TargetBlockUtilization = math.LegacyMustNewDecFromStr(fmt.Sprintf("%g", *targetUtil))
			}
			if setFlags["max-block-gas"] {
				params.DefaultMaxBlockGas = *maxBlockGas
			}
			if setFlags["min-gas-price"] {
				params.MinBaseGasPrice = math.LegacyMustNewDecFromStr(fmt.Sprintf("%g", *minGasPrice))
			}

			return runGasSim(&cache, *outputFile, params)
		},
	}
}

func runGasSim(cache *GasData, outputFile string, params dynamicfeetypes.Params) error {
	state := dynamicfeetypes.NewState(
		params.Window,
		params.MinBaseGasPrice,
		params.MaxLearningRate,
	)

	logger := log.NewNopLogger()
	maxBlockGas := params.DefaultMaxBlockGas

	simBlocks := make([]*BlockGasData, 0, len(cache.Blocks))
	for _, b := range cache.Blocks {
		totalGas := b.TotalGas
		if totalGas < 0 {
			totalGas = 0
		}

		state.Window[state.Index] = uint64(totalGas)

		state.UpdateLearningRate(params, maxBlockGas)
		state.UpdateBaseGasPrice(logger, params, maxBlockGas)

		gp, _ := state.BaseGasPrice.Float64()
		lr, _ := state.LearningRate.Float64()

		simBlocks = append(simBlocks, &BlockGasData{
			Height:       b.Height,
			TotalGas:     b.TotalGas,
			GasPrice:     gp,
			LearningRate: lr,
			TxCount:      b.TxCount,
			Timestamp:    b.Timestamp,
		})

		state.IncrementHeight()
	}

	result := &GasData{
		Params: &params,
		Blocks: simBlocks,
	}
	return generateGasChart(result, outputFile)
}

// ============================================================
// Shared: cache, chart generation
// ============================================================

func loadCache(cacheFile string, noCache bool) *GasData {
	if noCache {
		return &GasData{}
	}

	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return &GasData{}
	}

	var cache GasData
	if err := json.Unmarshal(data, &cache); err != nil {
		return &GasData{}
	}

	return &cache
}

func saveCache(cacheFile string, cache *GasData) error {
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cacheFile, data, 0o644)
}

func generateGasChart(data *GasData, outputFile string) error {
	blocksData := data.Blocks
	if len(blocksData) == 0 {
		return fmt.Errorf("no block data to display")
	}

	p := data.Params
	alpha, _ := p.Alpha.Float64()
	beta, _ := p.Beta.Float64()
	gamma, _ := p.Gamma.Float64()
	minLR, _ := p.MinLearningRate.Float64()
	maxLR, _ := p.MaxLearningRate.Float64()
	targetUtil, _ := p.TargetBlockUtilization.Float64()
	minGasPrice, _ := p.MinBaseGasPrice.Float64()

	bar := charts.NewBar()
	bar.SetGlobalOptions(
		charts.WithLegendOpts(opts.Legend{
			Show:   true,
			Top:    "10px",
			Left:   "center",
			Orient: "horizontal",
		}),
		charts.WithGridOpts(opts.Grid{
			Top: "50px",
		}),
		charts.WithTooltipOpts(opts.Tooltip{
			Show:    true,
			Trigger: "axis",
			AxisPointer: &opts.AxisPointer{
				Type: "cross",
			},
			Formatter: opts.FuncOpts(`function(params) {
				var idx = params[0].dataIndex;
				var result = '<b>Block ' + params[0].axisValue + '</b><br/>';
				result += timestamps[idx] + '<br/>';
				result += 'Transactions: ' + txCounts[idx] + '<br/>';
				for (var i = 0; i < params.length; i++) {
					var p = params[i];
					if (p.seriesName === 'Gas Threshold') continue;
					result += p.marker + ' ' + p.seriesName + ': ' + p.value + '<br/>';
				}
				return result;
			}`),
		}),
		charts.WithXAxisOpts(opts.XAxis{
			Name: "Block Height",
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Name: "Gas Consumed",
			Type: "value",
		}),
		charts.WithInitializationOpts(opts.Initialization{
			Width:  "100%",
			Height: "600px",
		}),
	)

	gasThreshold := int64(float64(p.DefaultMaxBlockGas) * targetUtil)

	xAxis := make([]string, len(blocksData))
	blockHeights := make([]int64, len(blocksData))
	txCounts := make([]int, len(blocksData))
	timestamps := make([]string, len(blocksData))
	gasBarData := make([]opts.BarData, len(blocksData))
	gasPriceLineData := make([]opts.LineData, len(blocksData))
	lrLineData := make([]opts.LineData, len(blocksData))

	var maxGas int64
	var maxGasPrice float64
	hasLR := false
	for i, block := range blocksData {
		xAxis[i] = humanize.Comma(block.Height)
		blockHeights[i] = block.Height
		txCounts[i] = block.TxCount
		timestamps[i] = block.Timestamp.Format("2006-01-02 15:04:05 UTC")
		gasBarData[i] = opts.BarData{Value: block.TotalGas}
		gasPriceLineData[i] = opts.LineData{Value: block.GasPrice}
		lrLineData[i] = opts.LineData{Value: block.LearningRate}
		if block.TotalGas > maxGas {
			maxGas = block.TotalGas
		}
		if block.GasPrice > maxGasPrice {
			maxGasPrice = block.GasPrice
		}
		if block.LearningRate > 0 {
			hasLR = true
		}
	}

	gasPriceAxisMax := 0.2
	if maxGasPrice > gasPriceAxisMax {
		gasPriceAxisMax = maxGasPrice * 1.1
	}

	blockHeightsJSON, _ := json.Marshal(blockHeights)
	txCountsJSON, _ := json.Marshal(txCounts)
	timestampsJSON, _ := json.Marshal(timestamps)
	paramsTableHTML := fmt.Sprintf(`<table style="margin:10px auto;border-collapse:collapse;font-family:monospace;font-size:13px">
<tr>
<th style="padding:4px 12px;border:1px solid #ddd;background:#f5f5f5">Alpha (α)</th>
<th style="padding:4px 12px;border:1px solid #ddd;background:#f5f5f5">Beta (β)</th>
<th style="padding:4px 12px;border:1px solid #ddd;background:#f5f5f5">Gamma (γ)</th>
<th style="padding:4px 12px;border:1px solid #ddd;background:#f5f5f5">MinLR</th>
<th style="padding:4px 12px;border:1px solid #ddd;background:#f5f5f5">MaxLR</th>
<th style="padding:4px 12px;border:1px solid #ddd;background:#f5f5f5">Window</th>
<th style="padding:4px 12px;border:1px solid #ddd;background:#f5f5f5">MaxBlockGas</th>
<th style="padding:4px 12px;border:1px solid #ddd;background:#f5f5f5">Target</th>
<th style="padding:4px 12px;border:1px solid #ddd;background:#f5f5f5">MinPrice</th>
<th style="padding:4px 12px;border:1px solid #ddd;background:#f5f5f5">Denom</th>
<th style="padding:4px 12px;border:1px solid #ddd;background:#f5f5f5">Blocks</th>
</tr><tr>
<td style="padding:4px 12px;border:1px solid #ddd;text-align:center">%.4f</td>
<td style="padding:4px 12px;border:1px solid #ddd;text-align:center">%.2f</td>
<td style="padding:4px 12px;border:1px solid #ddd;text-align:center">%.2f</td>
<td style="padding:4px 12px;border:1px solid #ddd;text-align:center">%.4f</td>
<td style="padding:4px 12px;border:1px solid #ddd;text-align:center">%.2f</td>
<td style="padding:4px 12px;border:1px solid #ddd;text-align:center">%d</td>
<td style="padding:4px 12px;border:1px solid #ddd;text-align:center">%s</td>
<td style="padding:4px 12px;border:1px solid #ddd;text-align:center">%.0f%%</td>
<td style="padding:4px 12px;border:1px solid #ddd;text-align:center">%.4f</td>
<td style="padding:4px 12px;border:1px solid #ddd;text-align:center">%s</td>
<td style="padding:4px 12px;border:1px solid #ddd;text-align:center">%s – %s</td>
</tr></table>`,
		alpha, beta, gamma, minLR, maxLR, p.Window,
		humanize.Comma(int64(p.DefaultMaxBlockGas)), targetUtil*100,
		minGasPrice, p.FeeDenom,
		humanize.Comma(blocksData[0].Height), humanize.Comma(blocksData[len(blocksData)-1].Height))

	clickHandler := fmt.Sprintf(`
		var blockHeights = %s;
		var txCounts = %s;
		var timestamps = %s;
		goecharts_%s.on('click', function(params) {
			if (params.seriesName === 'Gas Consumed') {
				var height = blockHeights[params.dataIndex];
				window.open('https://www.mintscan.io/atomone/block/' + height, '_blank');
			}
		});
	`, string(blockHeightsJSON), string(txCountsJSON), string(timestampsJSON), bar.ChartID)

	bar.SetXAxis(xAxis).
		AddSeries("Gas Consumed", gasBarData).
		ExtendYAxis(opts.YAxis{
			Name: "Gas Price (photon)",
			Type: "value",
			Min:  0,
			Max:  gasPriceAxisMax,
		})

	line := charts.NewLine()
	line.SetXAxis(xAxis).
		AddSeries("Gas Price", gasPriceLineData,
			charts.WithLineChartOpts(opts.LineChart{
				YAxisIndex: 1,
				Smooth:     true,
			}),
			charts.WithItemStyleOpts(opts.ItemStyle{
				Color: "#00e676",
			}),
		)

	if maxGas > gasThreshold {
		thresholdLineData := make([]opts.LineData, len(blocksData))
		for i := range blocksData {
			thresholdLineData[i] = opts.LineData{Value: gasThreshold}
		}
		line.AddSeries("Gas Threshold", thresholdLineData,
			charts.WithLineChartOpts(opts.LineChart{
				YAxisIndex: 0,
			}),
			charts.WithItemStyleOpts(opts.ItemStyle{
				Color: "#ee6666",
			}),
			charts.WithLineStyleOpts(opts.LineStyle{
				Type:  "dashed",
				Width: 2,
			}),
		)
	}

	if hasLR {
		bar.ExtendYAxis(opts.YAxis{
			Type:      "value",
			Min:       0,
			Max:       0.55,
			AxisLabel: &opts.AxisLabel{Show: false},
			AxisLine:  &opts.AxisLine{Show: false},
			SplitLine: &opts.SplitLine{Show: false},
		})
		line.AddSeries("Learning Rate", lrLineData,
			charts.WithLineChartOpts(opts.LineChart{
				YAxisIndex: 2,
				Smooth:     true,
			}),
			charts.WithItemStyleOpts(opts.ItemStyle{
				Color: "#ff9800",
			}),
			charts.WithLineStyleOpts(opts.LineStyle{
				Width: 2,
			}),
		)
	}

	page := components.NewPage()
	page.PageTitle = "AtomOne Gas Monitor"

	bar.Overlap(line)
	bar.AddJSFuncs(clickHandler)
	page.AddCharts(bar)

	var (
		f   *os.File
		err error
	)
	if outputFile != "" {
		f, err = os.Create(outputFile)
	} else {
		f, err = os.CreateTemp("", "gasmonitor*.html")
	}
	if err != nil {
		return err
	}
	defer f.Close()

	absPath, err := filepath.Abs(f.Name())
	if err != nil {
		absPath = f.Name()
	}

	var buf bytes.Buffer
	if err := page.Render(&buf); err != nil {
		return err
	}
	html := strings.Replace(buf.String(), "<body>", "<body>\n"+paramsTableHTML, 1)
	if _, err := f.WriteString(html); err != nil {
		return err
	}

	fmt.Printf("Chart rendered to %s\n", absPath)
	if outputFile != "" {
		return nil
	}
	return browser.OpenFile(absPath)
}
