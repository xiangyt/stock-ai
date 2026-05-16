package model

import (
	"time"

	"gorm.io/gorm"
)

// TradeType 交易类型
type TradeType int8

const (
	TradeTypeBuy  TradeType = 1 // 买入
	TradeTypeSell TradeType = 2 // 卖出
)

// PositionStatus 持仓状态
type PositionStatus string

const (
	PositionHolding PositionStatus = "holding" // 持有中
	PositionClosed PositionStatus = "closed"   // 已清仓
)

// ============================================================================
//  Position — 持仓记录
// ============================================================================

// Position 持仓表（GORM Model）
type Position struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	UID       uint           `gorm:"index;not null;default:0;comment:用户ID" json:"uid"`
	StockCode string         `gorm:"size:6;not null;column:stock_code" json:"stock_code"`
	Quantity  int            `gorm:"not null;default:0" json:"quantity"`          // 持仓数量(股)
	AvgCost   float64        `gorm:"type:decimal(14,4);not null;default:0.0000" json:"avg_cost"` // 平均成本价
	Status    string         `gorm:"size:20;not null;default:holding" json:"status"` // holding / closed
	Note      string         `gorm:"size:500" json:"note"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Position) TableName() string { return "positions" }

// PositionDetail 持仓详情响应（关联股票名称等）
type PositionDetail struct {
	ID            uint              `json:"id"`
	UID           uint              `json:"uid"`
	StockCode     string            `json:"stock_code"`
	StockName     string            `json:"stock_name,omitempty"` // 关联查询出的股票名称
	Quantity      int               `json:"quantity"`
	AvgCost       float64           `json:"avg_cost"`
	Status        string            `json:"status"`
	TotalCost     float64           `json:"total_cost"`     // 总成本 = avg_cost * quantity
	TradeCount    int               `json:"trade_count"`    // 交易笔数
	Note          string            `json:"note"`
	CreatedAt     string            `json:"created_at"`
	UpdatedAt     string            `json:"updated_at"`
	Trades        []PositionTradeDetail `json:"trades,omitempty"` // 详情时携带交易记录
}

// ToPositionDetail 将 Position 转为 Detail 响应
func (p *Position) ToPositionDetail() *PositionDetail {
	return &PositionDetail{
		ID:         p.ID,
		UID:        p.UID,
		StockCode:  p.StockCode,
		Quantity:   p.Quantity,
		AvgCost:    p.AvgCost,
		Status:     p.Status,
		TotalCost:  round4(p.AvgCost * float64(p.Quantity)),
		Note:       p.Note,
		CreatedAt:  p.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
		UpdatedAt:  p.UpdatedAt.Format("2006-01-02T15:04:05.000Z"),
	}
}

// ============================================================================
//  PositionTrade — 交易记录
// ============================================================================

// PositionTrade 交易记录表（GORM Model）
type PositionTrade struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	PositionID uint      `gorm:"index;not null;comment:关联持仓ID" json:"position_id"`
	UID        uint      `gorm:"not null;default:0;comment:用户ID" json:"uid"`
	TradeType  int8      `gorm:"not null;comment:交易类型1买2卖" json:"trade_type"`
	Quantity   int       `gorm:"not null" json:"quantity"`             // 交易数量(股)
	Price      float64   `gorm:"type:decimal(14,4);not null" json:"price"` // 成交价格
	Amount     float64   `gorm:"type:decimal(20,4);not null;default:0.0000" json:"amount"` // 成交金额
	Commission float64   `gorm:"type:decimal(14,4);not null;default:0.0000" json:"commission"` // 手续费
	TradeDate  time.Time `gorm:"type:date;not null;column:trade_date" json:"trade_date"`
	Note       string    `gorm:"size:500" json:"note"`
	CreatedAt  time.Time `json:"created_at"`
}

func (PositionTrade) TableName() string { return "position_trades" }

// PositionTradeDetail 交易记录详情响应
type PositionTradeDetail struct {
	ID         uint    `json:"id"`
	PositionID uint    `json:"position_id"`
	TradeType  int8    `json:"trade_type"`
	TradeTypeName string `json:"trade_type_name"` // "买入" / "卖出"
	Quantity   int     `json:"quantity"`
	Price      float64 `json:"price"`
	Amount     float64 `json:"amount"`
	Commission float64 `json:"commission"`
	TradeDate  string  `json:"trade_date"`
	Note       string  `json:"note"`
}

// ToDetail 将 PositionTrade 转为 Detail 响应
func (t *PositionTrade) ToDetail() *PositionTradeDetail {
	typeName := "卖出"
	if t.TradeType == 1 {
		typeName = "买入"
	}
	return &PositionTradeDetail{
		ID:            t.ID,
		PositionID:    t.PositionID,
		TradeType:     t.TradeType,
		TradeTypeName: typeName,
		Quantity:      t.Quantity,
		Price:         t.Price,
		Amount:        round4(t.Amount),
		Commission:    round4(t.Commission),
		TradeDate:     t.TradeDate.Format("2006-01-02"),
		Note:          t.Note,
	}
}

// ============================================================================
//  辅助函数
// ============================================================================

// round4 四舍五入保留4位小数
func round4(v float64) float64 {
	return float64(int(v*10000+0.5)) / 10000
}
