package backtest

// AShareFeeCalculator A 股费用计算器。
type AShareFeeCalculator struct{ rate float64; minCommission bool }

// NewAShareFeeCalculator 创建 A 股费用计算器。rate 为佣金率（万分之）。
func NewAShareFeeCalculator(rate float64, minCommission bool) *AShareFeeCalculator {
	return &AShareFeeCalculator{rate: rate, minCommission: minCommission}
}

// BuyFee 计算买入佣金（元）。
func (f *AShareFeeCalculator) BuyFee(amount float64) float64 {
	return calcCommission(amount, f.rate, f.minCommission)
}

// SellFee 计算卖出费用（佣金+印花税）。
func (f *AShareFeeCalculator) SellFee(amount float64) float64 {
	return calcCommission(amount, f.rate, f.minCommission) + amount*0.001
}

// calcCommission 计算佣金。
func calcCommission(amount, rate float64, minCommission bool) float64 {
	if amount <= 0 { return 0 }
	c := amount * rate / 10000.0
	if minCommission && c < 5.0 { return 5.0 }
	return c
}
