package service

import (
	"fmt"
	"log"
	"strings"
	"time"

	"stock-ai/internal/db"
	"stock-ai/internal/model"

	"gorm.io/gorm"
)

// ========== 选股服务 (API 请求/响应类型) ==========

// FilterRequest 筛选请求
type FilterRequest struct {
	Query      string            `json:"query"`      // 自然语言查询
	Conditions []FilterCond      `json:"conditions"` // 结构化条件
	Market     string            `json:"market"`     // A股/港股/美股
}

type FilterCond struct {
	Field    string      `json:"field"`    // 均线/MACD/KDJ等
	Operator string      `json:"operator"` // 大于/小于/等于
	Value    interface{} `json:"value"`    // 数值
}

// FilterResponse 筛选响应
type FilterResponse struct {
	Success         bool        `json:"success"`
	Message         string      `json:"message"`
	Total           int         `json:"total"`
	Stocks          []StockInfo `json:"stocks"`
	ExecutionTimeMs int64       `json:"execution_time_ms"`
}

// StockInfo 股票信息(返回给前端的)
type StockInfo struct {
	Code              string   `json:"code"`
	Name              string   `json:"name"`
	Exchange          string   `json:"exchange"`
	ListingBoard      string   `json:"listing_board"`
	Industry          string   `json:"industry"`
	Price             float64  `json:"price"`
	ChangePercent     float64  `json:"change_percent"`
	Volume            int64    `json:"volume"`
	TotalMarketCap    float64  `json:"total_market_cap"`
	CirculateMarketCap float64 `json:"circulate_market_cap"`
	IssuePrice        float64  `json:"issue_price"`
	IssuePE           float64  `json:"issue_pe"`
	ListDate          string   `json:"list_date"`
	MatchedConditions []string `json:"matched_conditions"`
}

// HotTopicResponse 热门题材响应
type HotTopicResponse struct {
	Success bool           `json:"success"`
	Topics  []HotTopicInfo `json:"topics"`
}

type HotTopicInfo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Tag         string   `json:"tag"`
	Description string   `json:"description"`
	Stocks      []string `json:"related_stocks"`
	Heat        int      `json:"heat"`
}

// StockDetailResponse 股票详情响应
type StockDetailResponse struct {
	Success bool                `json:"success"`
	Stock  model.Stock         `json:"stock"`
	Price  *model.StockPrice   `json:"price"`
}

// PriceHistoryResponse 价格历史响应
type PriceHistoryResponse struct {
	Success bool                  `json:"success"`
	Code    string                `json:"code"`
	Prices  []model.StockPrice   `json:"prices"`
}

type StockService struct {
	db *gorm.DB
}

func NewStockService() *StockService {
	return &StockService{
		db: db.GetDB(),
	}
}

// FilterStocks 条件选股
func (s *StockService) FilterStocks(req FilterRequest) (*FilterResponse, error) {
	start := time.Now()

	query := s.db.Model(&model.Stock{}).Joins("JOIN stock_prices ON stocks.code = stock_prices.stock_code")
	query = query.Where("stock_prices.date = ?", getLatestTradeDate())

	var matchedConditions []string

	if req.Query != "" {
		query = query.Where("stocks.name LIKE ? OR stocks.code LIKE ?",
			"%"+req.Query+"%", "%"+req.Query+"%")
		matchedConditions = append(matchedConditions, "关键词: "+req.Query)
	}

	for _, cond := range req.Conditions {
		switch cond.Field {
		case "均线":
			val := parseFloat(cond.Value)
			switch cond.Operator {
			case "大于", "大于等于":
				query = query.Where("stock_prices.close >= ?", val)
				matchedConditions = append(matchedConditions, fmt.Sprintf("价格 >= %.2f", val))
			case "小于", "小于等于":
				query = query.Where("stock_prices.close <= ?", val)
				matchedConditions = append(matchedConditions, fmt.Sprintf("价格 <= %.2f", val))
			}
		case "资金流入":
			query = query.Where("stock_prices.change_pct > 0")
			matchedConditions = append(matchedConditions, "资金流入(涨)")
		case "MACD":
			val := parseFloat(cond.Value)
			switch cond.Operator {
			case "大于", "大于等于":
				query = query.Where("stock_prices.macd >= ?", val)
				matchedConditions = append(matchedConditions, fmt.Sprintf("MACD >= %.2f", val))
			case "小于", "小于等于":
				query = query.Where("stock_prices.macd <= ?", val)
				matchedConditions = append(matchedConditions, fmt.Sprintf("MACD <= %.2f", val))
			}
		case "KDJ":
			val := parseFloat(cond.Value)
			switch cond.Operator {
			case "大于", "大于等于":
				query = query.Where("stock_prices.kdj_k >= ?", val)
				matchedConditions = append(matchedConditions, fmt.Sprintf("KDJ-K >= %.2f", val))
			case "小于", "小于等于":
				query = query.Where("stock_prices.kdj_k <= ?", val)
				matchedConditions = append(matchedConditions, fmt.Sprintf("KDJ-K <= %.2f", val))
			}
		case "RSI":
			val := parseFloat(cond.Value)
			switch cond.Operator {
			case "大于", "大于等于":
				query = query.Where("stock_prices.rsi6 >= ?", val)
				matchedConditions = append(matchedConditions, fmt.Sprintf("RSI6 >= %.2f", val))
			case "小于", "小于等于":
				query = query.Where("stock_prices.rsi6 <= ?", val)
				matchedConditions = append(matchedConditions, fmt.Sprintf("RSI6 <= %.2f", val))
			}
		case "RSL":
			val := parseFloat(cond.Value)
			switch cond.Operator {
			case "大于", "大于等于":
				query = query.Where("stock_prices.volume >= ?", val*10000)
				matchedConditions = append(matchedConditions, fmt.Sprintf("成交量 >= %.0f万", val))
			}
		}
	}

	var results []struct {
		model.Stock
		model.StockPrice
	}

	if err := query.Find(&results).Error; err != nil {
		return nil, err
	}

	stocks := make([]StockInfo, 0, len(results))
	for _, r := range results {
		stocks = append(stocks, StockInfo{
			Code:              r.Stock.Code,
			Name:              r.Stock.Name,
			Exchange:          r.Stock.Exchange,
			ListingBoard:      r.Stock.ListingBoard,
			Industry:          r.Stock.Industry,
			Price:             r.StockPrice.Close,
			ChangePercent:     r.StockPrice.ChangePct,
			Volume:            r.StockPrice.Volume,
			TotalMarketCap:    r.StockPrice.TotalMarketCap,
			CirculateMarketCap: r.StockPrice.CirculateMarketCap,
			IssuePrice:        r.Stock.IssuePrice,
			IssuePE:           r.Stock.IssuePE,
			ListDate:          r.Stock.ListDate,
			MatchedConditions: matchedConditions,
		})
	}

	executionTime := time.Since(start).Milliseconds()

	return &FilterResponse{
		Success:         true,
		Message:         "选股完成",
		Total:           len(stocks),
		Stocks:          stocks,
		ExecutionTimeMs: executionTime,
	}, nil
}

// AIQuery AI自然语言选股
func (s *StockService) AIQuery(query string) (*FilterResponse, error) {
	req := FilterRequest{Query: query}
	queryLower := strings.ToLower(query)

	if strings.Contains(queryLower, "macd金叉") || strings.Contains(queryLower, "macd 金叉") {
		req.Conditions = append(req.Conditions, FilterCond{Field: "MACD", Operator: "大于", Value: 0})
	}
	if strings.Contains(queryLower, "kdj超卖") || strings.Contains(queryLower, "kdj 超卖") {
		req.Conditions = append(req.Conditions, FilterCond{Field: "KDJ", Operator: "小于", Value: 20})
	}
	if strings.Contains(queryLower, "涨停") {
		req.Conditions = append(req.Conditions, FilterCond{Field: "均线", Operator: "大于等于", Value: 9.9})
	}
	if strings.Contains(queryLower, "放量") {
		req.Conditions = append(req.Conditions, FilterCond{Field: "RSL", Operator: "大于", Value: 100})
	}
	if strings.Contains(queryLower, "低价") {
		req.Conditions = append(req.Conditions, FilterCond{Field: "均线", Operator: "小于", Value: 10})
	}
	if strings.Contains(queryLower, "高价") || strings.Contains(queryLower, "蓝筹") {
		req.Conditions = append(req.Conditions, FilterCond{Field: "均线", Operator: "大于", Value: 50})
	}

	return s.FilterStocks(req)
}

// GetHotTopics 获取热门题材
func (s *StockService) GetHotTopics() (*HotTopicResponse, error) {
	var topics []model.HotTopic
	if err := s.db.Order("heat DESC").Limit(10).Find(&topics).Error; err != nil {
		return nil, err
	}

	result := make([]HotTopicInfo, 0, len(topics))
	for _, t := range topics {
		stocks := strings.Split(t.RelatedCodes, ",")
		result = append(result, HotTopicInfo{
			ID:          fmt.Sprintf("%d", t.ID),
			Name:        t.Name,
			Tag:         t.Tag,
			Description: t.Description,
			Stocks:      stocks,
			Heat:        t.Heat,
		})
	}

	return &HotTopicResponse{Success: true, Topics: result}, nil
}

// GetStockDetail 获取股票详情(包含基本信息+最新价格)
func (s *StockService) GetStockDetail(code string) (*StockDetailResponse, error) {
	var stock model.Stock
	if err := s.db.Where("code = ?", code).First(&stock).Error; err != nil {
		return nil, fmt.Errorf("股票不存在 %s: %w", code, err)
	}

	var price model.StockPrice
	priceErr := s.db.Where("stock_code = ?", code).Order("date DESC").First(&price).Error

	resp := &StockDetailResponse{Success: true, Stock: stock}
	if priceErr == nil {
		resp.Price = &price
	}

	return resp, nil
}

// GetStockPrices 获取股票历史价格
func (s *StockService) GetStockPrices(code string, days int) (*PriceHistoryResponse, error) {
	if days <= 0 { days = 30 }
	if days > 365 { days = 365 }

	var prices []model.StockPrice
	if err := s.db.Where("stock_code = ?", code).Order("date ASC").Limit(days).Find(&prices).Error; err != nil {
		return nil, err
	}

	return &PriceHistoryResponse{Success: true, Code: code, Prices: prices}, nil
}

// InitMockData 初始化模拟数据(开发测试使用DataCollectService的mock适配器)
func (s *StockService) InitMockData() error {
	var count int64
	s.db.Model(&model.Stock{}).Count(&count)
	if count > 0 { return nil }

	log.Println("开始初始化模拟数据...")

	collector := NewDataCollectService()
	task, err := collector.CollectStockList("mock")
	if err != nil { return fmt.Errorf("获取股票列表失败: %w", err) }

	topics := []model.HotTopic{
		{Name: "人形机器人", Tag: "热门", Description: "AI赋能机器人产业加速落地，关注核心零部件与系统集成商", Keywords: "机器人,AI,智能硬件", RelatedCodes: "002230,300124,002049", Heat: 95, Trend: "up"},
		{Name: "商业航天", Tag: "政策", Description: "商业航天政策持续催化，卫星互联网产业链迎来爆发期", Keywords: "卫星,航天,低轨轨道", RelatedCodes: "600118,300455,002025", Heat: 88, Trend: "up"},
		{Name: "短视频游戏", Tag: "游戏", Description: "短剧+游戏融合新赛道，IP变现模式持续创新", Keywords: "游戏,短视频,元宇宙", RelatedCodes: "300413,002624,300031", Heat: 82, Trend: "stable"},
		{Name: "存储芯片", Tag: "芯片", Description: "存储芯片周期底部已现，AI算力需求驱动复苏", Keywords: "存储,NAND,DRAM", RelatedCodes: "603986,603501,300223", Heat: 78, Trend: "up"},
		{Name: "创新药", Tag: "医药", Description: "国产创新药出海加速，关注临床管线进展", Keywords: "新药,临床,CRO", RelatedCodes: "600276,300122,688235", Heat: 75, Trend: "stable"},
	}

	for _, topic := range topics {
		s.db.Create(&topic)
	}

	defaultSources := []model.DataSourceConfig{
		{Name: "mock", DisplayName: "模拟数据(开发测试)", Type: "sdk", Status: "active", Priority: 100, Config: `{}`, Description: "用于开发和测试的模拟数据源"},
		{Name: "tushare", DisplayName: "Tushare Pro", Type: "api", Status: "disabled", Priority: 1, DailyQuota: 5000, Description: "Tushare Pro金融数据接口"},
		{Name: "eastmoney", DisplayName: "东方财富", Type: "web_crawl", Status: "disabled", Priority: 2, Description: "东方财富网数据爬取"},
		{Name: "akshare", DisplayName: "AKShare", Type: "sdk", Status: "disabled", Priority: 3, Description: "开源金融数据接口库"},
	}

	for _, src := range defaultSources {
		s.db.Create(&src)
	}

	log.Printf("模拟数据初始化完成: %d 只股票, %d 个题材, %d 个数据源", task.Total, len(topics), len(defaultSources))
	return nil
}

func parseFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64: return val
	case float32: return float64(val)
	case int: return float64(val)
	case int64: return float64(val)
	case string:
		var f float64
		fmt.Sscanf(val, "%f", &f)
		return f
	default: return 0
	}
}

func getLatestTradeDate() string {
	today := time.Now()
	switch today.Weekday() {
	case time.Sunday: return today.AddDate(0, 0, -2).Format("2006-01-02")
	case time.Saturday: return today.AddDate(0, 0, -1).Format("2006-01-02")
	default: return today.Format("2006-01-02")
	}
}
