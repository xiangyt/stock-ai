package technical

import (
	"stock-ai/internal/indicator"
)

// All 返回技术面指标实例
func All() []indicator.Indicator {
	return []indicator.Indicator{
		NewMa(),
		NewPattern(),
		NewRedThree(),
		NewChanLongShort(),
		NewTopBottom(),
		NewRedTop(),
		NewMacd(),
		NewKdj(),
		NewRsi(),
		NewBoll(),
		NewCci(),
		NewCyq(),
	}
}
