package main

var currencyToAsset = map[string]struct {
	AssetHash string
	Precision int
}{
	"FUSD":   {"0d86b2f6a8c3b02a8c7c8836b83a081e68b7e2b4bcdfc58981fc5486f59f7518", 8},
	"USDT":   {"f3d1ec678811398cd2ae277cbe3849c6f6dbd72c74bc542f7c4b11ff0e820958", 8},
	"tL-BTC": {"144c654344aa716d6f3abcc1ca90e5641e4e2a7f633bc09fe3baf64585819a49", 8},
}

var assetToCurrency = map[string]string{
	"0d86b2f6a8c3b02a8c7c8836b83a081e68b7e2b4bcdfc58981fc5486f59f7518": "FUSD",
	"f3d1ec678811398cd2ae277cbe3849c6f6dbd72c74bc542f7c4b11ff0e820958": "USDT",
	"144c654344aa716d6f3abcc1ca90e5641e4e2a7f633bc09fe3baf64585819a49": "tL-BTC",
}
