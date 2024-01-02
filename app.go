package main

import (
	"fmt"
	"log"
	"time"
)

func watchForTrades(order *Order, oceanURL string) error {
	if duration := time.Since(order.Timestamp); duration > 10*time.Minute {
		err := updateOrderStatus(order.ID, "Expired")
		if err != nil {
			return fmt.Errorf("error updating order status: %w", err)
		}
	}

	utxos, err := fetchUnspents(order.Address)
	if err != nil {
		return fmt.Errorf("error fetching unspents: %w", err)
	}

	// TODO Check also the asset type
	if coinsAreMoreThan(utxos, order.Input.Amount) {
		updateOrderStatus(order.ID, "Funded")

		trades, err := executeTrades(
			order,
			utxos,
			oceanURL,
		)
		if err != nil {
			return fmt.Errorf("error executing trade: %v", err)
		}

		for _, trade := range trades {
			log.Printf("executed trade for order ID: %s\n", trade.Order.ID)
		}

		updateOrderStatus(order.ID, "Fulfilled")
	}
	return nil
}

func coinsAreMoreThan(utxos []*UTXO, amount uint64) bool {
	// Calculate the total value of UTXOs
	totalValue := uint64(0)
	for _, utxo := range utxos {
		totalValue += utxo.Value
	}

	return totalValue >= amount
}

func executeTrades(order *Order, unspents []*UTXO, oceanURL string) ([]*Trade, error) {
	walletSvc, err := NewWalletService(oceanURL)
	if err != nil {
		return nil, err
	}

	trades := []*Trade{}
	for _, unspent := range unspents {
		prevout, err := fetchPrevout(unspent.Txid, unspent.Index)
		if err != nil {
			return nil, err
		}
		unspent.Prevout = prevout
		trade := FromFundedOrder(
			walletSvc,
			order,
			unspent,
		)

		if trade.Status != Funded {
			return nil, fmt.Errorf("trade is not funded: %v", err)
		}

		// Execute the trade
		err = trade.ExecuteTrade()
		if err != nil {
			return nil, err
		}
		trades = append(trades, trade)
	}

	return trades, nil
}
