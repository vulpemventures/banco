package main

import (
	"testing"
)

type MockRatesClient struct{}

func (mrc *MockRatesClient) MarketPrice(base, quote string) (float64, error) {
	return 0.0, nil
}

func (mrc *MockRatesClient) Subscribe() error {
	return nil
}

func (mrc *MockRatesClient) FeePercentage(inputTicker, outputTicker string) float64 {
	return 0.0
}

func TestRates(t *testing.T) {
	mockClient := &MockRatesClient{}
	rates := Rates{
		Client: mockClient,
	}

	// Test MarketPriceStream
	err := rates.Client.Subscribe()
	if err != nil {
		t.Errorf("Error calling MarketPriceStream: %s", err.Error())
	}

	// Test FeePercentage
	fee := rates.Client.FeePercentage("L-BTC", "USD")
	if fee != 0.0 {
		t.Errorf("Expected fee percentage to be 0.0, got %f", fee)
	}

	_, err = rates.Client.MarketPrice("L-BTC", "USDT")
	if err != nil {
		t.Errorf("Error calling MarketPrice: %s", err.Error())
	}
}
