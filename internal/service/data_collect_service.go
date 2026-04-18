package service

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"stock-ai/internal/adapter"
	"stock-ai/internal/db"
	"stock-ai/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ========== 数据采集服务 ==========

// DataCollectService 数据采集服务
type DataCollectService struct {
	db       *gorm.DB
	registry *adapter.Registry // 数据源注册中心
}

func NewDataCollectService() *DataCollectService {
	return &DataCollectService{
		db:       db.GetDB(),
		registry: adapter.GetRegistry(),
	}
}

// ========== 采集结果 ==========

// CollectResult 采集结果
type CollectResult struct {
	Total     int `json:"total"`      // 总条数
	NewCount  int `json:"new_count"`  // 新增数量
	UpdCount  int `json:"upd_count"`  // 更新数量
	FailCount int `json:"fail_count"` // 失败数量
}

// KLineCollectRequest K线采集请求参数
type KLineCollectRequest struct {
	Source  string `json:"source"`   // 数据源名称: eastmoney / ths
	KLineType string `json:"kline_type"` // 周期: daily / weekly / monthly / yearly
	AdjType string `json:"adj_type"`   // 复权类型: qfq(前复权)/不复权/bqq(后复权), 默认 qfq
}

// KLinePeriod K线周期常量
const (
	KLineDaily   = "daily"
	KLineWeekly  = "weekly"
	KLineMonthly = "monthly"
	KLineYearly  = "yearly"
)

// ========== 采集执行 ==========

// CollectStockList 执行股票列表采集（外部调用入口）
// 流程: 指定数据源 → 获取全量股票列表 → 遍历获取详情 → upsert入库
func (s *DataCollectService) CollectStockList(sourceName string) (*CollectResult, error) {
	ctx := context.Background()

	// 1. 获取适配器
	adp, err := s.getAdapter(sourceName)
	if err != nil {
		return nil, fmt.Errorf("获取数据源失败: %w", err)
	}

	log.Printf("[采集] 开始股票列表采集, source=%s", sourceName)

	// 2. 获取全量股票列表
	allStocks, err := adp.GetStockList(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取股票列表失败: %w", err)
	}

	log.Printf("[采集] 股票列表获取完成, 共 %d 只", len(allStocks))

	// 3. 遍历每只股票获取详情并upsert入库
	result := &CollectResult{Total: len(allStocks)}
	for i, stock := range allStocks {
		code := stock.Code

		detail, detailErr := adp.GetStockDetail(ctx, code)
		if detailErr != nil {
			log.Printf("[采集] 获取详情失败 [%s]: %v", code, detailErr)
			continue
		}

		newStock := toModelStock(code, detail)
		rowsAffected := s.upsertStock(newStock)

		if rowsAffected == 0 {
			result.NewCount++
		} else {
			result.UpdCount++
		}

		// 每100只打印一次进度
		if (i+1)%100 == 0 || i == len(allStocks)-1 {
			log.Printf("[采集] 详情进度: %d/%d (新增%d, 更新%d)", i+1, len(allStocks), result.NewCount, result.UpdCount)
		}
	}

	log.Printf("[采集] 完成: total=%d, new=%d, upd=%d", result.Total, result.NewCount, result.UpdCount)
	return result, nil
}

// CollectStockDetail 采集单只股票详情（外部调用入口）
func (s *DataCollectService) CollectStockDetail(sourceName, code string) (*model.Stock, error) {
	ctx := context.Background()

	adp, err := s.getAdapter(sourceName)
	if err != nil {
		return nil, fmt.Errorf("获取数据源失败: %w", err)
	}

	detail, err := adp.GetStockDetail(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("获取股票详情失败: %w", err)
	}

	// upsert (原子操作，无并发问题)
	stock := toModelStock(code, detail)
	s.upsertStock(stock)

	log.Printf("[采集] 详情采集完成 [%s]: %s", code, stock.Name)
	return &stock, nil
}

// ========== K线采集 ==========

// CollectKLine 采集单只股票的K线数据
func (s *DataCollectService) CollectKLine(sourceName, code, klineType, adjType string) (*CollectResult, error) {
	ctx := context.Background()

	adp, err := s.getAdapter(sourceName)
	if err != nil {
		return nil, fmt.Errorf("获取数据源失败: %w", err)
	}

	if adjType == "" {
		adjType = adapter.AdjQFQ
	}

	klines, err := s.fetchKLines(ctx, adp, code, klineType, adjType)
	if err != nil {
		return nil, err
	}

	result := s.upsertKLines(code, klineType, klines)
	log.Printf("[采集-K线] 完成 [%s/%s]: total=%d, new=%d, upd=%d", code, klineType, result.Total, result.NewCount, result.UpdCount)
	return result, nil
}

// CollectKLineBatch 全量采集所有股票的K线数据
// 流程: 从数据库获取全量股票列表 → 顺序执行每只股票的K线采集
func (s *DataCollectService) CollectKLineBatch(sourceName, klineType, adjType string) (*CollectResult, error) {
	ctx := context.Background()

	adp, err := s.getAdapter(sourceName)
	if err != nil {
		return nil, fmt.Errorf("获取数据源失败: %w", err)
	}

	if adjType == "" {
		adjType = adapter.AdjQFQ
	}

	// 从数据库获取全量股票列表
	var stocks []model.Stock
	if err := s.db.Select("code").Find(&stocks).Error; err != nil {
		return nil, fmt.Errorf("获取股票列表失败: %w", err)
	}

	if len(stocks) == 0 {
		return &CollectResult{}, nil
	}

	log.Printf("[采集-K线] 开始全量%sk线采集, 共 %d 只股票, 数据源=%s", klineType, len(stocks), sourceName)

	result := &CollectResult{Total: len(stocks)}
	for i, stock := range stocks {
		klines, fetchErr := s.fetchKLines(ctx, adp, stock.Code, klineType, adjType)
		if fetchErr != nil {
			log.Printf("[采集-K线] 获取K线失败 [%s]: %v", stock.Code, fetchErr)
			result.FailCount++
			continue
		}

		partial := s.upsertKLines(stock.Code, klineType, klines)
		result.NewCount += partial.NewCount
		result.UpdCount += partial.UpdCount

		if (i+1)%100 == 0 || i == len(stocks)-1 {
			log.Printf("[采集-K线] 全量%s进度: %d/%d (新增%d, 更新%d)", klineType, i+1, len(stocks), result.NewCount, result.UpdCount)
		}
	}

	log.Printf("[采集-K线] 全量%s完成: total=%d, new=%d, upd=%d, fail=%d", klineType, result.Total, result.NewCount, result.UpdCount, result.FailCount)
	return result, nil
}

// ========== 内部辅助函数 ==========

// getAdapter 从注册中心获取指定名称的适配器
func (s *DataCollectService) getAdapter(preferredName string) (adapter.DataSource, error) {
	if preferredName != "" {
		adp, ok := s.registry.Get(preferredName)
		if ok {
			return adp, nil
		}
		return nil, fmt.Errorf("数据源未注册或不可用: %s", preferredName)
	}

	// 没指定则按优先级从数据库取可用源
	var configs []model.DataSourceConfig
	s.db.Where("status = ?", "active").Order("priority ASC").Find(&configs)

	for _, cfg := range configs {
		adp, ok := s.registry.Get(cfg.Name)
		if ok && cfg.IsActive() {
			return adp, nil
		}
	}

	// fallback: 注册中心的第一个可用源
	for _, name := range s.registry.Names() {
		adp, ok := s.registry.Get(name)
		if ok {
			return adp, nil
		}
	}

	return nil, fmt.Errorf("无可用数据源，请先在 config.yaml 中启用并注册数据源")
}

// upsertStock 原子 upsert 操作 (INSERT ON DUPLICATE KEY UPDATE)
// 返回受影响的行数: 0=新增(INSERT), >0=更新(UPDATE)
func (s *DataCollectService) upsertStock(stock model.Stock) int64 {
	result := s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "code"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"name", "full_name", "english_name",
			"exchange", "exchange_name",
			"listing_board", "board_name",
			"list_date", "delist_date",
			"issue_price", "issue_pe", "issue_shares",
			"industry", "industry_code", "sector",
			"updated",
		}),
	}).Create(&stock)
	if result.Error != nil {
		log.Printf("[采集] upsert失败 [%s]: %v", stock.Code, result.Error)
		return -1
	}
	return result.RowsAffected
}

// toModelStock 将适配器的 StockBasic 转换为 GORM 模型 model.Stock
func toModelStock(code string, detail *adapter.StockBasic) model.Stock {
	if detail == nil {
		return model.Stock{Code: code}
	}
	return model.Stock{
		Code:         code,
		Name:         detail.Name,
		FullName:     detail.FullName,
		EnglishName:  detail.FullNameEn,
		Exchange:     detail.Exchange,
		ExchangeName: getExchangeName(detail.Exchange),
		ListingBoard: detail.ListingBoard,
		BoardName:    getBoardName(detail.ListingBoard),
		ListDate:     detail.ListDate,
		IssuePrice:   detail.IssuePrice,
		IssuePE:      detail.IssuePE,
		IssueShares:  detail.TotalIssueNum,
		Industry:     detail.Industry,
		Sector:       detail.Sector,
	}
}

// getExchangeName 获取交易所中文名
func getExchangeName(exchange string) string {
	switch exchange {
	case "SSE":
		return "上海证券交易所"
	case "SZSE":
		return "深圳证券交易所"
	case "BSE":
		return "北京证券交易所"
	default:
		return exchange
	}
}

// getBoardName 获取板块中文名
func getBoardName(board string) string {
	switch board {
	case "main":
		return "主板"
	case "chinext":
		return "创业板"
	case "star":
		return "科创板"
	case "bse":
		return "北交所"
	default:
		return board
	}
}

// ========== K线辅助函数 ==========

// fetchKLines 根据周期调用对应的 adapter 方法获取K线数据
func (s *DataCollectService) fetchKLines(ctx context.Context, adp adapter.DataSource, code, klineType, adjType string) ([]adapter.StockPriceDaily, error) {
	switch klineType {
	case KLineDaily:
		return adp.GetDailyKLine(ctx, code, adjType)
	case KLineWeekly:
		return adp.GetWeeklyKLine(ctx, code, adjType)
	case KLineMonthly:
		return adp.GetMonthlyKLine(ctx, code, adjType)
	case KLineYearly:
		return adp.GetYearlyKLine(ctx, code, adjType)
	default:
		return nil, fmt.Errorf("不支持的K线周期: %s (支持: daily/weekly/monthly/yearly)", klineType)
	}
}

// upsertKLines 将K线数据批量写入对应周期的表
// 返回统计结果
func (s *DataCollectService) upsertKLines(code string, klineType string, klines []adapter.StockPriceDaily) *CollectResult {
	result := &CollectResult{Total: len(klines)}
	if len(klines) == 0 {
		return result
	}

	for _, k := range klines {
		tradeDate := parseTradeDate(k.Date)
		rowsAffected := s.doKlineUpsert(code, tradeDate, klineType, &k)

		if rowsAffected == 0 {
			result.NewCount++
		} else if rowsAffected > 0 {
			result.UpdCount++
		}
	}

	return result
}

// doKlineUpsert 根据周期类型执行具体的 upsert 操作（内部根据类型创建正确的 model）
func (s *DataCollectService) doKlineUpsert(code string, tradeDate int, klineType string, k *adapter.StockPriceDaily) int64 {
	switch klineType {
	case KLineDaily:
		m := model.DailyKline{
			StockCode: code, TradeDate: tradeDate,
			Open: int(k.Open), High: int(k.High), Low: int(k.Low), Close: int(k.Close),
			Volume: k.Volume, Amount: k.Amount, TurnoverRate: k.Turnover,
		}
		return s.upsertKlineRecord("daily_kline", m)
	case KLineWeekly:
		m := model.WeeklyKline{
			StockCode: code, TradeDate: tradeDate,
			Open: int(k.Open), High: int(k.High), Low: int(k.Low), Close: int(k.Close),
			Volume: k.Volume, Amount: k.Amount, TurnoverRate: k.Turnover,
		}
		return s.upsertKlineRecord("weekly_kline", m)
	case KLineMonthly:
		m := model.MonthlyKline{
			StockCode: code, TradeDate: tradeDate,
			Open: int(k.Open), High: int(k.High), Low: int(k.Low), Close: int(k.Close),
			Volume: k.Volume, Amount: k.Amount, TurnoverRate: k.Turnover,
		}
		return s.upsertKlineRecord("monthly_kline", m)
	case KLineYearly:
		m := model.YearlyKline{
			StockCode: code, TradeDate: tradeDate,
			Open: int(k.Open), High: int(k.High), Low: int(k.Low), Close: int(k.Close),
			Volume: k.Volume, Amount: k.Amount, TurnoverRate: k.Turnover,
		}
		return s.upsertKlineRecord("yearly_kline", m)
	default:
		return -1
	}
}

// upsertKlineRecord 通用K线 upsert (INSERT ON DUPLICATE KEY UPDATE)
func (s *DataCollectService) upsertKlineRecord(tableName string, model interface{}) int64 {
	updateCols := []string{"open", "high", "low", "close", "volume", "amount", "turnover_rate"}
	result := s.db.Table(tableName).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "stock_code"}, {Name: "trade_date"}},
		DoUpdates: clause.AssignmentColumns(updateCols),
	}).Create(model)

	if result.Error != nil {
		log.Printf("[采集-K线] upsert失败 [%s]: %v", tableName, result.Error)
		return -1
	}
	return result.RowsAffected
}

// parseTradeDate 将 YYYY-MM-DD 格式日期转为 YYYYMMDD 整数
func parseTradeDate(dateStr string) int {
	if len(dateStr) >= 10 {
		if v, err := strconv.Atoi(dateStr[:4] + dateStr[5:7] + dateStr[8:10]); err == nil {
			return v
		}
	}
	return 0
}
