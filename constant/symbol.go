package constant

import "dca-bot/config"

func GetTokenMap() map[string]any {
	return map[string]any{
		"btcusdt": map[float64]string{
			2:   config.BTC2,
			1.5: config.BTC1x5,
			1:   config.BTC1,
		},
		"ethusdt": map[float64]string{
			2:   config.ETH2,
			1.5: config.ETH1x5,
			1:   config.ETH1,
		},
	}
}

func GetFixedRangeTokenMap() map[string]any {
	return map[string]any{
		"btcusdt": config.BTC_FIX,
	}
}
