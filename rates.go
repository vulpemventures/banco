package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type Price struct {
	LastUpdated time.Time
	Rate        float64
}

func getFeePercentage(inputTicker, outputTicker string) float64 {
	// Set a fixed fee percentage
	const feePercentage = -0.2 // 0.2% fee
	return feePercentage
}

func getConversionRate(inputCurrency, outputCurrency string) (float64, error) {
	// Convert the input and output currencies using the currencyMapping
	inputCurrencySymbol, ok := currencyToSymbol[inputCurrency]
	if !ok {
		return 0, fmt.Errorf("unknown input currency: %s", inputCurrency)
	}

	outputCurrencySymbol, ok := currencyToSymbol[outputCurrency]
	if !ok {
		return 0, fmt.Errorf("unknown output currency: %s", outputCurrency)
	}

	// Check if the input and output are BTC and USD (or inverted)
	if (inputCurrencySymbol == "BTC" && outputCurrencySymbol == "USD") || (inputCurrencySymbol == "USD" && outputCurrencySymbol == "BTC") {
		rate, err := fetchConversionRate()
		if err != nil {
			return 0, err
		}

		// If the input is BTC and output is USD, return the rate
		if inputCurrencySymbol == "BTC" && outputCurrencySymbol == "USD" {
			return rate, nil
		}

		// If the input is USD and output is BTC, calculate the proportion
		return 1 / rate, nil
	}

	// For other currencies, use a hardcoded mapping of the rate for now
	rate := 1.0
	return rate, nil
}

type Response struct {
	Timestamp string `json:"timestamp"`
	LastPrice string `json:"lastPrice"`
}

var cache = make(map[string]Price)
var mutex = &sync.Mutex{}

func fetchConversionRate() (float64, error) {
	// Check the cache
	tradingPair := "BTCUSD"
	mutex.Lock()
	price, found := cache[tradingPair]
	mutex.Unlock()

	if found && time.Since(price.LastUpdated).Seconds() < 30 {
		// Return the cached rate if it's less than 10 seconds old
		return price.Rate, nil
	}

	// Fetch the conversion rate from the API
	resp, err := http.Get("https://oracle.fuji-labs.io/oracle/" + tradingPair)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	rate, err := strconv.ParseFloat(result.LastPrice, 64)
	if err != nil {
		return 0, err
	}

	// Update the cache
	mutex.Lock()
	cache[tradingPair] = Price{
		LastUpdated: time.Now(),
		Rate:        rate,
	}
	mutex.Unlock()

	return rate, nil
}
