package main

type Asset struct {
	AssetHash string
	Precision int
	Ticker    string
	Name      string
}

var currencyToSymbol = map[string]string{
	"USDT":  "USD",
	"FUSD":  "USD",
	"L-BTC": "BTC",
}

var currencyToAsset = map[string]Asset{
	"FUSD":  {"0d86b2f6a8c3b02a8c7c8836b83a081e68b7e2b4bcdfc58981fc5486f59f7518", 8, "FUSD", "Fuji USD"},
	"USDT":  {"f3d1ec678811398cd2ae277cbe3849c6f6dbd72c74bc542f7c4b11ff0e820958", 8, "USDT", "Tether USD"},
	"L-BTC": {"144c654344aa716d6f3abcc1ca90e5641e4e2a7f633bc09fe3baf64585819a49", 8, "L-BTC", "Liquid Bitcoin"},
}

var assetToCurrency = map[string]string{
	"0d86b2f6a8c3b02a8c7c8836b83a081e68b7e2b4bcdfc58981fc5486f59f7518": "FUSD",
	"f3d1ec678811398cd2ae277cbe3849c6f6dbd72c74bc542f7c4b11ff0e820958": "USDT",
	"144c654344aa716d6f3abcc1ca90e5641e4e2a7f633bc09fe3baf64585819a49": "L-BTC",
}

func tradableAssets(firstAsset string) []Asset {
	assets := make([]Asset, 0, len(currencyToAsset))
	for _, asset := range currencyToAsset {
		assets = append(assets, asset)
	}

	// Find the index of the firstAsset in the assets slice
	index := -1
	for i, asset := range assets {
		if asset.Ticker == firstAsset {
			index = i
			break
		}
	}

	// Move the firstAsset to the beginning of the assets slice
	if index >= 0 {
		assets[0], assets[index] = assets[index], assets[0]
	}

	return assets
}
