package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"stock-ai/internal/db"
	"stock-ai/internal/model"
	"stock-ai/internal/subscription/quotecache"
	"stock-ai/internal/subscription/watchlist"
	"stock-ai/utils"
)

// PortfolioService 持仓管理业务逻辑
type PortfolioService struct {
	cache                  quotecache.QuoteCache // 可选：为 nil 时不填充现价/盈亏
	watchlistMgr           *watchlist.Manager    // 可选：为 nil 时不同步关注列表
	notifyHoldingChangedFn func()                // 可选：持仓股变动时通知 Monitor 重算 ScopeHeld
}

// NewPortfolioService 创建持仓服务实例
func NewPortfolioService() *PortfolioService {
	return &PortfolioService{}
}

// SetQuoteCache 设置行情缓存（可选，main.go 启动后注入）
func (svc *PortfolioService) SetQuoteCache(cache quotecache.QuoteCache) {
	svc.cache = cache
}

// SetWatchlistManager 设置关注列表管理器（可选，main.go 启动后注入）
func (svc *PortfolioService) SetWatchlistManager(mgr *watchlist.Manager) {
	svc.watchlistMgr = mgr
}

// SetNotifyHoldingChanged 设置持仓股变动回调（main.go 注入 Monitor.NotifyHoldingChanged）
func (svc *PortfolioService) SetNotifyHoldingChanged(fn func()) {
	svc.notifyHoldingChangedFn = fn
}

// notifyHoldingChanged 安全调用持仓变动回调
func (svc *PortfolioService) notifyHoldingChanged() {
	if svc.notifyHoldingChangedFn != nil {
		svc.notifyHoldingChangedFn()
	}
}

// ============================================================================
//  请求/响应结构体
// ============================================================================

// OpenPositionReq 建仓请求
type OpenPositionReq struct {
	StockCode string  `json:"stock_code" binding:"required,len=6"` // 股票代码(6位数字)
	Quantity  int     `json:"quantity" binding:"required,gt=0"`    // 买入数量(股)
	Price     float64 `json:"price" binding:"required,gt=0"`       // 成交价格
	TradeDate string  `json:"trade_date" binding:"required"`       // 交易日期 YYYY-MM-DD
	Note      string  `json:"note"`                                // 备注
}

// TradeReq 加仓/减仓请求（共用）
type TradeReq struct {
	Quantity  int     `json:"quantity" binding:"required,gt=0"` // 交易数量(股)
	Price     float64 `json:"price" binding:"required,gt=0"`    // 成交价格
	TradeDate string  `json:"trade_date" binding:"required"`    // 交易日期 YYYY-MM-DD
	Note      string  `json:"note"`                             // 备注
}

// UpdatePositionReq 更新持仓备注请求
type UpdatePositionReq struct {
	Note string `json:"note"`
}

// TradeConfigResp 交易配置响应
type TradeConfigResp struct {
	CommissionRate float64 `json:"commission_rate"` // 手续费率 (如 0.00025)
	MinCommission  bool    `json:"min_commission"`  // 是否不免五 (true=有最低5元)
}

// PositionListResp 持仓列表响应
type PositionListResp struct {
	List    []model.PositionDetail `json:"list"`
	Total   int64                  `json:"total"`
	Page    int                    `json:"page"`
	Size    int                    `json:"size"`
	Summary PositionSummary        `json:"summary"` // 统计概览
}

// PositionSummary 持仓统计概览
type PositionSummary struct {
	HoldingCount  int     `json:"holding_count"`  // 持仓数量
	ClosedCount   int     `json:"closed_count"`   // 已清仓数量
	TotalCost     float64 `json:"total_cost"`     // 总投入成本
	TotalQuantity int     `json:"total_quantity"` // 总持仓股数
}

// TradeConfigUpdateReq 更新交易配置请求
type TradeConfigUpdateReq struct {
	CommissionRate float64 `json:"commission_rate" binding:"required,gt=0,lte=10"` // 手续费率(单位:万分之x，如2.5表示万分之2.5)
	MinCommission  bool    `json:"min_commission"`                                 // 是否免五
}

// ============================================================================
//  核心业务方法
// ============================================================================

// OpenPosition 建仓：创建持仓记录 + 首笔买入交易
func (svc *PortfolioService) OpenPosition(req *OpenPositionReq, uid uint) (*model.PositionDetail, error) {
	// 1. 检查该股票是否已有 active 持仓
	existing, _ := db.GetPositionByStockCodeAndUID(req.StockCode, uid)
	if existing != nil && existing.Status == string(model.PositionHolding) {
		return nil, errors.New("该股票已有持仓，请使用加仓")
	}

	// 2. 解析交易日期
	tradeDate, err := time.Parse("2006-01-02", req.TradeDate)
	if err != nil {
		return nil, errors.New("交易日期格式错误，应为 YYYY-MM-DD")
	}

	amount := float64(req.Quantity) * req.Price

	// 3. 创建持仓记录
	position := &model.Position{
		UID:       uid,
		StockCode: req.StockCode,
		Quantity:  req.Quantity,
		AvgCost:   req.Price,
		Status:    string(model.PositionHolding),
		Note:      req.Note,
	}
	if err := db.CreatePosition(position); err != nil {
		return nil, fmt.Errorf("创建持仓失败: %w", err)
	}

	// 建仓后同步到关注列表
	if svc.watchlistMgr != nil {
		svc.watchlistMgr.OnPositionOpened(uid,req.StockCode)
		svc.notifyHoldingChanged()
	}

	// 4. 计算手续费并创建交易记录
	commission := svc.calculateCommission(uid, amount)
	trade := &model.PositionTrade{
		PositionID: position.ID,
		UID:        uid,
		TradeType:  int8(model.TradeTypeBuy),
		Quantity:   req.Quantity,
		Price:      req.Price,
		Amount:     amount,
		Commission: commission,
		TradeDate:  tradeDate,
		Note:       req.Note,
	}
	if err := db.CreateTrade(trade); err != nil {
		return nil, fmt.Errorf("创建交易记录失败: %w", err)
	}

	// 5. 组装返回详情
	detail := position.ToPositionDetail()
	svc.enrichPositionDetail(detail, uid)
	detail.Trades = []model.PositionTradeDetail{*trade.ToDetail()}

	return detail, nil
}

// BuyMore 加仓：新增买入记录 + 重算平均成本
func (svc *PortfolioService) BuyMore(id uint, req *TradeReq, uid uint) (*model.PositionDetail, error) {
	position, err := db.GetPositionByIDAndUID(id, uid)
	if err != nil {
		return nil, errors.New("持仓不存在")
	}
	if position.Status == string(model.PositionClosed) {
		return nil, errors.New("该持仓已清仓，无法加仓")
	}

	tradeDate, err := time.Parse("2006-01-02", req.TradeDate)
	if err != nil {
		return nil, errors.New("交易日期格式错误，应为 YYYY-MM-DD")
	}

	amount := float64(req.Quantity) * req.Price

	// 重算加权平均成本
	totalCost := float64(position.Quantity)*position.AvgCost + amount
	newQty := position.Quantity + req.Quantity
	newAvgCost := totalCost / float64(newQty)
	// 四舍五入保留4位
	newAvgCost = math.Round(newAvgCost*10000) / 10000

	// 更新持仓
	position.Quantity = newQty
	position.AvgCost = newAvgCost
	if err := db.UpdatePosition(position); err != nil {
		return nil, fmt.Errorf("更新持仓失败: %w", err)
	}

	// 加仓后同步到关注列表
	if svc.watchlistMgr != nil {
		svc.watchlistMgr.OnPositionOpened(uid,position.StockCode)
	}

	// 创建买入交易记录
	commission := svc.calculateCommission(uid, amount)
	trade := &model.PositionTrade{
		PositionID: id,
		UID:        uid,
		TradeType:  int8(model.TradeTypeBuy),
		Quantity:   req.Quantity,
		Price:      req.Price,
		Amount:     amount,
		Commission: commission,
		TradeDate:  tradeDate,
		Note:       req.Note,
	}
	if err := db.CreateTrade(trade); err != nil {
		return nil, fmt.Errorf("创建交易记录失败: %w", err)
	}

	detail := position.ToPositionDetail()
	svc.enrichPositionDetail(detail, uid)
	return detail, nil
}

// SellPartial 减仓：新增卖出记录 + 扣减持仓数量
func (svc *PortfolioService) SellPartial(id uint, req *TradeReq, uid uint) (*model.PositionDetail, error) {
	position, err := db.GetPositionByIDAndUID(id, uid)
	if err != nil {
		return nil, errors.New("持仓不存在")
	}
	if position.Status == string(model.PositionClosed) {
		return nil, errors.New("该持仓已清仓，无法减仓")
	}
	if req.Quantity >= position.Quantity {
		return nil, fmt.Errorf("减仓数量(%d)必须小于当前持仓数(%d)，如需全部卖出请使用清仓", req.Quantity, position.Quantity)
	}

	tradeDate, err := time.Parse("2006-01-02", req.TradeDate)
	if err != nil {
		return nil, errors.New("交易日期格式错误，应为 YYYY-MM-DD")
	}

	amount := float64(req.Quantity) * req.Price

	// 减少持仓数量（avg_cost 不变）
	position.Quantity -= req.Quantity
	if err := db.UpdatePosition(position); err != nil {
		return nil, fmt.Errorf("更新持仓失败: %w", err)
	}

	// 创建卖出交易记录
	commission := svc.calculateCommission(uid, amount)
	trade := &model.PositionTrade{
		PositionID: id,
		UID:        uid,
		TradeType:  int8(model.TradeTypeSell),
		Quantity:   req.Quantity,
		Price:      req.Price,
		Amount:     amount,
		Commission: commission,
		TradeDate:  tradeDate,
		Note:       req.Note,
	}
	if err := db.CreateTrade(trade); err != nil {
		return nil, fmt.Errorf("创建交易记录失败: %w", err)
	}

	detail := position.ToPositionDetail()
	svc.enrichPositionDetail(detail, uid)
	return detail, nil
}

// ClosePosition 清仓：全部卖出 + 标记为 closed
func (svc *PortfolioService) ClosePosition(id uint, price float64, tradeDateStr, note string, uid uint) (*model.PositionDetail, error) {
	position, err := db.GetPositionByIDAndUID(id, uid)
	if err != nil {
		return nil, errors.New("持仓不存在")
	}
	if position.Status == string(model.PositionClosed) {
		return nil, errors.New("该持仓已清仓")
	}
	if position.Quantity <= 0 {
		return nil, errors.New("当前无持仓数量，无需清仓")
	}

	tradeDate, err := time.Parse("2006-01-02", tradeDateStr)
	if err != nil {
		return nil, errors.New("交易日期格式错误，应为 YYYY-MM-DD")
	}

	qty := position.Quantity
	amount := float64(qty) * price

	// 更新持仓状态
	position.Quantity = 0
	position.Status = string(model.PositionClosed)
	if err := db.UpdatePosition(position); err != nil {
		return nil, fmt.Errorf("更新持仓失败: %w", err)
	}

	// 清仓后同步到关注列表（内部检查是否还有人持有）
	if svc.watchlistMgr != nil {
		svc.watchlistMgr.OnPositionClosed(uid, position.StockCode)
		svc.notifyHoldingChanged()
	}

	// 创建清仓卖出记录
	commission := svc.calculateCommission(uid, amount)
	trade := &model.PositionTrade{
		PositionID: id,
		UID:        uid,
		TradeType:  int8(model.TradeTypeSell),
		Quantity:   qty,
		Price:      price,
		Amount:     amount,
		Commission: commission,
		TradeDate:  tradeDate,
		Note:       note,
	}
	if err := db.CreateTrade(trade); err != nil {
		return nil, fmt.Errorf("创建交易记录失败: %w", err)
	}

	detail := position.ToPositionDetail()
	svc.enrichPositionDetail(detail, uid)
	return detail, nil
}

// GetPositionByID 获取持仓详情（含交易记录）
func (svc *PortfolioService) GetPositionByID(id, uid uint) (*model.PositionDetail, error) {
	position, err := db.GetPositionByIDAndUID(id, uid)
	if err != nil {
		return nil, errors.New("持仓不存在或无权访问")
	}

	detail := position.ToPositionDetail()
	svc.enrichPositionDetail(detail, uid)

	// 加载交易记录
	trades, _ := db.ListTradesByPositionID(id)
	tradeDetails := make([]model.PositionTradeDetail, 0, len(trades))
	for _, t := range trades {
		tradeDetails = append(tradeDetails, *t.ToDetail())
	}
	detail.Trades = tradeDetails
	detail.TradeCount = len(tradeDetails)

	return detail, nil
}

// ListPositions 查询持仓列表（含统计概览）
func (svc *PortfolioService) ListPositions(uid uint, statusFilter string, page, pageSize int) (*PositionListResp, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	positions, total, err := db.ListPositions(uid, statusFilter, page, pageSize)
	if err != nil {
		return nil, err
	}

	// 组装详情 + 关联股票名称
	details := make([]model.PositionDetail, 0, len(positions))
	var summary PositionSummary
	for _, p := range positions {
		d := p.ToPositionDetail()
		svc.enrichPositionDetail(d, uid)
		details = append(details, *d)

		// 累计统计
		if p.Status == string(model.PositionHolding) {
			summary.HoldingCount++
			summary.TotalQuantity += p.Quantity
			summary.TotalCost += p.AvgCost * float64(p.Quantity)
		} else if p.Status == string(model.PositionClosed) {
			summary.ClosedCount++
			summary.TotalCost += p.AvgCost * float64(p.Quantity) // 已清仓也计入总投入
		}
	}

	// 四舍五入总成本
	summary.TotalCost = math.Round(summary.TotalCost*10000) / 10000

	return &PositionListResp{
		List:    details,
		Total:   total,
		Page:    page,
		Size:    pageSize,
		Summary: summary,
	}, nil
}

// UpdatePositionNote 更新持仓备注
func (svc *PortfolioService) UpdatePositionNote(id uint, note string, uid uint) error {
	position, err := db.GetPositionByIDAndUID(id, uid)
	if err != nil {
		return errors.New("持仓不存在或无权访问")
	}
	position.Note = note
	return db.UpdatePosition(position)
}

// DeletePosition 删除持仓
func (svc *PortfolioService) DeletePosition(id, uid uint) error {
	// 先查持仓获取股票代码（用于降级优先级）
	position, err := db.GetPositionByIDAndUID(id, uid)
	if err != nil {
		return errors.New("持仓不存在或无权访问")
	}

	if err := db.DeletePositionByID(id, uid); err != nil {
		return err
	}

	// 删除后同步到关注列表（内部检查是否还有人持有）
	if svc.watchlistMgr != nil {
		svc.watchlistMgr.OnPositionClosed(uid, position.StockCode)
		svc.notifyHoldingChanged()
	}
	return nil
}

// ============================================================================
//  交易配置相关
// ============================================================================

// GetTradeConfig 获取用户交易配置
func (svc *PortfolioService) GetTradeConfig(uid uint) (*TradeConfigResp, error) {
	user, err := db.GetUserByID(uid)
	if err != nil {
		return nil, errors.New("获取用户信息失败")
	}
	return &TradeConfigResp{
		CommissionRate: user.CommissionRate,
		MinCommission:  user.MinCommission,
	}, nil
}

// UpdateTradeConfig 更新用户交易配置
func (svc *PortfolioService) UpdateTradeConfig(uid uint, req *TradeConfigUpdateReq) error {
	if req.CommissionRate <= 0 || req.CommissionRate > 10 {
		return errors.New("手续费率必须在 0~10 之间（单位：万分之x）")
	}
	return db.UpdateUserTradeConfig(uid, req.CommissionRate, req.MinCommission)
}

// ============================================================================
//  内部辅助方法
// ============================================================================

// calculateCommission 根据用户配置计算手续费
// commission_rate 单位为万分之x（如 2.5 = 万分之2.5）
// 公式: amount × (commission_rate / 10000)
// 规则: 如果 min_commission=true 且结果 < 5 则取 5 元
func (svc *PortfolioService) calculateCommission(uid uint, amount float64) float64 {
	config, err := svc.GetTradeConfig(uid)
	if err != nil {
		// 默认值: 万分之2.5, 不免五
		config = &TradeConfigResp{CommissionRate: 2.5, MinCommission: true}
	}

	fee := amount * config.CommissionRate / 10000.0
	// 四舍五入保留4位
	fee = math.Round(fee*10000) / 10000

	// 不免五: 最低手续费 5 元
	if config.MinCommission && fee < 5.0 {
		fee = 5.0
	}

	return fee
}

// enrichPositionDetail 补充持仓详情中的关联数据（股票名称、现价）
func (svc *PortfolioService) enrichPositionDetail(d *model.PositionDetail, _ uint) {
	// 1. 通过 stock_code 从 stocks 表查询股票名称
	var stock model.Stock
	err := db.GetDB().Where("code = ?", d.StockCode).First(&stock).Error
	if err == nil {
		d.StockName = stock.Name
	} else {
		d.StockName = d.StockCode // 查不到就用代码兜底
	}

	// 2. 获取现价：根据时间判断数据源
	if utils.IsPriceUpdateTime() {
		// 交易日 10:00~19:00 → 从 quotecache 获取实时价
		if svc.cache != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			qd, err := svc.cache.Get(ctx, d.StockCode)
			cancel()
			if err == nil && qd != nil && qd.Intraday != nil && qd.Intraday.Current > 0 {
				price := float64(qd.Intraday.Current) / 100 // 分→元
				d.CurrentPrice = math.Round(price*100) / 100
				return
			}
		}
	}

	// 非更新时段 或 行情接口失败 → 从日K线表取最新收盘价
	closePrice, err := db.GetLatestDailyClose(d.StockCode)
	if err == nil && closePrice > 0 {
		d.CurrentPrice = math.Round(closePrice*100) / 100
	}
}
