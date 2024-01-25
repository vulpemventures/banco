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

type Esplora struct {
	BaseAPIURL  string
	NetworkName string
}

var EsploraAPIURLs = map[string]string{
	"liquid":  "https://blockstream.info/liquid/api",
	"testnet": "https://blockstream.info/liquidtestnet/api",
	"regtest": "http://localhost:3001",
}

var EsploraURLs = map[string]string{
	"liquid":  "https://blockstream.info/liquid",
	"testnet": "https://blockstream.info/liquidtestnet",
	"regtest": "http://localhost:5001",
}

func NewEsplora(networkName string) (*Esplora, error) {
	// Get the base API URL for the network
	baseAPIURL, ok := EsploraAPIURLs[networkName]
	if !ok {
		return nil, fmt.Errorf("invalid network %s", networkName)
	}

	return &Esplora{
		BaseAPIURL:  baseAPIURL,
		NetworkName: networkName,
	}, nil
}

func (e *Esplora) FetchTransactionHistory(address string) ([]Transaction, error) {
	apiURL := fmt.Sprintf("%s/address/%s/txs", e.BaseAPIURL, address)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("error fetching transaction history: %w", err)
	}

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	var transactions []Transaction
	err = json.Unmarshal(body, &transactions)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling JSON: %w", err)
	}

	return transactions, nil
}

func (e *Esplora) FetchPrevout(txHash string, txIndex int) (*transaction.TxOutput, error) {
	apiURL := fmt.Sprintf("%s/tx/%s/hex", e.BaseAPIURL, txHash)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("error fetching raw transaction: %v", err)
	}

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}

	tx, err := transaction.NewTxFromHex(string(body))
	if err != nil {
		return nil, fmt.Errorf("error creating transaction from hex: %v", err)

	}

	txOutput := tx.Outputs[txIndex]
	return txOutput, nil
}

func (e *Esplora) FetchUnspents(address string) ([]*UTXO, error) {
	apiURL := fmt.Sprintf("%s/address/%s/utxo", e.BaseAPIURL, address)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("error fetching UTXOs: %v", err)
	}

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}

	var utxos []*UTXO
	err = json.Unmarshal(body, &utxos)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling JSON: %v", err)
	}

	return utxos, nil
}
