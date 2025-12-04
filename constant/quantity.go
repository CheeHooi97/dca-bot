package constant

// priceDecimal = 3 eg: ADA, 0.752
// positionSizeDecimal = 2 eg: ETH, 0.02
var SymbolPrecisionMap = map[string][]int{
	"btcusdt":   {2, 3},
	"ethusdt":   {2, 2},
	"adausdt":   {3, 0},
	"bnbusdt":   {1, 1},
	"solusdt":   {1, 1},
	"xrpusdt":   {2, 0},
	"dogeusdt":  {4, 0},
	"suiusdt":   {3, 0},
	"linkusdt":  {2, 0},
	"avaxusdt":  {2, 0},
	"tonusdt":   {3, 0},
	"dotusdt":   {3, 0},
	"thetausdt": {4, 0},
	"wldusdt":   {3, 0},
	"tiausdt":   {2, 0},
	"trumpusdt": {2, 0},
}

var QuantityMap = map[string]float64{
	"btcusdt":   0.005,
	"ethusdt":   0.2,
	"adausdt":   700,
	"bnbusdt":   0.5,
	"solusdt":   1,
	"xrpusdt":   100,
	"dogeusdt":  500,
	"suiusdt":   40,
	"linkusdt":  10,
	"avaxusdt":  5,
	"tonusdt":   40,
	"dotusdt":   30,
	"thetausdt": 150,
	"wldusdt":   100,
	"tiausdt":   50,
	"trumpusdt": 10,
}

var PercentageMap = map[string]float64{
	"1m":  2.0,
	"5m":  2.0,
	"15m": 3.0,
	"1h":  3.0,
	"4h":  5.0,
	"1d":  6.0,
}
