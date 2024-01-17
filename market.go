package main

import "fmt"

type Market struct {
	BaseAsset         string
	QuoteAsset        string
	BuyPercentageFee  float64
	SellPercentageFee float64
	BuyLimit          uint64
	SellLimit         uint64
}

func getMarkets() []Market {
	return []Market{
		{
			BaseAsset:         "L-BTC",
			QuoteAsset:        "L-BTC",
			BuyPercentageFee:  0.1,
			SellPercentageFee: 0.1,
			BuyLimit:          500000000000,
			SellLimit:         500000000000,
		},
		{
			BaseAsset:         "FUSD",
			QuoteAsset:        "USDT",
			BuyPercentageFee:  0.1,
			SellPercentageFee: 3,
			BuyLimit:          500000000000,
			SellLimit:         500000000000,
		},
		{
			BaseAsset:         "L-BTC",
			QuoteAsset:        "USDT",
			BuyPercentageFee:  0.1,
			SellPercentageFee: 0.75,
			BuyLimit:          500000000000,
			SellLimit:         500000000000,
		},
		{
			BaseAsset:         "L-BTC",
			QuoteAsset:        "FUSD",
			BuyPercentageFee:  0.1,
			SellPercentageFee: 0.75,
			BuyLimit:          500000000000,
			SellLimit:         500000000000,
		},
		// Add more markets here if needed
	}
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

func getTradingPair(pair string) *Market {
	markets := getMarkets()
	for _, market := range markets {
		if market.BaseAsset+"/"+market.QuoteAsset == pair {
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
	<p class="text-gray-600"><strong>Buy Percentage Fee:</strong> %f</p>
	<p class="text-gray-600"><strong>Sell Percentage Fee:</strong> %f</p>
</div>`
	div = fmt.Sprintf(div, mkt.BuyLimit, mkt.SellLimit, mkt.BuyPercentageFee, mkt.SellPercentageFee)

	return div
}
