package main

import (
	"fmt"
	"log"
	"time"

	"github.com/vulpemventures/go-elements/network"
)

// Network: Map of network names to struct instances
var SupportedNetworks map[string]*network.Network = map[string]*network.Network{
	"liquid":  &network.Liquid,
	"testnet": &network.Testnet,
	"regtest": &network.Regtest,
}

func getTransactionsForAddress(addr, networkName string) ([]Transaction, error) {
	esplora, err := NewEsplora(networkName)
	if err != nil {
		return nil, fmt.Errorf("esplora initialization error: %w", err)
	}
	transactions, err := esplora.FetchTransactionHistory(addr)
	if err != nil {
		return nil, fmt.Errorf("esplora fetch txs error: %w", err)
	}

	return transactions, nil
}

func watchForTrades(order *Order, oceanURL, oceanAccountName, networkName string) error {
	if duration := time.Since(order.Timestamp); duration > 10*time.Minute {
		err := updateOrderStatus(order.ID, "Expired")
		if err != nil {
			return fmt.Errorf("error updating order status: %w", err)
		}
	}

	esplora, err := NewEsplora(networkName)
	if err != nil {
		return fmt.Errorf("esplora initialization error: %w", err)
	}
	utxos, err := esplora.FetchUnspents(order.Address)
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
			oceanAccountName,
			networkName,
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

func executeTrades(order *Order, unspents []*UTXO, oceanURL, oceanAccountName, networkName string) ([]*Trade, error) {
	walletSvc, err := NewWalletService(oceanURL, oceanAccountName)
	if err != nil {
		return nil, err
	}

	esplora, err := NewEsplora(networkName)
	if err != nil {
		return nil, fmt.Errorf("esplora initialization error: %w", err)
	}

	trades := []*Trade{}
	for _, unspent := range unspents {
		prevout, err := esplora.FetchPrevout(unspent.Txid, unspent.Index)
		if err != nil {
			return nil, err
		}
		unspent.Prevout = prevout
		trade, err := FromFundedOrder(
			walletSvc,
			order,
			unspent,
		)
		if err != nil {
			return nil, err
		}

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
