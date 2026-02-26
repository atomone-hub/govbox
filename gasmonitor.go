package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/components"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/pkg/browser"

	rpcclient "github.com/cometbft/cometbft/rpc/client"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
)

// BlockGasData holds gas data for a single block
type BlockGasData struct {
	Height    int64     `json:"height"`
	TotalGas  int64     `json:"total_gas"`
	GasPrice  float64   `json:"gas_price"`
	TxCount   int       `json:"tx_count"`
	Timestamp time.Time `json:"timestamp"`
}

// GasCache holds cached block data
type GasCache struct {
	Title  string                    `json:"title,omitempty"`
	Blocks map[int64]*BlockGasData   `json:"blocks"`
}

const gasMonitorCacheFile = "data/gasmonitor_cache.json"

func gasMonitorCmd() *ffcli.Command {
	fs := flag.NewFlagSet("gasmonitor", flag.ContinueOnError)
	rpcEndpoint := fs.String("rpc", "https://atomone-rpc.allinbits.com:443", "RPC endpoint URL")
	startBlock := fs.Int64("start", 0, "Start block height (0 = latest - numBlocks)")
	numBlocks := fs.Int("num", 100, "Number of blocks to fetch")
	untilStable := fs.Bool("until-stable", false, "Keep fetching until gas stabilizes below 1,000,000")
	noCache := fs.Bool("no-cache", false, "Disable cache")
	inputFile := fs.String("input-file", "", "Generate chart from this JSON file (GasCache format), skip RPC")
	outputFile := fs.String("output-file", "", "Write chart to this file instead of a temp file")

	return &ffcli.Command{
		Name:       "gasmonitor",
		ShortUsage: "govbox gasmonitor [flags]",
		ShortHelp:  "Display a chart comparing total gas per block with dynamicfee gas price",
		FlagSet:    fs,
		Exec: func(ctx context.Context, args []string) error {
			if err := fs.Parse(args); err != nil {
				return err
			}
			if *inputFile != "" {
				return generateChartFromFile(*inputFile, *outputFile)
			}
			return runGasMonitor(ctx, *rpcEndpoint, *startBlock, *numBlocks, *untilStable, *noCache, *outputFile)
		},
	}
}

func generateChartFromFile(path, outputFile string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}
	var cache GasCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return fmt.Errorf("failed to parse input file: %w", err)
	}
	blocks := make([]*BlockGasData, 0, len(cache.Blocks))
	for _, b := range cache.Blocks {
		blocks = append(blocks, b)
	}
	sort.Slice(blocks, func(i, j int) bool {
		return blocks[i].Height < blocks[j].Height
	})
	return generateGasChart(blocks, cache.Title, outputFile)
}

func runGasMonitor(ctx context.Context, rpcEndpoint string, startBlock int64, numBlocks int, untilStable, noCache bool, outputFile string) error {
	// Create RPC client
	client, err := rpchttp.New(rpcEndpoint, "/websocket")
	if err != nil {
		return fmt.Errorf("failed to create RPC client: %w", err)
	}

	// Get latest block height if startBlock is 0
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

	// Load cache
	cache := loadCache(gasMonitorCacheFile, noCache)

	// Fetch blocks
	blocksData := make([]*BlockGasData, 0, numBlocks)
	fetchCount := 0

	const stableGasThreshold int64 = 1_000_000
	const consecutiveStableBlocks = 10
	stableCount := 0

	for h := startBlock; ; h++ {
		// Check if we should stop (when not in untilStable mode)
		if !untilStable && h > endBlock {
			break
		}

		// Check cache first
		if data, ok := cache.Blocks[h]; ok {
			blocksData = append(blocksData, data)
			// Check stability condition
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

		// Fetch from RPC
		data, err := fetchBlockGasData(ctx, client, h)
		if err != nil {
			return fmt.Errorf("Warning: failed to fetch block %d: %v\n", h, err)
		}

		blocksData = append(blocksData, data)
		cache.Blocks[h] = data
		fetchCount++

		// Progress indicator
		if fetchCount%10 == 0 {
			fmt.Printf("Fetched %d blocks...\n", fetchCount)
		}

		// Check stability condition
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

	// Save cache
	if fetchCount > 0 {
		if err := saveCache(gasMonitorCacheFile, cache); err != nil {
			fmt.Printf("Warning: failed to save cache: %v\n", err)
		}
	}

	// Sort by height
	sort.Slice(blocksData, func(i, j int) bool {
		return blocksData[i].Height < blocksData[j].Height
	})

	// Generate chart
	return generateGasChart(blocksData, "", outputFile)
}

func fetchBlockGasData(ctx context.Context, client *rpchttp.HTTP, height int64) (*BlockGasData, error) {
	// Fetch block results to get total gas used
	blockResults, err := client.BlockResults(ctx, &height)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch block results: %w", err)
	}

	// Fetch block header to get timestamp
	block, err := client.Block(ctx, &height)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch block header: %w", err)
	}

	var totalGas int64
	for _, txResult := range blockResults.TxsResults {
		totalGas += txResult.GasUsed
	}

	// Fetch gas price from dynamicfee module via ABCI query
	gasPrice, err := fetchDynamicfeeGasPrice(ctx, client, height)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch gas price: %w", err)
	}

	return &BlockGasData{
		Height:    height,
		TotalGas:  totalGas,
		GasPrice:  gasPrice,
		TxCount:   len(blockResults.TxsResults),
		Timestamp: block.Block.Header.Time,
	}, nil
}

func fetchDynamicfeeGasPrice(ctx context.Context, client *rpchttp.HTTP, height int64) (float64, error) {
	// Query the dynamicfee module state via ABCI query at specific height
	resp, err := client.ABCIQueryWithOptions(ctx,
		"/atomone.dynamicfee.v1.Query/State",
		nil, // empty request
		rpcclient.ABCIQueryOptions{Height: height},
	)
	if err != nil {
		return 0, fmt.Errorf("ABCI query failed: %w", err)
	}

	if resp.Response.Code != 0 {
		return 0, fmt.Errorf("ABCI query error: %s", resp.Response.Log)
	}

	// Parse the protobuf response manually
	// State message: base_gas_price is field 1 (string type)
	return parseBaseGasPriceFromProto(resp.Response.Value)
}

// parseBaseGasPriceFromProto extracts base_gas_price from protobuf-encoded StateResponse message
// The response is wrapped: StateResponse { State state = 1; } where State { string base_gas_price = 1; ... }
// The base_gas_price is in SDK Dec format (integer with 18 decimal places of precision)
func parseBaseGasPriceFromProto(data []byte) (float64, error) {
	if len(data) == 0 {
		return 0, fmt.Errorf("empty response")
	}

	// First, unwrap the StateResponse to get the State message (field 1)
	stateData, err := extractProtoField(data, 1)
	if err != nil {
		return 0, fmt.Errorf("failed to extract State from response: %w", err)
	}

	// Now extract base_gas_price (field 1) from the State message
	priceData, err := extractProtoField(stateData, 1)
	if err != nil {
		return 0, fmt.Errorf("failed to extract base_gas_price from State: %w", err)
	}

	// The price is stored as a string integer with 18 decimal places
	// e.g., "10000000000000000" = 0.01 (10^16 / 10^18)
	priceInt, err := strconv.ParseFloat(string(priceData), 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse base_gas_price: %w", err)
	}

	// Convert from SDK Dec format (18 decimal precision) to float
	return priceInt / 1e18, nil
}

// extractProtoField extracts a length-delimited field from protobuf data
func extractProtoField(data []byte, targetField int) ([]byte, error) {
	i := 0
	for i < len(data) {
		if i >= len(data) {
			break
		}

		// Read tag byte
		tag := data[i]
		fieldNum := int(tag >> 3)
		wireType := tag & 0x07
		i++

		if wireType == 2 { // Length-delimited
			if i >= len(data) {
				return nil, fmt.Errorf("truncated data at length")
			}
			length := int(data[i])
			i++
			if i+length > len(data) {
				return nil, fmt.Errorf("truncated field data")
			}

			if fieldNum == targetField {
				return data[i : i+length], nil
			}
			i += length
		} else if wireType == 0 { // Varint
			for i < len(data) && data[i]&0x80 != 0 {
				i++
			}
			i++
		} else {
			return nil, fmt.Errorf("unsupported wire type %d", wireType)
		}
	}

	return nil, fmt.Errorf("field %d not found", targetField)
}

func loadCache(cacheFile string, noCache bool) *GasCache {
	cache := &GasCache{
		Blocks: make(map[int64]*BlockGasData),
	}
	if noCache {
		return cache
	}

	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return cache
	}

	if err := json.Unmarshal(data, cache); err != nil {
		return &GasCache{Blocks: make(map[int64]*BlockGasData)}
	}

	return cache
}

func saveCache(cacheFile string, cache *GasCache) error {
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cacheFile, data, 0o644)
}

func generateGasChart(blocksData []*BlockGasData, title, outputFile string) error {
	if len(blocksData) == 0 {
		return fmt.Errorf("no block data to display")
	}

	if title == "" {
		title = "AtomOne Gas Monitor"
	}

	// Create chart
	bar := charts.NewBar()
	bar.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{
			Title:    title,
			Subtitle: fmt.Sprintf("Blocks %d - %d", blocksData[0].Height, blocksData[len(blocksData)-1].Height),
		}),
		charts.WithLegendOpts(opts.Legend{
			Show:   true,
			Top:    "50px",
			Left:   "center",
			Orient: "horizontal",
		}),
		charts.WithGridOpts(opts.Grid{
			Top: "100px",
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
			Name: "Total Gas",
			Type: "value",
		}),
		charts.WithInitializationOpts(opts.Initialization{
			Width:  "100%",
			Height: "600px",
		}),
	)

	// Prepare data
	const gasThreshold int64 = 50_000_000 // Gas limit before price increase

	xAxis := make([]string, len(blocksData))
	blockHeights := make([]int64, len(blocksData))
	txCounts := make([]int, len(blocksData))
	timestamps := make([]string, len(blocksData))
	gasBarData := make([]opts.BarData, len(blocksData))
	gasPriceLineData := make([]opts.LineData, len(blocksData))

	var maxGas int64
	var maxGasPrice float64
	for i, block := range blocksData {
		xAxis[i] = humanize.Comma(block.Height)
		blockHeights[i] = block.Height
		txCounts[i] = block.TxCount
		timestamps[i] = block.Timestamp.Format("2006-01-02 15:04:05 UTC")
		gasBarData[i] = opts.BarData{Value: block.TotalGas}
		gasPriceLineData[i] = opts.LineData{Value: block.GasPrice}
		if block.TotalGas > maxGas {
			maxGas = block.TotalGas
		}
		if block.GasPrice > maxGasPrice {
			maxGasPrice = block.GasPrice
		}
	}

	// Set gas price Y-axis max with some headroom (minimum 0.2)
	gasPriceAxisMax := 0.2
	if maxGasPrice > gasPriceAxisMax {
		gasPriceAxisMax = maxGasPrice * 1.1 // 10% headroom
	}

	// Add click handler and tooltip data
	blockHeightsJSON, _ := json.Marshal(blockHeights)
	txCountsJSON, _ := json.Marshal(txCounts)
	timestampsJSON, _ := json.Marshal(timestamps)
	clickHandler := fmt.Sprintf(`
		var blockHeights = %s;
		var txCounts = %s;
		var timestamps = %s;
		goecharts_%s.on('click', function(params) {
			if (params.seriesName === 'Total Gas') {
				var height = blockHeights[params.dataIndex];
				window.open('https://www.mintscan.io/atomone/block/' + height, '_blank');
			}
		});
	`, string(blockHeightsJSON), string(txCountsJSON), string(timestampsJSON), bar.ChartID)

	bar.SetXAxis(xAxis).
		AddSeries("Total Gas", gasBarData).
		ExtendYAxis(opts.YAxis{
			Name: "Gas Price (photon)",
			Type: "value",
			Min:  0,
			Max:  gasPriceAxisMax,
		})

	// Create line chart for gas price
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

	// Add threshold line only if any block exceeds it
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

	// Combine into a page
	page := components.NewPage()
	page.PageTitle = "AtomOne Gas Monitor"

	// Create combined chart by overlapping bar and line
	bar.Overlap(line)

	// Add click handler JavaScript
	bar.AddJSFuncs(clickHandler)

	page.AddCharts(bar)

	// Render chart
	var f *os.File
	var err error
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

	if err := page.Render(f); err != nil {
		return err
	}

	fmt.Printf("Chart rendered to %s\n", absPath)
	if outputFile != "" {
		return nil
	}
	return browser.OpenFile(absPath)
}
