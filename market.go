package main

import "fmt"

type Market struct {
	BaseAsset         string
	QuoteAsset        string
	BuyFixedFee       uint64
	BuyPercentageFee  float64
	SellFixedFee      uint64
	SellPercentageFee float64
	BuyLimit          uint64
	SellLimit         uint64
}

func getMarkets() []Market {
	markets := []Market{
		{
			BaseAsset:         "FUSD",
			QuoteAsset:        "USDT",
			BuyFixedFee:       100000000,
			BuyPercentageFee:  0.1,
			SellFixedFee:      100000000,
			SellPercentageFee: 3,
			BuyLimit:          500000000000,
			SellLimit:         500000000000,
		},
		{
			BaseAsset:         "L-BTC",
			QuoteAsset:        "USDT",
			BuyFixedFee:       1000,
			BuyPercentageFee:  0.1,
			SellFixedFee:      100000000,
			SellPercentageFee: 0.75,
			BuyLimit:          500000000000,
			SellLimit:         500000000000,
		},
		{
			BaseAsset:         "L-BTC",
			QuoteAsset:        "FUSD",
			BuyFixedFee:       1000,
			BuyPercentageFee:  0.1,
			SellFixedFee:      100000000,
			SellPercentageFee: 0.75,
			BuyLimit:          500000000000,
			SellLimit:         500000000000,
		},
		// Add more markets here if needed
	}
	return markets
}

func getMarket(baseAsset, quoteAsset string) *Market {
	markets := getMarkets()
	for _, market := range markets {
		if (market.BaseAsset == baseAsset && market.QuoteAsset == quoteAsset) ||
			(market.BaseAsset == quoteAsset && market.QuoteAsset == baseAsset) {
			return &market
		}
	}
	return nil
}

func printMarketInfo(mkt *Market) string {
	// Print a <div> for each limit, fee percentage, and fixed fee
	div := `<div class="bg-white p-6 rounded-lg shadow-lg">
	<h2 class="text-2xl font-bold mb-2 text-gray-700">Market Details</h2>
	<p class="text-gray-600"><strong>Buy Limit:</strong> %d</p>
	<p class="text-gray-600"><strong>Sell Limit:</strong> %d</p>
	<p class="text-gray-600"><strong>Buy Fixed Fee:</strong> %f</p>
	<p class="text-gray-600"><strong>Sell Fixed Fee:</strong> %f</p>
	<p class="text-gray-600"><strong>Buy Percentage Fee:</strong> %f</p>
	<p class="text-gray-600"><strong>Sell Percentage Fee:</strong> %f</p>
</div>`
	div = fmt.Sprintf(div, mkt.BuyLimit, mkt.SellLimit, float64(mkt.BuyFixedFee), float64(mkt.SellFixedFee), mkt.BuyPercentageFee, mkt.SellPercentageFee)

	return div
}
