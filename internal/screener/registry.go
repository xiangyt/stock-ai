package screener

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"time"

	"stock-ai/internal/model"
	"stock-ai/internal/screener/indicators"
	"stock-ai/internal/screener/operators"
)

// ============================================================================
//  IndicatorRegistry — 内置指标注册表
//
//  职责：
//    1. 定义所有内置指标的元数据（Indicator 定义）
//    2. 注册每个指标对应的 Evaluator
//    3. 提供预设信号模板（Signal Template）供用户快速选用
//
//  使用方式：
//    engine := NewEngine()
//    RegisterBuiltInIndicators(engine)
//    // 现在引擎支持所有内置指标的评估了
// ============================================================================

// BuiltInIndicator 所有内置指标的定义
var BuiltInIndicators = []model.Indicator{

	// ==================== 技术面：趋势类 ====================
	{
		ID:          "ma5",
		Category:    model.CategoryTechnical,
		Name:        "MA5",
		NameEn:      "MA5",
		Description: "5日移动平均线，短期趋势参考",
		ValueType:   model.ValueTypeNumber,
		DataSource:  model.DataSourceStockPrice,
		Fields:      []string{"ma5"},
		Unit:        "",
		Version:     1,
	},
	{
		ID:          "ma10",
		Category:    model.CategoryTechnical,
		Name:        "MA10",
		NameEn:      "MA10",
		Description: "10日移动平均线",
		ValueType:   model.ValueTypeNumber,
		DataSource:  model.DataSourceStockPrice,
		Fields:      []string{"ma10"},
		Unit:        "",
		Version:     1,
	},
	{
		ID:          "ma20",
		Category:    model.CategoryTechnical,
		Name:        "MA20",
		NameEn:      "MA20",
		Description: "20日移动平均线，生命线/操作线",
		ValueType:   model.ValueTypeNumber,
		DataSource:  model.DataSourceStockPrice,
		Fields:      []string{"ma20"},
		Unit:        "",
		Version:     1,
	},
	{
		ID:          "ma60",
		Category:    model.CategoryTechnical,
		Name:        "MA60",
		NameEn:      "MA60",
		Description: "60日移动平均线，决策线/季线",
		ValueType:   model.ValueTypeNumber,
		DataSource:  model.DataSourceStockPrice,
		Fields:      []string{"ma60"},
		Unit:        "",
		Version:     1,
	},
	{
		ID:          "ma_cross",
		Category:    model.CategoryTechnical,
		Name:        "均线金叉/死叉",
		NameEn:      "MA Cross",
		Description: "短期均线上穿/下穿长期均线",
		ValueType:   model.ValueTypeSeries,
		DataSource:  model.DataSourceStockPrice,
		Fields:      []string{"ma5", "ma10", "ma20"},
		Unit:        "",
		Version:     1,
	},

	// ==================== 技术面：动量类 ====================
	{
		ID:          "macd",
		Category:    model.CategoryTechnical,
		Name:        "MACD",
		NameEn:      "MACD",
		Description: "指数平滑异同移动平均线(DIF值)",
		ValueType:   model.ValueTypeNumber,
		DataSource:  model.DataSourceStockPrice,
		Fields:      []string{"macd"},
		Unit:        "",
		Version:     1,
	},
	{
		ID:          "macd_signal",
		Category:    model.CategoryTechnical,
		Name:        "MACD信号线(DEA)",
		NameEn:      "MACD Signal",
		Description: "MACD的信号线(DEA)",
		ValueType:   model.ValueTypeNumber,
		DataSource:  model.DataSourceStockPrice,
		Fields:      []string{"macd_signal"},
		Unit:        "",
		Version:     1,
	},
	{
		ID:          "macd_hist",
		Category:    model.CategoryTechnical,
		Name:        "MACD柱状图",
		NameEn:      "MACD Histogram",
		Description: "MACD柱状图(DIF-DEA)，正=红柱(多头), 负=绿柱(空头)",
		ValueType:   model.ValueTypeNumber,
		DataSource:  model.DataSourceStockPrice,
		Fields:      []string{"macd_hist"},
		Unit:        "",
		Version:     1,
	},
	{
		ID:          "macd_cross",
		Category:    model.CategoryTechnical,
		Name:        "MACD金叉/死叉",
		NameEn:      "MACD Cross",
		Description: "MACD DIF线上穿/下穿 DEA 线",
		ValueType:   model.ValueTypeSeries,
		DataSource:  model.DataSourceStockPrice,
		Fields:      []string{"macd", "macd_signal", "macd_hist"},
		Unit:        "",
		Version:     1,
	},
	{
		ID:          "macd_divergence",
		Category:    model.CategoryTechnical,
		Name:        "MACD背离",
		NameEn:      "MACD Divergence",
		Description: "价格与MACD的背离关系(底背离看涨, 顶背离看跌)",
		ValueType:   model.ValueTypeSeries,
		DataSource:  model.DataSourceKline,
		Fields:      []string{"close", "macd", "macd_hist"},
		Unit:        "",
		Version:     1,
	},

	// ==================== 行情面 ====================
	{
		ID:          "price",
		Category:    model.CategoryMarket,
		Name:        "收盘价",
		NameEn:      "Close Price",
		Description: "最新收盘价",
		ValueType:   model.ValueTypeNumber,
		DataSource:  model.DataSourceKline,
		Fields:      []string{"close"},
		Unit:        "元",
		Version:     1,
	},
	{
		ID:          "change_pct",
		Category:    model.CategoryMarket,
		Name:        "涨跌幅",
		NameEn:      "Change Percent",
		Description: "当日涨跌幅(%)",
		ValueType:   model.ValueTypeNumber,
		DataSource:  model.DataSourceStockPrice,
		Fields:      []string{"change_pct"},
		Unit:        "%",
		Version:     1,
	},
	{
		ID:          "amplitude",
		Category:    model.CategoryMarket,
		Name:        "振幅",
		NameEn:      "Amplitude",
		Description: "当日振幅(%) = (最高-最低)/昨收*100",
		ValueType:   model.ValueTypeNumber,
		DataSource:  model.DataSourceStockPrice,
		Fields:      []string{"amplitude"},
		Unit:        "%",
		Version:     1,
	},
	{
		ID:          "turnover_rate",
		Category:    model.CategoryMarket,
		Name:        "换手率",
		NameEn:      "Turnover Rate",
		Description: "当日换手率(%)",
		ValueType:   model.ValueTypeNumber,
		DataSource:  model.DataSourceKline,
		Fields:      []string{"turnover_rate"},
		Unit:        "%",
		Version:     1,
	},
	{
		ID:          "volume",
		Category:    model.CategoryMarket,
		Name:        "成交量",
		NameEn:      "Volume",
		Description: "当日成交量(手)",
		ValueType:   model.ValueTypeNumber,
		DataSource:  model.DataSourceKline,
		Fields:      []string{"volume"},
		Unit:        "手",
		Version:     1,
	},

	{
		ID:          "limit_up",
		Category:    model.CategoryMarket,
		Name:        "涨停",
		NameEn:      "Limit Up",
		Description: "当日是否涨停",
		ValueType:   model.ValueTypeBool,
		DataSource:  model.DataSourceStockPrice,
		Fields:      []string{"change_pct"},
		Unit:        "",
		Version:     1,
	},
	{
		ID:          "total_market_cap",
		Category:    model.CategoryMarket,
		Name:        "总市值",
		NameEn:      "Total Market Cap",
		Description: "总市值(亿元)",
		ValueType:   model.ValueTypeNumber,
		DataSource:  model.DataSourceStockPrice,
		Fields:      []string{"total_market_cap"},
		Unit:        "亿",
		Version:     1,
	},
	{
		ID:          "circulate_market_cap",
		Category:    model.CategoryMarket,
		Name:        "流通市值",
		NameEn:      "Circulating Market Cap",
		Description: "流通市值(亿元)",
		ValueType:   model.ValueTypeNumber,
		DataSource:  model.DataSourceStockPrice,
		Fields:      []string{"circulate_market_cap"},
		Unit:        "亿",
		Version:     1,
	},

	// ==================== 基本面 ====================
	{
		ID:          "listing_board",
		Category:    model.CategoryFundamental,
		Name:        "上市板块",
		NameEn:      "Listing Board",
		Description: "主板/创业板/科创板/北交所",
		ValueType:   model.ValueTypeEnum,
		DataSource:  model.DataSourceStockInfo,
		Fields:      []string{"listing_board"},
		Unit:        "",
		Version:     1,
	},
	{
		ID:          "industry",
		Category:    model.CategoryFundamental,
		Name:        "所属行业",
		NameEn:      "Industry",
		Description: "申万一级行业分类",
		ValueType:   model.ValueTypeEnum,
		DataSource:  model.DataSourceStockInfo,
		Fields:      []string{"industry"},
		Unit:        "",
		Version:     1,
	},
	{
		ID:          "list_date",
		Category:    model.CategoryFundamental,
		Name:        "上市时间",
		NameEn:      "List Date",
		Description: "上市日期, 可用于筛选次新股/老股",
		ValueType:   model.ValueTypeNumber,
		DataSource:  model.DataSourceStockInfo,
		Fields:      []string{"list_date"},
		Unit:        "天",
		Version:     1,
	},

	// ==================== 财务面：盈利能力 ====================
	{
		ID:          "pe_ttm",
		Category:    model.CategoryFinancial,
		Name:        "市盈率(TTM)",
		NameEn:      "PE TTM",
		Description: "滚动市盈率, <0表示亏损",
		ValueType:   model.ValueTypeNumber,
		DataSource:  model.DataSourceRealtime,
		Fields:      []string{},
		Unit:        "倍",
		Version:     1,
	},
	{
		ID:          "roe_w",
		Category:    model.CategoryFinancial,
		Name:        "ROE(加权)",
		NameEn:      "ROE (Weighted)",
		Description: "净资产收益率-加权(%), 衡量股东权益回报效率",
		ValueType:   model.ValueTypeNumber,
		DataSource:  model.DataSourceFinancial,
		Fields:      []string{"roe_w"},
		Unit:        "%",
		Version:     1,
	},
	{
		ID:          "roa",
		Category:    model.CategoryFinancial,
		Name:        "总资产收益率",
		NameEn:      "ROA",
		Description: "总资产收益率(%), 衡量资产利用效率",
		ValueType:   model.ValueTypeNumber,
		DataSource:  model.DataSourceFinancial,
		Fields:      []string{"roa"},
		Unit:        "%",
		Version:     1,
	},
	{
		ID:          "gross_margin",
		Category:    model.CategoryFinancial,
		Name:        "毛利率",
		NameEn:      "Gross Margin",
		Description: "销售毛利率(%), 毛利/营收",
		ValueType:   model.ValueTypeNumber,
		DataSource:  model.DataSourceFinancial,
		Fields:      []string{"gross_margin"},
		Unit:        "%",
		Version:     1,
	},
	{
		ID:          "net_margin",
		Category:    model.CategoryFinancial,
		Name:        "净利率",
		NameEn:      "Net Margin",
		Description: "销售净利率(%), 净利润/营收",
		ValueType:   model.ValueTypeNumber,
		DataSource:  model.DataSourceFinancial,
		Fields:      []string{"net_margin"},
		Unit:        "%",
		Version:     1,
	},

	// ==================== 财务面：成长能力 ====================
	{
		ID:          "revenue_yoy",
		Category:    model.CategoryFinancial,
		Name:        "营收同比",
		NameEn:      "Revenue YoY",
		Description: "营业总收入同比增长(%)",
		ValueType:   model.ValueTypeNumber,
		DataSource:  model.DataSourceFinancial,
		Fields:      []string{"revenue_yoy"},
		Unit:        "%",
		Version:     1,
	},
	{
		ID:          "profit_yoy",
		Category:    model.CategoryFinancial,
		Name:        "净利润同比",
		NameEn:      "Profit YoY",
		Description: "归母净利润同比增长(%)",
		ValueType:   model.ValueTypeNumber,
		DataSource:  model.DataSourceFinancial,
		Fields:      []string{"parent_net_profit_yoy"},
		Unit:        "%",
		Version:     1,
	},

	// ==================== 财务面：偿债能力 ====================
	{
		ID:          "debt_ratio",
		Category:    model.CategoryFinancial,
		Name:        "资产负债率",
		NameEn:      "Debt Ratio",
		Description: "资产负债率(%), 越低越安全(一般<60%)",
		ValueType:   model.ValueTypeNumber,
		DataSource:  model.DataSourceFinancial,
		Fields:      []string{"debt_ratio"},
		Unit:        "%",
		Version:     1,
	},
	{
		ID:          "current_ratio",
		Category:    model.CategoryFinancial,
		Name:        "流动比率",
		NameEn:      "Current Ratio",
		Description: "流动比率(倍), >1.5 较安全",
		ValueType:   model.ValueTypeNumber,
		DataSource:  model.DataSourceFinancial,
		Fields:      []string{"current_ratio"},
		Unit:        "倍",
		Version:     1,
	},
	{
		ID:          "quick_ratio",
		Category:    model.CategoryFinancial,
		Name:        "速动比率",
		NameEn:      "Quick Ratio",
		Description: "速动比率(倍), 排除存货后的短期偿债能力",
		ValueType:   model.ValueTypeNumber,
		DataSource:  model.DataSourceFinancial,
		Fields:      []string{"quick_ratio"},
		Unit:        "倍",
		Version:     1,
	},

	// ==================== 财务面：每股指标 ====================
	{
		ID:          "eps",
		Category:    model.CategoryFinancial,
		Name:        "每股收益(EPS)",
		NameEn:      "EPS",
		Description: "基本每股收益(元), >0 表示盈利",
		ValueType:   model.ValueTypeNumber,
		DataSource:  model.DataSourceFinancial,
		Fields:      []string{"basic_eps"},
		Unit:        "元",
		Version:     1,
	},
	{
		ID:          "bvps",
		Category:    model.CategoryFinancial,
		Name:        "每股净资产(BPS)",
		NameEn:      "BVPS",
		Description: "每股净资产(元)",
		ValueType:   model.ValueTypeNumber,
		DataSource:  model.DataSourceFinancial,
		Fields:      []string{"bvps"},
		Unit:        "元",
		Version:     1,
	},
	{
		ID:          "ocfps",
		Category:    model.CategoryFinancial,
		Name:        "每股经营现金流",
		NameEn:      "OCFPS",
		Description: "每股经营现金流(元)",
		ValueType:   model.ValueTypeNumber,
		DataSource:  model.DataSourceFinancial,
		Fields:      []string{"ocfps"},
		Unit:        "元",
		Version:     1,
	},
}

// ============================================================================
//  预设信号模板 (Signal Templates) — 常用信号的快捷配置
// ============================================================================

// PresetSignal 预设信号定义（含参数元信息）
//
// ParamDefs 是核心新增字段：
//   - 前端根据此数组自动渲染参数输入表单
//   - LLM 根据此数组理解可接受的参数及其约束
//   - 运行时用于校验用户/LLM 传入的参数值
type PresetSignal struct {
	ID          string                   // 模板 ID (唯一标识)
	Name        string                   // 显示名称
	Category    model.IndicatorCategory           // 所属大类
	IndicatorID string                   // 关联指标 ID
	DefaultCfg  func() model.Signal      // 默认配置工厂（返回含默认值的 Signal）
	ParamDefs   []model.SignalParamDef   // 参数元信息列表（前端表单 + LLM 约束）
}

// PresetSignals 所有预设信号
var PresetSignals = []PresetSignal{

	// ===== 技术面 =====

	// MACD 金叉
	{
		ID:          "macd_golden_cross",
		Name:        "MACD金叉",
		Category:    model.CategoryTechnical,
		IndicatorID: "macd_cross",
		DefaultCfg: func() model.Signal {
			return model.Signal{
				ID:          "tmpl_macd_golden_cross",
				IndicatorID: "macd_cross",
				Name:        "MACD金叉",
				Description: "MACD的DIF线上穿DEA线，看多信号",
				Operator:    model.OpCrossUp,
				WithinDays:  1,
				LookbackDays: 5,
				Weight:      1.0,
				Order:       1,
				Enabled:     true,
			}
		},
		ParamDefs: []model.SignalParamDef{
			{
				Key:         "within_days",
				Label:       "时间窗口(天)",
				Type:        model.ParamTypeDays,
				Default:     1,
				Min:         0,
				Max:         60,
				Step:        1,
				Required:    false,
				Description: "在最近N天内出现金叉。0=不限制窗口（仅判断是否处于金叉状态），1=今天或昨天刚金叉，3=近3天内出现过金叉",
				Placeholder: "1",
				Example:     "填1=近1天; 填3=近3天; 填0=仅看当前状态",
				Unit:        "天",
				Group:       "time_window",
			},
			{
				Key:         "ago_from_days",
				Label:       "起始偏移(天)",
				Type:        model.ParamTypeDays,
				Default:     0,
				Min:         0,
				Max:         60,
				Step:        1,
				Required:    false,
				Description: "从N天前开始看。配合时间窗口使用：ago=1 + within=3 表示 1~3天前之间是否出现金叉",
				Placeholder: "0",
				Example:     "填0=从今天开始; 填1=排除今天，从昨天开始",
				Unit:        "天",
				Group:       "time_window",
			},
		},
	},
	// MACD 死叉
	{
		ID:          "macd_death_cross",
		Name:        "MACD死叉",
		Category:    model.CategoryTechnical,
		IndicatorID: "macd_cross",
		DefaultCfg: func() model.Signal {
			return model.Signal{
				ID:          "tmpl_macd_death_cross",
				IndicatorID: "macd_cross",
				Name:        "MACD死叉",
				Description: "MACD的DIF线下穿DEA线，看空信号",
				Operator:    model.OpCrossDown,
				WithinDays:  1,
				LookbackDays: 5,
				Weight:      1.0,
				Order:       1,
				Enabled:     true,
			}
		},
		ParamDefs: []model.SignalParamDef{
			{Key: "within_days", Label: "时间窗口(天)", Type: model.ParamTypeDays, Default: 1, Min: 0, Max: 60, Step: 1,
				Description: "在最近N天内出现死叉", Placeholder: "1", Example: "1=近1天; 3=近3天; 0=仅看当前状态", Unit: "天", Group: "time_window"},
			{Key: "ago_from_days", Label: "起始偏移(天)", Type: model.ParamTypeDays, Default: 0, Min: 0, Max: 60, Step: 1,
				Description: "从N天前开始看", Placeholder: "0", Unit: "天", Group: "time_window"},
		},
	},
	// MACD 底背离
	{
		ID:          "macd_bull_divergence",
		Name:        "MACD底背离",
		Category:    model.CategoryTechnical,
		IndicatorID: "macd_divergence",
		DefaultCfg: func() model.Signal {
			return model.Signal{
				ID:          "tmpl_macd_bull_divergence",
				IndicatorID: "macd_divergence",
				Name:        "MACD底背离",
				Description: "股价创新低但MACD未创新低，潜在反转信号",
				Operator:    model.OpDivergencePos,
				LookbackDays: 26,
				Weight:      0.8,
				Order:       3,
				Enabled:     true,
			}
		},
		ParamDefs: []model.SignalParamDef{
			{Key: "lookback_days", Label: "回看天数", Type: model.ParamTypeDays, Default: 26, Min: 10, Max: 120, Step: 1,
				Description: "用于判断背离的历史数据长度。越长越准但越慢", Placeholder: "26", Example: "26=约半年周期; 60=约一季度", Unit: "天", Group: "depth"},
		},
	},
	// MACD 柱状图转正
	{
		ID:          "macd_hist_positive",
		Name:        "MACD红柱",
		Category:    model.CategoryTechnical,
		IndicatorID: "macd_hist",
		DefaultCfg: func() model.Signal {
			return model.Signal{
				ID:          "tmpl_macd_hist_positive",
				IndicatorID: "macd_hist",
				Name:        "MACD柱状图>0",
				Description: "MACD柱状图由负转正(红柱), 多头动能增强",
				Operator:    model.OpGT,
				ValueNumber: 0,
				Weight:      0.7,
				Order:       4,
				Enabled:     true,
			}
		},
		ParamDefs: []model.SignalParamDef{
			{Key: "value_number", Label: "阈值", Type: model.ParamTypeNumber, Default: 0.0, Min: -1, Max: 1, Step: 0.01,
				Description: "MACD柱状图的阈值。默认>0表示红柱(正柱)", Placeholder: "0", Example: "0=红柱; 0.1=强红柱", Unit: "", Group: "value_range"},
		},
	},

	// 均线多头排列
	{
		ID:          "ma_bullish_arrange",
		Name:        "均线多头排列",
		Category:    model.CategoryTechnical,
		IndicatorID: "ma_cross",
		DefaultCfg: func() model.Signal {
			return model.Signal{
				ID:          "tmpl_ma_bullish",
				IndicatorID: "ma_cross",
				Name:        "均线多头排列(MA5>MA10>MA20)",
				Description: "短中长期均线依次向上排列, 强势格局",
				Operator:    model.OpCrossUp,
				WithinDays:  3,
				Weight:      0.9,
				Order:       5,
				Enabled:     true,
			}
		},
		ParamDefs: []model.SignalParamDef{
			{Key: "within_days", Label: "持续天数", Type: model.ParamTypeDays, Default: 3, Min: 1, Max: 20, Step: 1,
				Description: "连续N天保持多头排列", Placeholder: "3", Example: "1=当天; 3=连续3天; 5=近一周", Unit: "天", Group: "time_window"},
		},
	},
	// 股价站上 MA20
	{
		ID:          "price_above_ma20",
		Name:        "股价站上20日线",
		Category:    model.CategoryTechnical,
		IndicatorID: "ma20",
		DefaultCfg: func() model.Signal {
			return model.Signal{
				ID:          "tmpl_price_above_ma20",
				IndicatorID: "ma20",
				Name:        "收盘价>MA20",
				Description: "收盘价站在20日均线上方, 中期趋势向好",
				Operator:    model.OpGT,
				ValueNumber: 0,
				Weight:      0.7,
				Order:       6,
				Enabled:     true,
			}
		},
	},

	// ===== 行情面 =====

	// 涨停
	{
		ID:          "market_limit_up",
		Name:        "涨停",
		Category:    model.CategoryMarket,
		IndicatorID: "limit_up",
		DefaultCfg: func() model.Signal {
			return model.Signal{
				ID:          "tmpl_limit_up",
				IndicatorID: "limit_up",
				Name:        "涨停",
				Description: "当日涨停或接近涨停(>=9.5%)",
				Operator:    model.OpGTE,
				ValueNumber: 9.5,
				Weight:      0.8,
				Order:       1,
				Enabled:     true,
			}
		},
		ParamDefs: []model.SignalParamDef{
			{Key: "value_number", Label: "涨幅阈值(%)", Type: model.ParamTypeNumber, Default: 9.5, Min: 5, Max: 20, Step: 0.1,
				Description: "涨跌幅达到此值视为触发。主板涨停=20%(科创板); 9.5%≈涨停", Placeholder: "9.5", Example: "9.5=近涨停; 5=大涨; 20=科创板涨停", Unit: "%", Group: "value_range"},
		},
	},
	// 高换手
	{
		ID:          "market_high_turnover",
		Name:        "高换手",
		Category:    model.CategoryMarket,
		IndicatorID: "turnover_rate",
		DefaultCfg: func() model.Signal {
			return model.Signal{
				ID:          "tmpl_high_turnover",
				IndicatorID: "turnover_rate",
				Name:        "高换手(>5%)",
				Description: "换手率超过5%, 交投活跃",
				Operator:    model.OpGT,
				ValueNumber: 5,
				Weight:      0.5,
				Order:       3,
				Enabled:     true,
			}
		},
		ParamDefs: []model.SignalParamDef{
			{Key: "value_number", Label: "换手率阈值(%)", Type: model.ParamTypeNumber, Default: float64(5), Min: 1, Max: 50, Step: 0.5,
				Description: "换手率超过此值视为高活跃度", Placeholder: "5", Example: "3=活跃; 5=高换手; 10=极高换手", Unit: "%", Group: "value_range"},
		},
	},
	// 低价股
	{
		ID:          "market_low_price",
		Name:        "低价股",
		Category:    model.CategoryMarket,
		IndicatorID: "price",
		DefaultCfg: func() model.Signal {
			return model.Signal{
				ID:          "tmpl_low_price",
				IndicatorID: "price",
				Name:        "低价股(<=10元)",
				Description: "收盘价不超过10元",
				Operator:    model.OpLTE,
				ValueNumber: 10,
				Weight:      0.5,
				Order:       10,
				Enabled:     true,
			}
		},
		ParamDefs: []model.SignalParamDef{
			{Key: "value_number", Label: "价格上限(元)", Type: model.ParamTypeNumber, Default: float64(10), Min: 1, Max: 100, Step: 0.5,
				Description: "收盘价不超过此值视为低价股", Placeholder: "10", Example: "5=低价; 10=中低价; 30=中等价位", Unit: "元", Group: "value_range"},
		},
	},
	// 小市值
	{
		ID:          "market_small_cap",
		Name:        "小市值",
		Category:    model.CategoryMarket,
		IndicatorID: "circulate_market_cap",
		DefaultCfg: func() model.Signal {
			return model.Signal{
				ID:          "tmpl_small_cap",
				IndicatorID: "circulate_market_cap",
				Name:        "小市值(<50亿)",
				Description: "流通市值小于50亿, 弹性较好",
				Operator:    model.OpLT,
				ValueNumber: 50,
				Weight:      0.5,
				Order:       11,
				Enabled:     true,
			}
		},
		ParamDefs: []model.SignalParamDef{
			{Key: "value_number", Label: "市值上限(亿)", Type: model.ParamTypeNumber, Default: float64(50), Min: 5, Max: 500, Step: 5,
				Description: "流通市值不超过此值", Placeholder: "50", Example: "20=微盘; 50=小盘; 200=中盘", Unit: "亿", Group: "value_range"},
		},
	},

	// ===== 基本面 =====

	// 创业板/科创板
	{
		ID:          "fund_chinext_or_star",
		Name:        "创业板/科创板",
		Category:    model.CategoryFundamental,
		IndicatorID: "listing_board",
		DefaultCfg: func() model.Signal {
			return model.Signal{
				ID:          "tmpl_chinext_star",
				IndicatorID: "listing_board",
				Name:        "创业板或科创板",
				Description: "上市板块为创业板或科创板",
				Operator:    model.OpIn,
				ValueList:   []string{"chinext", "star"},
				Weight:      0.5,
				Order:       20,
				Enabled:     true,
			}
		},
	},

	// ===== 财务面 =====

	// PE 合理区间（★ 用户核心需求：支持自定义 25-50 等任意区间）
	{
		ID:          "fin_pe_reasonable",
		Name:        "PE区间",
		Category:    model.CategoryFinancial,
		IndicatorID: "pe_ttm",
		DefaultCfg: func() model.Signal {
			return model.Signal{
				ID:          "tmpl_pe_reasonable",
				IndicatorID: "pe_ttm",
				Name:        "市盈率区间(0~30倍)",
				Description: "TTM市盈率在指定范围内, 排除亏损和高估",
				Operator:    model.OpBetween,
				MinValue:    0,
				MaxValue:    30,
				Weight:      0.7,
				Order:       15,
				Enabled:     true,
			}
		},
		ParamDefs: []model.SignalParamDef{
			{Key: "min_value", Label: "PE下界(倍)", Type: model.ParamTypeRange, Default: 0.0, Min: -50, Max: 200, Step: 1,
				Description: "市盈率下限。负数表示亏损，0 表示排除亏损股", Placeholder: "0", Example: "0=排除亏损; 10=低估值; 25=合理偏低", Unit: "倍", Group: "value_range", ConditionValue: "between"},
			{Key: "max_value", Label: "PE上界(倍)", Type: model.ParamTypeRange, Default: float64(30), Min: 0, Max: 500, Step: 1,
				Description: "市盈率上限。超过此值认为高估", Placeholder: "30", Example: "20=低估; 30=合理; 50=可接受; 100=高成长容忍", Unit: "倍", Group: "value_range"},
		},
	},
	// 高 ROE
	{
		ID:          "fin_high_roe",
		Name:        "高ROE",
		Category:    model.CategoryFinancial,
		IndicatorID: "roe_w",
		DefaultCfg: func() model.Signal {
			return model.Signal{
				ID:          "tmpl_high_roe",
				IndicatorID: "roe_w",
				Name:        "高ROE(>=15%)",
				Description: "净资产收益率>=15%, 优质公司特征",
				Operator:    model.OpGTE,
				ValueNumber: 15,
				Weight:      0.9,
				Order:       14,
				Enabled:     true,
			}
		},
		ParamDefs: []model.SignalParamDef{
			{Key: "value_number", Label: "ROE下界(%)", Type: model.ParamTypeThreshold, Default: float64(15), Min: 0, Max: 50, Step: 1,
				Description: "加权净资产收益率不低于此值。巴菲特偏好>15%", Placeholder: "15", Example: "10=一般; 15=优秀; 20=卓越; 30=极强", Unit: "%", Group: "threshold"},
		},
	},
	// 盈利公司 (EPS>0)
	{
		ID:          "fin_profitable",
		Name:        "盈利(EPS>0)",
		Category:    model.CategoryFinancial,
		IndicatorID: "eps",
		DefaultCfg: func() model.Signal {
			return model.Signal{
				ID:          "tmpl_profitable",
				IndicatorID: "eps",
				Name:        "盈利公司(EPS>0)",
				Description: "基本每股收益大于0, 排除亏损企业",
				Operator:    model.OpGT,
				ValueNumber: 0,
				Weight:      1.0,
				Order:       13,
				Enabled:     true,
			}
		},
		ParamDefs: []model.SignalParamDef{
			{Key: "value_number", Label: "EPS下界(元)", Type: model.ParamTypeNumber, Default: 0.0, Min: -5, Max: 10, Step: 0.01,
				Description: "基本每股收益阈值。>0 表示排除亏损", Placeholder: "0", Example: "0=非亏损; 0.5=微利; 1=盈利; 3=高利润", Unit: "元", Group: "value_range"},
		},
	},
	// 高成长
	{
		ID:          "fin_high_growth",
		Name:        "高成长",
		Category:    model.CategoryFinancial,
		IndicatorID: "revenue_yoy",
		DefaultCfg: func() model.Signal {
			return model.Signal{
				ID:          "tmpl_high_growth",
				IndicatorID: "revenue_yoy",
				Name:        "高成长(营收同比>20%)",
				Description: "营业总收入同比增长超过20%",
				Operator:    model.OpGT,
				ValueNumber: 20,
				Weight:      0.9,
				Order:       15,
				Enabled:     true,
			}
		},
		ParamDefs: []model.SignalParamDef{
			{Key: "value_number", Label: "营收增速下界(%)", Type: model.ParamTypeNumber, Default: float64(20), Min: -50, Max: 200, Step: 1,
				Description: "营业总收入同比增速不低于此值", Placeholder: "20", Example: "10=稳步增长; 20=高速增长; 50=爆发式增长", Unit: "%", Group: "value_range"},
		},
	},
	// 低负债
	{
		ID:          "fin_low_debt",
		Name:        "低负债",
		Category:    model.CategoryFinancial,
		IndicatorID: "debt_ratio",
		DefaultCfg: func() model.Signal {
			return model.Signal{
				ID:          "tmpl_low_debt",
				IndicatorID: "debt_ratio",
				Name:        "低负债(负债率<50%)",
				Description: "资产负债率低于50%，财务稳健",
				Operator:    model.OpLT,
				ValueNumber: 50,
				Weight:      0.7,
				Order:       16,
				Enabled:     true,
			}
		},
		ParamDefs: []model.SignalParamDef{
			{Key: "value_number", Label: "负债率上限(%)", Type: model.ParamTypeThreshold, Default: float64(50), Min: 10, Max: 100, Step: 1,
				Description: "资产负债率不超过此值。一般认为<60%较安全", Placeholder: "50", Example: "30=很安全; 50=健康; 70=偏高; 80=高风险", Unit: "%", Group: "threshold"},
		},
	},
	// 正经营现金流
	{
		ID:          "fin_positive_ocf",
		Name:        "正经营现金流",
		Category:    model.CategoryFinancial,
		IndicatorID: "ocfps",
		DefaultCfg: func() model.Signal {
			return model.Signal{
				ID:          "tmpl_positive_ocf",
				IndicatorID: "ocfps",
				Name:        "正经营现金流(OCF>0)",
				Description: "每股经营现金流转正，真金白银赚钱",
				Operator:    model.OpGT,
				ValueNumber: 0,
				Weight:      0.8,
				Order:       17,
				Enabled:     true,
			}
		},
	},
}

// ============================================================================
//  预设策略模板 (Strategy Templates)
// ============================================================================

// PresetStrategy 预设策略模板
type PresetStrategy struct {
	ID         string            // 模板 ID
	Name       string            // 策略名称
	Desc       string            // 描述
	Signals    []string // 引用的预设信号 ID 列表
	LogicalOp  model.LogicalOp   // 逻辑关系
}

// 预设策略模板列表（常用组合策略）
var BuiltInStrategies = []PresetStrategy{
	{
		ID:        "strat_tech_momentum",
		Name:      "技术面强势策略",
		Desc:      "捕捉强势股：MACD金叉 + 均线多头排列",
		Signals:   []string{"macd_golden_cross", "ma_bullish_arrange"},
		LogicalOp: model.LogicalAND,
	},
	{
		ID:        "strat_value_growth",
		Name:      "价值成长策略",
		Desc:      "寻找低估的高成长股：盈利+高ROE+高成长+低PE+低负债",
		Signals:   []string{"fin_profitable", "fin_high_roe", "fin_high_growth", "fin_pe_reasonable", "fin_low_debt"},
		LogicalOp: model.LogicalAND,
	},
	{
		ID:        "strat_small_hot",
		Name:      "小盘热点策略",
		Desc:      "小市值+高活跃度：小市值+放量+高换手+涨停附近",
		Signals:   []string{"market_small_cap", "market_high_volume", "market_high_turnover", "market_limit_up"},
		LogicalOp: model.LogicalAND,
	},
	{
		ID:        "strat_quality_stock",
		Name:      "优质白马策略",
		Desc:      "基本面过硬的白马股：盈利+高ROE+正现金流+低负债+合理估值",
		Signals:   []string{"fin_profitable", "fin_high_roe", "fin_positive_ocf", "fin_low_debt", "fin_pe_reasonable"},
		LogicalOp: model.LogicalAND,
	},
}

// ============================================================================
//  注册函数 — 将所有内置指标和评估器注入 Engine
// ============================================================================

// RegisterBuiltInIndicators 注册所有内置指标到引擎中
func RegisterBuiltInIndicators(e *Engine) {

	// ========== 数值型通用比较（使用 NumberEvaluatorFactory）==========
	numberIndicators := map[string]string{
		// 字段名 → 单位
		"ma5": "", "ma10": "", "ma20": "", "ma60": "",
		"macd": "", "macd_signal": "", "macd_hist": "",
		"pe_ttm": "倍",
		"price": "元", "change_pct": "%", "amplitude": "%",
		"turnover_rate": "%", "volume": "手",
		"total_market_cap": "亿", "circulate_market_cap": "亿",
	}
	for id, unit := range numberIndicators {
		e.RegisterEvaluator(id, NumberEvaluatorFactory(id, unit))
	}

	// ========== 财报型通用比较 ==========
	financialIndicators := map[string]string{
		"roe_w": "%", "roa": "%", "gross_margin": "%",
		"net_margin": "%", "revenue_yoy": "%",
		"profit_yoy": "%", // 注意字段映射
		"debt_ratio": "%", "current_ratio": "倍",
		"quick_ratio": "倍", "eps": "元", "bvps": "元",
		"ocfps": "元",
	}
	for id, unit := range financialIndicators {
		e.RegisterEvaluator(id, FinancialEvaluatorFactory(id, unit))
	}
	// 修正 profit_yoy 的实际字段名
	e.RegisterEvaluator("profit_yoy", FinancialEvaluatorFactory("parent_net_profit_yoy", "%"))

	// ========== 枚举型比较 ==========
	enumIndicators := []string{"listing_board", "industry"}
	for _, id := range enumIndicators {
		e.RegisterEvaluator(id, EnumEvaluatorFactory(id))
	}

	// ========== 序列型专用评估器（需要历史数据计算）==========

	// MACD
	e.RegisterEvaluator("macd_cross",      indicators.EvaluateMACDCross)
	e.RegisterEvaluator("macd_divergence", indicators.EvaluateMACDDivergence)

	// 均线
	e.RegisterEvaluator("ma_cross",        indicators.EvaluateMACross)

	log.Printf("[screener] 内置指标注册完成: %d 个指标", len(BuiltInIndicators))
}

// GetIndicatorByID 按 ID 获取指标定义
func GetIndicatorByID(id string) (*model.Indicator, bool) {
	for i := range BuiltInIndicators {
		if BuiltInIndicators[i].ID == id {
			return &BuiltInIndicators[i], true
		}
	}
	return nil, false
}

// GetPresetSignalByID 获取预设信号模板
func GetPresetSignalByID(id string) (*PresetSignal, bool) {
	for i := range PresetSignals {
		if PresetSignals[i].ID == id {
			return &PresetSignals[i], true
		}
	}
	return nil, false
}

// ListIndicatorsByCategory 按类别列出所有内置指标
func ListIndicatorsByCategory() map[model.IndicatorCategory][]model.Indicator {
	result := make(map[model.IndicatorCategory][]model.Indicator)
	for _, ind := range BuiltInIndicators {
		result[ind.Category] = append(result[ind.Category], ind)
	}
	// 每个类别内按 ID 排序
	for cat := range result {
		sort.Slice(result[cat], func(i, j int) bool {
			return result[cat][i].ID < result[cat][j].ID
		})
	}
	return result
}

// ListPresetsByCategory 按类别列出所有预设信号
func ListPresetsByCategory() map[model.IndicatorCategory][]PresetSignal {
	result := make(map[model.IndicatorCategory][]PresetSignal)
	for i := range PresetSignals {
		sig := PresetSignals[i]
		result[sig.Category] = append(result[sig.Category], sig)
	}
	return result
}

// ============================================================================
//  自由组合模式 — 指标→操作符→参数 的动态组装接口
//
//  设计目标：
//    1. 前端展示指标列表 → 用户选择一个指标（如 "macd_cross"）
//    2. 调用 GetIndicatorOperators() → 返回该指标支持的所有操作符+参数表单
//    3. 用户选择操作符 + 填入参数
//    4. 调用 BuildSignal() → 生成合法的 Signal 对象
//    5. 将 Signal 加入 Strategy.Conditions，调用 Engine.Execute()
// ============================================================================

// IndicatorWithOperators 带操作符选项的指标信息（前端渲染条件选择器所需）
type IndicatorWithOperators struct {
	model.Indicator                // 嵌入指标元数据
	Operators []operators.OperatorOption `json:"operators"` // 该指标支持的所有操作符（由各指标文件定义）
}

// GetIndicatorOperators 获取指定指标支持的所有操作符及其参数表单定义
//
// 查找顺序：
//  1. 先查 indicators 注册表（由各指标文件的 init() 显式注册）
//  2. 若未注册，根据 ValueType 回退到默认操作符集合（兼容尚未迁移的指标）
//
// 这是自由组合模式的核心 API：前端根据返回值动态渲染操作符下拉 + 参数输入控件
func GetIndicatorOperators(indicatorID string) (*IndicatorWithOperators, bool) {
	ind, ok := GetIndicatorByID(indicatorID)
	if !ok {
		return nil, false
	}

	// 优先取指标文件显式注册的操作符
	opts, registered := indicators.GetRegisteredOperators(indicatorID)
	if !registered {
		// 回退：根据 ValueType 使用默认操作符（兼容尚未迁移到 indicators 包的指标）
		opts = defaultOpsForType(ind.ValueType)
	}

	return &IndicatorWithOperators{
		Indicator: *ind,
		Operators: opts,
	}, true
}

// GetAllIndicatorsWithOperators 获取所有指标及其支持的操作符
// 用于前端一次性加载完整的指标树（分类 → 指标 → 操作符 → 参数表单）
func GetAllIndicatorsWithOperators() map[model.IndicatorCategory][]IndicatorWithOperators {
	result := make(map[model.IndicatorCategory][]IndicatorWithOperators)
	for _, ind := range BuiltInIndicators {
		ops, registered := indicators.GetRegisteredOperators(ind.ID)
		if !registered {
			ops = defaultOpsForType(ind.ValueType)
		}
		wrapped := IndicatorWithOperators{
			Indicator: ind,
			Operators: ops,
		}
		result[ind.Category] = append(result[ind.Category], wrapped)
	}
	return result
}

// defaultOpsForType 根据 ValueType 返回默认操作符（回退方案，用于尚未在 indicators 包注册的指标）
func defaultOpsForType(valueType model.ValueType) []operators.OperatorOption {
	switch valueType {
	case model.ValueTypeNumber:
		return operators.NumberOps()
	case model.ValueTypeBool:
		return operators.BoolOps()
	case model.ValueTypeEnum:
		return operators.EnumOps()
	case model.ValueTypeSeries:
		return operators.SeriesOps()
	default:
		return []operators.OperatorOption{}
	}
}

// BuildSignal 根据指标 ID、操作符和用户参数构建一个合法的 Signal
//
// 使用方式：
//   signal, err := screener.BuildSignal("pe_ttm", model.OpBetween, map[string]interface{}{
//       "min_value": 20.0,
//       "max_value": 50.0,
//   })
//   // → Signal{IndicatorID:"pe_ttm", Operator:OpBetween, MinValue:20, MaxValue:50, ...}
//
// 该函数会：
//   1. 验证指标是否存在、操作符是否被该指标类型支持
//   2. 根据操作符从 params 中提取并校验参数值
//   3. 自动填充默认值（用户未提供的可选参数）
//   4. 返回可直接传给 Engine.Execute() 的完整 Signal
func BuildSignal(indicatorID string, op model.CompareOperator, params map[string]interface{}) (*model.Signal, error) {
	// Step 1: 查找指标元数据
	ind, ok := GetIndicatorByID(indicatorID)
	if !ok {
		return nil, fmt.Errorf("指标不存在: %s", indicatorID)
	}

	// Step 2: 验证操作符是否被该指标支持
	supportedOps, registered := indicators.GetRegisteredOperators(indicatorID)
	if !registered {
		supportedOps = defaultOpsForType(ind.ValueType)
	}
	var found *operators.OperatorOption
	for i := range supportedOps {
		if supportedOps[i].Operator == op {
			found = &supportedOps[i]
			break
		}
	}
	if found == nil {
		return nil, fmt.Errorf("操作符 %q 不被指标 %q (类型=%s) 支持", op, indicatorID, ind.ValueType)
	}

	// Step 3: 构建 Signal
	signal := &model.Signal{
		ID:          fmt.Sprintf("custom_%s_%s_%d", indicatorID, op, time.Now().Unix()),
		IndicatorID: indicatorID,
		Name:        ind.Name,
		Description: fmt.Sprintf("%s %s %s", ind.Name, found.Symbol, found.Example),
		Category:    ind.Category,
		Operator:    op,
		Weight:      1.0,
		Enabled:     true,
	}

	// Step 4: 根据操作符提取参数
	switch op {
	case model.OpGT, model.OpGTE, model.OpLT, model.OpLTE:
		// 单值比较：需要 value_number
		val, okVal := getFloatParam(params, "value_number")
		if !okVal {
			return nil, fmt.Errorf("%s 操作需要 value_number 参数 (如 PE > ?)", found.Label)
		}
		signal.ValueNumber = val

	case model.OpEQ, model.OpNEQ:
		// 等于/不等于：数值型用 value_number，枚举型用 value_list
		if ind.ValueType == model.ValueTypeEnum || ind.ValueType == model.ValueTypeBool {
			list, okList := getStringSliceParam(params, "value_list")
			if !okList || len(list) == 0 {
				return nil, fmt.Errorf("%s 操作需要 value_list 参数", found.Label)
			}
			signal.ValueList = list
		} else {
			val, okVal := getFloatParam(params, "value_number")
			if !okVal {
				return nil, fmt.Errorf("%s 操作需要 value_number 参数", found.Label)
			}
			signal.ValueNumber = val
		}

	case model.OpBetween, model.OpNotBetween:
		// 区间比较：需要 min_value 和 max_value
		minV, okMin := getFloatParam(params, "min_value")
		maxV, okMax := getFloatParam(params, "max_value")
		if !okMin || !okMax {
			return nil, fmt.Errorf("%s 操作需要 min_value 和 max_value 参数", found.Label)
		}
		if minV >= maxV {
			return nil, fmt.Errorf("区间下界(%.2f) 必须小于上界(%.2f)", minV, maxV)
		}
		signal.MinValue = minV
		signal.MaxValue = maxV

	case model.OpIn, model.OpNotIn, model.OpContains:
		// 枚举集合：需要 value_list
		list, okList := getStringSliceParam(params, "value_list")
		if !okList || len(list) == 0 {
			return nil, fmt.Errorf("%s 操作需要 value_list 参数", found.Label)
		}
		signal.ValueList = list

	case model.OpCrossUp, model.OpCrossDown, model.OpBreakout, model.OpBreakdown:
		// 序列形态：时间窗口参数（可选，有默认值）
		if wd, ok := getIntParam(params, "within_days"); ok {
			signal.WithinDays = wd
		} else if len(found.Params) > 0 && found.Params[0].Default != nil {
			if d, okInt := toInt(found.Params[0].Default); okInt {
				signal.WithinDays = d
			}
		}
		if ago, ok := getIntParam(params, "ago_from_days"); ok {
			signal.AgoFromDays = ago
		}

	case model.OpDivergencePos, model.OpDivergenceNeg:
		// 背离：回看深度参数（可选）
		if lb, ok := getIntParam(params, "lookback_days"); ok {
			signal.LookbackDays = lb
		} else {
			signal.LookbackDays = 26 // 默认约半年周期
		}
	}

	return signal, nil
}

// --- BuildSignal 辅助函数 ---

func getFloatParam(params map[string]interface{}, key string) (float64, bool) {
	v, ok := params[key]
	if !ok {
		return 0, false
	}
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case json.Number:
		f, err := val.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func getIntParam(params map[string]interface{}, key string) (int, bool) {
	v, ok := params[key]
	if !ok {
		return 0, false
	}
	switch val := v.(type) {
	case int:
		return val, true
	case int64:
		return int(val), true
	case float64:
		return int(val), true
	case json.Number:
		i, err := val.Int64()
		return int(i), err == nil
	default:
		return 0, false
	}
}

func getStringSliceParam(params map[string]interface{}, key string) ([]string, bool) {
	v, ok := params[key]
	if !ok {
		return nil, false
	}
	switch val := v.(type) {
	case []string:
		return val, true
	case []interface{}:
		result := make([]string, len(val))
		for i, item := range val {
			if s, ok := item.(string); ok {
				result[i] = s
			} else {
				result[i] = fmt.Sprintf("%v", item)
			}
		}
		return result, true
	default:
		// 单字符串也接受，转为单元素切片
		if s, ok := v.(string); ok && s != "" {
			return []string{s}, true
		}
		return nil, false
	}
}

func toInt(v interface{}) (int, bool) {
	switch val := v.(type) {
	case int:
		return val, true
	case int64:
		return int(val), true
	case float64:
		return int(val), true
	case json.Number:
		i, err := val.Int64()
		return int(i), err == nil
	default:
		return 0, false
	}
}

// 注：序列型评估器的具体实现已迁移到 indicators 包（同 package）
//   - MACD  金叉/死叉/背离 → indicators/macd.go (EvaluateMACDCross / EvaluateMACDDivergence)
//   - MA    均线排列       → indicators/ma.go   (EvaluateMACross)
//
// 通用工具函数（min/max/abs/timeNow/passMark/findValleys 等）→ indicators/evaluator.go
