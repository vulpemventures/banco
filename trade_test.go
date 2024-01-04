package main

import (
	"encoding/hex"
	"fmt"
	"testing"

	"math/rand"

	"github.com/stretchr/testify/assert"
)

func TestTrade_ExecuteTrade(t *testing.T) {
	// setup ocean wallet client
	walletSvc, err := NewWalletService("localhost:18000")
	if err != nil {
		t.Fatal("newOrder", err)
	}
	// Create a sample order
	order, err := submitDummyOrder()
	if err != nil {
		t.Fatal("newOrder", err)
	}

	trade := FromFundedOrder(
		walletSvc,
		order,
		&UTXO{
			Txid:  "foo",
			Index: 1,
		},
	)

	// Execute the trade
	err = trade.ExecuteTrade()

	// Assert that the trade was executed successfully
	assert.NoError(t, err)
}

func TestTrade_CancelTrade(t *testing.T) {
	// setup ocean wallet client
	walletSvc, err := NewWalletService("http://localhost:18000")
	if err != nil {
		t.Fatal("newOrder")
	}
	// Create a sample order
	order, err := submitDummyOrder()
	if err != nil {
		t.Fatal("newOrder")
	}

	// Create a new trade
	trade := FromFundedOrder(walletSvc, order, &UTXO{
		Txid:  "foo",
		Index: 1,
	})

	// Cancel the trade
	err = trade.CancelTrade()

	// Assert that the trade was cancelled successfully
	assert.NoError(t, err)
}

func submitDummyOrder() (*Order, error) {
	// Generate random traderScript
	traderScript := make([]byte, 32)
	traderScriptHex := hex.EncodeToString(traderScript)

	// Generate random inputCurrency and inputValue
	inputCurrency := generateRandomCurrency()
	inputValue := generateRandomValue()

	// Generate random outputCurrency and outputValue
	outputCurrency := generateRandomCurrency()
	outputValue := generateRandomValue()

	// Create the order with the generated values
	return NewOrder(traderScriptHex, inputCurrency, inputValue, outputCurrency, outputValue)
}

func generateRandomCurrency() string {
	currencies := []string{"FUSD", "USDT", "L-BTC"}
	randomIndex := rand.Intn(len(currencies))
	return currencies[randomIndex]
}

func generateRandomValue() string {
	// Generate random value using your preferred method
	// For example, you can generate a random float value
	// within a specific range
	minValue := 1.0
	maxValue := 1000.0
	randomValue := minValue + rand.Float64()*(maxValue-minValue)
	return fmt.Sprintf("%.2f", randomValue)
}
