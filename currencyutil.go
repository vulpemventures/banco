package main

type Asset struct {
	AssetHash string
	Precision int
	Ticker    string
	Name      string
}

var currencyToAsset = map[string]Asset{
	"USDT":  {"f3d1ec678811398cd2ae277cbe3849c6f6dbd72c74bc542f7c4b11ff0e820958", 8, "USDT", "Tether USD"},
	"L-BTC": {"144c654344aa716d6f3abcc1ca90e5641e4e2a7f633bc09fe3baf64585819a49", 8, "L-BTC", "Liquid Bitcoin"},
}

var assetToCurrency = map[string]string{
	"f3d1ec678811398cd2ae277cbe3849c6f6dbd72c74bc542f7c4b11ff0e820958": "USDT",
	"144c654344aa716d6f3abcc1ca90e5641e4e2a7f633bc09fe3baf64585819a49": "L-BTC",
}
