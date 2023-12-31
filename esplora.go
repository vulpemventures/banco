package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/vulpemventures/go-elements/transaction"
)

type Transaction struct {
	TxID   string `json:"txid"`
	Status struct {
		Confirmed   bool   `json:"confirmed"`
		BlockHeight int    `json:"block_height"`
		BlockHash   string `json:"block_hash"`
		BlockTime   int    `json:"block_time"`
	} `json:"status"`
}

func fetchTransactionHistory(address string) ([]Transaction, error) {
	apiURL := fmt.Sprintf("https://blockstream.info/liquidtestnet/api/address/%s/txs", address)

	resp, err := http.Get(apiURL)
	if err != nil {
		fmt.Printf("Error fetching transaction history: %v\n", err)
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		fmt.Printf("Error reading response body: %v\n", err)
		return nil, err
	}

	var transactions []Transaction
	err = json.Unmarshal(body, &transactions)
	if err != nil {
		fmt.Printf("Error unmarshaling JSON: %v\n", err)
		return nil, err
	}

	return transactions, nil
}

func fetchPrevout(txHash string, txIndex int) (*transaction.TxOutput, error) {
	apiURL := fmt.Sprintf("https://blockstream.info/liquidtestnet/api/tx/%s/hex", txHash)

	resp, err := http.Get(apiURL)
	if err != nil {
		fmt.Printf("Error fetching raw transaction: %v\n", err)
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		fmt.Printf("Error reading response body: %v\n", err)
		return nil, err
	}

	tx, err := transaction.NewTxFromHex(string(body))
	if err != nil {
		fmt.Printf("Error creating transaction from hex: %v\n", err)
		return nil, err
	}

	txOutput := tx.Outputs[txIndex]
	return txOutput, nil
}

func fetchUnspents(address string) ([]*UTXO, error) {
	apiURL := fmt.Sprintf("https://blockstream.info/liquidtestnet/api/address/%s/utxo", address)

	resp, err := http.Get(apiURL)
	if err != nil {
		fmt.Printf("Error fetching UTXOs: %v\n", err)
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		fmt.Printf("Error reading response body: %v\n", err)
		return nil, err
	}

	var utxos []*UTXO
	err = json.Unmarshal(body, &utxos)
	if err != nil {
		fmt.Printf("Error unmarshaling JSON: %v\n", err)
		return nil, err
	}

	return utxos, nil
}
