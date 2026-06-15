# 回测分析 Part 1: Model 层

## 1.1 Strategy (策略定义)

文件: `internal/model/indicator.go` 第 382-414 行

```go
type Strategy struct {
    ID            uint           `gorm:"primarykey"`
    UID           uint           `gorm:"index"`
    Name          string         `gorm:"size:100;not null"`
    Description   string         `gorm:"size:500"`
    LogicalOp     LogicalOp      `gorm:"size:10;default:and"`  // "and" | "or"
    Conditions    string         `gorm:"type:text;not null"`   // JSON: []StrategySignal
    BacktestCount int            `gorm:"default