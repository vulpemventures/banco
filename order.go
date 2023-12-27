package main

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/vulpemventures/go-elements/elementsutil"
	"github.com/vulpemventures/go-elements/network"
)

type Order struct {
	ID            string
	Timestamp     time.Time
	Address       string
	FulfillScript []byte
	RefundScript  []byte
	TraderScript  []byte
	Input         struct {
		Asset  string
		Amount uint64
	}
	Output struct {
		Asset  string
		Amount uint64
	}
}
type OrderStatus string

func NewOrder(traderScriptHex, inputCurrency, inputValue, outputCurrency, outputValue string) (*Order, error) {

	traderScript, err := hex.DecodeString(traderScriptHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode trader script: %w", err)
	}

	inputAsset, ok := currencyToAsset[inputCurrency]
	if !ok {
		return nil, fmt.Errorf("failed to get input asset for currency: %s", inputCurrency)
	}
	inputAssetBytes, err := elementsutil.AssetHashToBytes(inputAsset.AssetHash)
	if err != nil {
		return nil, fmt.Errorf("failed to convert input asset hash: %w", err)
	}
	inputValueFloat, err := strconv.ParseFloat(inputValue, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse input value: %w", err)
	}
	inputAmount := uint64(float64(inputAsset.Precision) * inputValueFloat)
	if err != nil {
		return nil, fmt.Errorf("failed to get input asset: %w", err)
	}

	outputAsset, ok := currencyToAsset[outputCurrency]
	if !ok {
		return nil, fmt.Errorf("failed to get output asset for currency: %s", outputCurrency)
	}
	outputAssetBytes, err := elementsutil.AssetHashToBytes(outputAsset.AssetHash)
	if err != nil {
		return nil, fmt.Errorf("failed to convert output asset hash: %w", err)
	}
	outputValueFloat, err := strconv.ParseFloat(outputValue, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse output value: %w", err)
	}
	outputAmount := uint64(float64(outputAsset.Precision) * outputValueFloat)
	if err != nil {
		return nil, fmt.Errorf("failed to get output asset: %w", err)
	}

	fulfillScript, _ := FulfillScript(traderScript, outputAmount, outputAssetBytes)
	refundScript, _ := RefundScript(traderScript, inputAmount, inputAssetBytes)

	output, err := CreateFundingOutput(fulfillScript, refundScript, &network.Testnet)
	if err != nil {
		return nil, fmt.Errorf("failed to create funding output: %w", err)
	}
	address, err := output.TaprootAddress()
	if err != nil {
		return nil, fmt.Errorf("failed to get taproot address: %w", err)
	}

	return &Order{
		ID:            uuid.New().String(),
		Timestamp:     time.Now(),
		Address:       address,
		FulfillScript: fulfillScript,
		RefundScript:  refundScript,
		TraderScript:  traderScript,
		Input: struct {
			Asset  string
			Amount uint64
		}{
			Asset:  inputAsset.AssetHash,
			Amount: inputAmount,
		},
		Output: struct {
			Asset  string
			Amount uint64
		}{
			Asset:  outputAsset.AssetHash,
			Amount: outputAmount,
		},
	}, nil
}
