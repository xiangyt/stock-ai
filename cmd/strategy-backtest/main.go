// 策略全市场回溯工具 — 按交易日期遍历全市场，统计策略选股及未来走势。
//
// 用法:
//
//	go run ./cmd/strategy-backtest/
//
// 修改下方 startDate、endDate、signalIDs 等变量后重新运行。
// 所有数据从 DB 加载，K 线按 tradeDate 截断（模拟该日收市后视角）。
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"

	"stock-ai/internal/backtest/indicator"
	"stock-ai/internal/backtest/indicator/financial"
	"stock-ai/internal/backtest/indicator/fundamental"
	"stock-ai/internal/backtest/indicator/market"
	"stock-ai/internal/backtest/indicator/stocksource"
	"stock-ai/internal/backtest/indicator/technical"
	"stock-ai/internal/config"
	"stock-ai/internal/db"
	"stock-ai/internal/holiday"
	"stock-ai/internal/model"
	"stock-ai/utils"
)

// ============================================================================
//  调试配置 — 直接修改下面的变量
// ============================================================================

// startDate 起始日期 YYYYMMDD
var startDate = "20260101"

// endDate 结束日期 YYYYMMDD
var endDate = "20260701"

// strategyID 策略ID（从 strategies 表查询）
var strategyID uint = 20

// outputPath Excel 输出路径
var outputPath = fmt.Sprintf("strategy_%d_%s_%s.xlsx", strategyID, startDate, endDate)

// configPath 配置文件路径
var configPath = "config.yaml"

// maxConcurrency 引擎并发度
var maxConcurrency = 200

const (
	futureTradingDays = 20 // 未来交易日数量
	klineQueryLimit   = 25 // K线查询条数（略大于 futureTradingDays 以应对数据缺失）
)

// ============================================================================
// selectionRecord 单日选股记录
// ============================================================================

type selectionRecord struct {
	Code        string  // 股票代码
	Name        string  // 股票名称
	SelectDate  int     // 选出日期 YYYYMMDD
	SelectClose float64 // 选出日收盘价（元）
}

// ============================================================================
// main
// ============================================================================

func main() {
	log.Printf("策略ID: %d", strategyID)
	log.Printf("日期范围: %s ~ %s", startDate, endDate)
	log.Printf("并发度: %d", maxConcurrency)

	// 1. 初始化
	reg := initAll(configPath)
	defer db.Close()

	// 2. 从数据库加载策略，构建信号配置
	configs, err := loadStrategyConfigs(strategyID)
	if err != nil {
		log.Fatalf("加载策略失败: %v", err)
	}
	log.Printf("策略信号: %d 个条件", len(configs))
	for _, c := range configs {
		log.Printf("  → %s (op=%s)", c.SignalID, c.Operator)
	}

	// 3. 获取交易日列表
	tradingDays, err := getTradingDays(startDate, endDate)
	if err != nil {
		log.Fatalf("获取交易日失败: %v", err)
	}
	log.Printf("共 %d 个交易日", len(tradingDays))

	// 4. 逐日执行选股
	allResults := make([]selectionRecord, 0)
	for i, dayStr := range tradingDays {
		tradeDate, _ := parseTradeDate(dayStr)
		log.Printf("[%d/%d] 处理 %s (%d)...", i+1, len(tradingDays), dayStr, tradeDate)

		records := runDay(reg, tradeDate, configs)
		allResults = append(allResults, records...)
		log.Printf("  → 选出 %d 只", len(records))
	}

	log.Printf("共选出 %d 条记录", len(allResults))

	if len(allResults) == 0 {
		log.Println("无选股结果，不生成 Excel")
		return
	}

	// 5. 查询每只选出股票的未来 K 线
	futureData := queryFutureKlines(allResults)

	// 6. 导出 Excel
	if err := exportExcel(allResults, futureData, outputPath); err != nil {
		log.Fatalf("导出Excel失败: %v", err)
	}
	log.Printf("✅ Excel 已导出: %s", outputPath)
}

// ============================================================================
// 初始化
// ============================================================================

func initAll(cfgPath string) *indicator.Registry {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	log.Printf("[INFO] 配置加载成功: %s", cfgPath)

	if err := db.Init(&cfg.Database); err != nil {
		log.Fatalf("数据库连接失败: %v", err)
	}
	log.Println("[INFO] 数据库连接成功")

	// 加载节假日数据
	if err := holiday.GetProvider().Load(context.Background()); err != nil {
		log.Printf("[WARN] 节假日数据加载失败（将仅排除周末）: %v", err)
	}

	allIndicators := append(
		append(technical.All(), market.All()...),
		append(fundamental.All(), financial.All()...)...,
	)
	reg := indicator.NewRegistry(allIndicators)
	log.Printf("[INFO] 已注册 %d 个指标", len(allIndicators))

	return reg
}

// ============================================================================
// 交易日处理
// ============================================================================

// getTradingDays 获取 [start, end] 区间内所有交易日（YYYYMMDD 字符串）
func getTradingDays(start, end string) ([]string, error) {
	provider := holiday.GetProvider()
	startFmt, err := formatDateToDash(start)
	if err != nil {
		return nil, fmt.Errorf("起始日期格式错误: %w", err)
	}
	endFmt, err := formatDateToDash(end)
	if err != nil {
		return nil, fmt.Errorf("结束日期格式错误: %w", err)
	}
	return provider.GetTradingDays(startFmt, endFmt)
}

// formatDateToDash 将 YYYYMMDD 转为 YYYY-MM-DD
func formatDateToDash(date string) (string, error) {
	if len(date) != 8 {
		return "", fmt.Errorf("日期长度应为8: %s", date)
	}
	return date[0:4] + "-" + date[4:6] + "-" + date[6:8], nil
}

// parseTradeDate 将 YYYY-MM-DD 转为 YYYYMMDD int
func parseTradeDate(dayStr string) (int, error) {
	clean := strings.ReplaceAll(dayStr, "-", "")
	return strconv.Atoi(clean)
}

// ============================================================================
// 单日选股执行
// ============================================================================

// runDay 对指定交易日执行全市场选股，返回通过策略的股票列表。
func runDay(reg *indicator.Registry, tradeDate int, configs []*indicator.SignalConfig) []selectionRecord {
	// 加载当日有 K 线数据的非退市股票
	stocks, err := db.LoadStockCodesByTradeDate(tradeDate)
	if err != nil {
		log.Printf("  [WARN] 加载股票列表失败: %v", err)
		return nil
	}
	if len(stocks) == 0 {
		return nil
	}

	// 构建 StockSource 列表（懒加载）
	sources := make([]indicator.StockSource, len(stocks))
	for i := range stocks {
		sources[i] = stocksource.NewDBStock(&stocks[i], tradeDate)
	}

	// 执行选股引擎
	results := reg.Engine().Execute(sources, configs, maxConcurrency)

	// 收集通过的股票
	var records []selectionRecord
	for _, r := range results {
		if r.Result != indicator.ResultPassed {
			continue
		}
		records = append(records, selectionRecord{
			Code:        r.Code,
			Name:        r.Name,
			SelectDate:  tradeDate,
			SelectClose: r.Price,
		})
	}
	return records
}

// ============================================================================
// 策略加载与信号配置构建
// ============================================================================

// loadStrategyConfigs 从数据库加载策略，解析 Conditions JSON 并构建 SignalConfig 列表。
func loadStrategyConfigs(id uint) ([]*indicator.SignalConfig, error) {
	strategy, err := db.GetStrategyByID(id)
	if err != nil {
		return nil, fmt.Errorf("查询策略 %d 失败: %w", id, err)
	}
	if strategy == nil {
		return nil, fmt.Errorf("策略 %d 不存在", id)
	}

	var signals []model.StrategySignal
	if err := json.Unmarshal([]byte(strategy.Conditions), &signals); err != nil {
		return nil, fmt.Errorf("解析策略条件失败: %w", err)
	}
	if len(signals) == 0 {
		return nil, fmt.Errorf("策略 %d 无信号条件", id)
	}

	log.Printf("策略名称: %s, 逻辑: %s", strategy.Name, strategy.LogicalOp)

	configs := make([]*indicator.SignalConfig, 0, len(signals))
	for _, sig := range signals {
		cfg := &indicator.SignalConfig{
			SignalID: sig.SignalID,
			Operator: mapOperator(sig.Operator),
			Params:   convertParams(sig.Params),
		}
		configs = append(configs, cfg)
	}

	return configs, nil
}

// mapOperator 将策略存储的运算符字符串映射为 CompareOperator。
func mapOperator(op string) indicator.CompareOperator {
	switch op {
	case ">", "gt":
		return indicator.OpGT
	case ">=", "gte":
		return indicator.OpGTE
	case "<", "lt":
		return indicator.OpLT
	case "<=", "lte":
		return indicator.OpLTE
	case "=", "==", "eq":
		return indicator.OpEQ
	case "!=", "neq":
		return indicator.OpNEQ
	case "between":
		return indicator.OpBetween
	case "not_between":
		return indicator.OpNotBetween
	case "in":
		return indicator.OpIn
	case "not_in":
		return indicator.OpNotIn
	case "cross_up", "cross_above":
		return indicator.OpCrossAbove
	case "cross_down", "cross_below":
		return indicator.OpCrossBelow
	case "rising":
		return indicator.OpRising
	case "falling":
		return indicator.OpFalling
	default:
		log.Printf("[WARN] 未知运算符: %s，按原值使用", op)
		return indicator.CompareOperator(op)
	}
}

// convertParams 将策略 JSON 中的参数转为 map[string]any。
// JSON 反序列化时数字均为 float64，与 SignalConfig.Params 的 any 类型兼容。
func convertParams(params map[string]interface{}) map[string]any {
	if params == nil {
		return nil
	}
	result := make(map[string]any, len(params))
	for k, v := range params {
		result[k] = v
	}
	return result
}

// ============================================================================
// 未来K线查询
// ============================================================================

// futureKline 单条未来K线记录（价格单位：分，从 DB 原始值直接读取）
type futureKline struct {
	TradeDate int // 交易日 YYYYMMDD
	Open      int // 开盘价（分）
	High      int // 最高价（分）
	Low       int // 最低价（分）
	Close     int // 收盘价（分）
}

// queryFutureKlines 为每条选股记录查询未来 N 个交易日的 K 线。
// 返回 map[key] -> []futureKline，key 为 "code_selectDate"。
// K 线数据最多保留到今日，超出今天的交易日期不纳入。
func queryFutureKlines(records []selectionRecord) map[string][]futureKline {
	today := utils.TodayTradeDate()
	result := make(map[string][]futureKline, len(records))

	for _, rec := range records {
		key := fmt.Sprintf("%s_%d", rec.Code, rec.SelectDate)

		klines, err := db.FindDailyKlinesAfterDate(rec.Code, rec.SelectDate, klineQueryLimit)
		if err != nil {
			log.Printf("[WARN] 查询未来K线失败 %s: %v", rec.Code, err)
			result[key] = nil
			continue
		}

		futures := make([]futureKline, 0, futureTradingDays)
		for _, k := range klines {
			if len(futures) >= futureTradingDays {
				break
			}
			// K 线日期不超过今天
			if k.TradeDate > today {
				break
			}
			futures = append(futures, futureKline{
				TradeDate: k.TradeDate,
				Open:      k.Open,
				High:      k.High,
				Low:       k.Low,
				Close:     k.Close,
			})
		}
		result[key] = futures
	}
	return result
}

// ============================================================================
// Excel 导出
// ============================================================================

// exportExcel 将选股结果和未来走势导出到 Excel（4 个 Sheet：Open/High/Low/Close）。
//
// 表头日期：以所有选股记录中最晚的选出日为基准日 D+0，向后生成 futureTradingDays-1 个
// 交易日作为列头。每行从"自己的选出日"对齐到表头对应位置，之前/之后均显示 "--"。
func exportExcel(records []selectionRecord, futureData map[string][]futureKline, path string) error {
	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			log.Printf("[WARN] 关闭 Excel 文件失败: %v", err)
		}
	}()

	// 创建样式
	styles := createPriceStyles(f)

	// 计算最早/最晚选出日
	minSelectDate := 0
	maxSelectDate := 0
	for _, rec := range records {
		if minSelectDate == 0 || rec.SelectDate < minSelectDate {
			minSelectDate = rec.SelectDate
		}
		if rec.SelectDate > maxSelectDate {
			maxSelectDate = rec.SelectDate
		}
	}
	if minSelectDate == 0 {
		return fmt.Errorf("无有效选股记录")
	}

	// 表头日期范围 = [最早选出日, 最晚选出日 + futureTradingDays 个交易日]
	// 表头列 = 该区间内所有 A 股交易日（自然跳过周末和节假日）
	headerDates := buildHeaderRange(minSelectDate, maxSelectDate, futureTradingDays)

	// sheet 名称列表
	sheets := []string{"Open", "High", "Low", "Close"}

	for i, sheet := range sheets {
		if i == 0 {
			f.SetSheetName("Sheet1", sheet)
		} else {
			if _, err := f.NewSheet(sheet); err != nil {
				return fmt.Errorf("创建 Sheet %s 失败: %w", sheet, err)
			}
		}

		if err := writeSheet(f, sheet, records, futureData, headerDates, i, styles); err != nil {
			return fmt.Errorf("写入 Sheet %s 失败: %w", sheet, err)
		}
	}

	// 删除默认 Sheet1（如果还有）
	f.DeleteSheet("Sheet1")

	if err := f.SaveAs(path); err != nil {
		return fmt.Errorf("保存 Excel 失败: %w", err)
	}
	return nil
}

// buildHeaderRange 生成表头日期列表：[minSelectDate, maxSelectDate + futureTradingDays] 区间内
// 所有 A 股交易日（自动跳过周末和节假日）。
func buildHeaderRange(minSelectDate, maxSelectDate, futureTradingDays int) []int {
	// 终点 = maxSelectDate + futureTradingDays 个交易日，不超过今天
	today := utils.TodayTradeDate()
	endDate, err := holiday.GetProvider().AddTradingDays(
		fmt.Sprintf("%04d-%02d-%02d",
			maxSelectDate/10000, (maxSelectDate%10000)/100, maxSelectDate%100),
		futureTradingDays,
	)
	if err != nil {
		log.Printf("[WARN] 计算表头终点失败: %v", err)
		endDate = fmt.Sprintf("%04d-%02d-%02d",
			maxSelectDate/10000, (maxSelectDate%10000)/100, maxSelectDate%100)
	}
	endDateInt, _ := parseTradeDate(endDate)
	if endDateInt > today {
		endDate = fmt.Sprintf("%04d-%02d-%02d",
			today/10000, (today%10000)/100, today%100)
	}

	startDate := fmt.Sprintf("%04d-%02d-%02d",
		minSelectDate/10000, (minSelectDate%10000)/100, minSelectDate%100)

	dayStrs, err := holiday.GetProvider().GetTradingDays(startDate, endDate)
	if err != nil {
		log.Printf("[WARN] 获取表头交易日失败: %v", err)
		return nil
	}

	dates := make([]int, 0, len(dayStrs))
	for _, s := range dayStrs {
		td, err := parseTradeDate(s)
		if err != nil {
			continue
		}
		dates = append(dates, td)
	}
	return dates
}

// priceStyleSet 价格样式集合
type priceStyleSet struct {
	red    int
	black  int
	green  int
	header int
}

// createPriceStyles 创建三种价格颜色样式和表头样式
func createPriceStyles(f *excelize.File) *priceStyleSet {
	redStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: "FF0000", Size: 10},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	blackStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: "000000", Size: 10},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	greenStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: "008000", Size: 10},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 10},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"D9E1F2"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})

	return &priceStyleSet{
		red:    redStyle,
		black:  blackStyle,
		green:  greenStyle,
		header: headerStyle,
	}
}

// writeSheet 写入一个 Sheet 的数据。
// priceType: 0=Open, 1=High, 2=Low, 3=Close
// headerDates 为表头日期序列（YYYYMMDD int），长度为 futureTradingDays。
// 每行从自身选出日对齐到表头相应位置，之前/之后均填 "--"。
func writeSheet(
	f *excelize.File,
	sheet string,
	records []selectionRecord,
	futureData map[string][]futureKline,
	headerDates []int,
	priceType int,
	styles *priceStyleSet,
) error {
	// 写入表头
	headerLabels := append([]string{"股票代码", "股票名称", "选出日期", "选出日收盘价"},
		intToStrings(headerDates)...)
	for col, h := range headerLabels {
		cell, _ := excelize.CoordinatesToCellName(col+1, 1)
		f.SetCellValue(sheet, cell, h)
		f.SetCellStyle(sheet, cell, cell, styles.header)
	}

	// 填写数据
	for rowIdx, rec := range records {
		excelRow := rowIdx + 2 // 从第2行开始（第1行是表头）

		key := fmt.Sprintf("%s_%d", rec.Code, rec.SelectDate)
		futures := futureData[key]

		// 固定列：代码、名称、选出日期、选出日收盘价
		f.SetCellValue(sheet, cellName(1, excelRow), rec.Code)
		f.SetCellValue(sheet, cellName(2, excelRow), rec.Name)
		f.SetCellValue(sheet, cellName(3, excelRow), rec.SelectDate)
		selectCloseCell := cellName(4, excelRow)
		f.SetCellValue(sheet, selectCloseCell, rec.SelectClose)
		f.SetCellStyle(sheet, selectCloseCell, selectCloseCell, styles.black)

		// 选出日收盘价（分），用于颜色比较
		selectCloseFen := int(math.Round(rec.SelectClose * 100))

		// 将 futureKline 按 TradeDate 建立索引，同时记录最大 K 线日期
		priceByDate := make(map[int]int, len(futures))
		maxKlineDate := 0
		for _, fk := range futures {
			priceByDate[fk.TradeDate] = getPriceByType(fk, priceType)
			if fk.TradeDate > maxKlineDate {
				maxKlineDate = fk.TradeDate
			}
		}

		// 最小 K 线日期：表头中第一个 ≥ 该日期的列就是 D+0
		var minKlineDate int
		if len(futures) > 0 {
			minKlineDate = futures[0].TradeDate
		}

		// 按表头日期逐列填值
		for colIdx, headerDate := range headerDates {
			col := 5 + colIdx // 从第5列开始（前4列是元数据）
			cell := cellName(col, excelRow)

			// 表头日期早于 K 线起点（选出日+1）：留空
			if minKlineDate > 0 && headerDate < minKlineDate {
				continue
			}
			// 超出该股票实际 K 线范围：留空
			if maxKlineDate > 0 && headerDate > maxKlineDate {
				continue
			}
			// 范围内：有 K 线则填，无则 "--"（停牌）
			priceFen, ok := priceByDate[headerDate]
			if !ok {
				f.SetCellValue(sheet, cell, "--")
				f.SetCellStyle(sheet, cell, cell, styles.black)
				continue
			}
			f.SetCellValue(sheet, cell, formatPriceFen(priceFen))
			f.SetCellStyle(sheet, cell, cell, pickPriceStyle(priceFen, selectCloseFen, styles))
		}
	}

	// 设置列宽
	setColumnWidths(f, sheet)

	return nil
}

// intToStrings 将 int 切片转为字符串切片
func intToStrings(ints []int) []string {
	result := make([]string, len(ints))
	for i, v := range ints {
		result[i] = strconv.Itoa(v)
	}
	return result
}

// getPriceByType 根据类型提取价格（分）
func getPriceByType(f futureKline, priceType int) int {
	switch priceType {
	case 0:
		return f.Open
	case 1:
		return f.High
	case 2:
		return f.Low
	case 3:
		return f.Close
	default:
		return f.Close
	}
}

// pickPriceStyle 根据价格（分）与基准（分）的比较选择样式
func pickPriceStyle(priceFen, baseFen int, styles *priceStyleSet) int {
	if priceFen > baseFen {
		return styles.red
	}
	if priceFen < baseFen {
		return styles.green
	}
	return styles.black
}

// formatPriceFen 将数据库价格（分）转为 "XXX.XX" 字符串。
// 在倒数第2位前插入小数点，不足三位高位补零。
// 例如: 12345 → "123.45", 50 → "0.50", 1234567 → "12345.67"
func formatPriceFen(fen int) string {
	s := strconv.Itoa(fen)
	n := len(s)
	if n <= 2 {
		// 不足两位，前补零
		s = fmt.Sprintf("%03s", s)
		return "0." + s[len(s)-2:]
	}
	return s[:n-2] + "." + s[n-2:]
}

// cellName 将列号和行号转为 Excel 单元格名（如 A1）
func cellName(col, row int) string {
	name, _ := excelize.CoordinatesToCellName(col, row)
	return name
}

// setColumnWidths 设置列宽
func setColumnWidths(f *excelize.File, sheet string) {
	f.SetColWidth(sheet, "A", "A", 12) // 股票代码
	f.SetColWidth(sheet, "B", "B", 12) // 股票名称
	f.SetColWidth(sheet, "C", "C", 14) // 选出日期
	f.SetColWidth(sheet, "D", "D", 14) // 选出日收盘价

	// 未来交易日列
	startCol, _ := excelize.ColumnNumberToName(5)
	endCol, _ := excelize.ColumnNumberToName(5 + futureTradingDays - 1)
	f.SetColWidth(sheet, startCol, endCol, 10)
}

// ============================================================================
// 编译期接口校验
// ============================================================================

// 确保 DBStock 实现 StockSource
var _ indicator.StockSource = (*stocksource.DBStock)(nil)
