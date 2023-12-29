package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/vulpemventures/go-elements/transaction"
)

// Global state and mutex
var (
	isPolling = false
)

type OrderBook struct {
	OrderByID        map[string]*Order
	OrderIDByAddress map[string]string
	mu               sync.Mutex
}

var orderBook OrderBook = OrderBook{
	OrderByID:        make(map[string]*Order),
	OrderIDByAddress: make(map[string]string),
}

func main() {
	// Parse environment variables
	oceanURL := os.Getenv("OCEAN_URL")
	if oceanURL == "" {
		oceanURL = "localhost:18000"
	}
	watchIntervalStr := os.Getenv("WATCH_INTERVAL_SECONDS")
	watchInterval := -1
	if watchIntervalStr != "" {
		var err error
		watchInterval, err = strconv.Atoi(watchIntervalStr)
		if err != nil {
			log.Fatal("watchInterval: ", err)
		}
	}

	router := gin.Default()

	router.LoadHTMLGlob("web/*")

	router.POST("/api/offer", func(c *gin.Context) {

		// Extract values from the request
		inputValue := c.PostForm("input")
		outputValue := c.PostForm("output")
		inputCurrency := c.PostForm("inputCurrency")
		outputCurrency := c.PostForm("outputCurrency")
		traderScriptHex := c.PostForm("traderScript")

		order, err := NewOrder(traderScriptHex, inputCurrency, inputValue, outputCurrency, outputValue)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{})
			return
		}
		orderBook.OrderByID[order.ID] = order
		orderBook.OrderIDByAddress[order.Address] = order.ID

		// TODO start watching a specific address You can use Elements Core JSONRPC
		if watchInterval > 0 {
			orderBook.mu.Lock()
			if !isPolling {
				go startPolling(order.Address, uint64(order.Input.Amount), order.Input.Asset) // Replace with actual address logic
				isPolling = true
				fmt.Println("Polling started for the address " + order.Address)
			}
			orderBook.mu.Unlock()
		}

		c.Redirect(http.StatusSeeOther, "/offer/"+order.Address)
	})

	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "trade.html", gin.H{
			"IsDisabled": false,
		})
	})

	router.GET("/offer/:address", func(c *gin.Context) {
		address := c.Params.ByName("address")
		id, ok := orderBook.OrderIDByAddress[address]
		if id == "" || !ok {
			c.HTML(http.StatusNotFound, "404.html", gin.H{})
			return
		}
		order, ok := orderBook.OrderByID[id]
		if !ok {
			c.HTML(http.StatusNotFound, "404.html", gin.H{})
			return
		}

		utxos, err := fetchUnspents(order.Address)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{})
			return
		}

		status := "PENDING"
		if coinsAreMoreThan(utxos, order.Input.Amount) {
			//execute the trade
			status = "FUNDED"
			err := executeTrades(
				order,
				utxos,
				oceanURL,
			)
			if err != nil {
				err = fmt.Errorf("error executing trade: %v", err)
				fmt.Println(err)
				c.HTML(http.StatusInternalServerError, "error.html", gin.H{})
				return
			}
		}

		unspents := make([]map[string]interface{}, len(utxos))
		for i, utxo := range utxos {
			unspents[i] = map[string]interface{}{
				"Txid":      utxo.Txid,
				"TxidShort": utxo.Txid[:6] + "..." + utxo.Txid[len(utxo.Txid)-6:],
				"Index":     utxo.Index,
			}
		}

		inputCurrency := assetToCurrency[order.Input.Asset]
		outputCurrency := assetToCurrency[order.Output.Asset]

		/* transactions, err := fetchTransactionHistory(order.Address)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{})
			return
		} */

		c.HTML(http.StatusOK, "offer.html", gin.H{
			"address":        order.Address,
			"inputValue":     order.InputValue(),
			"inputCurrency":  inputCurrency,
			"outputValue":    order.OutputValue(),
			"outputCurrency": outputCurrency,
			"unspents":       unspents,
			//"transactions":   transactions,
			"status":         status,
			"inputAssetHash": order.Input.Asset,
			"inputAmount":    order.Input.Amount,
		})
	})

	router.GET("/login", func(c *gin.Context) {
		c.HTML(http.StatusOK, "login.html", gin.H{})
	})

	router.Run(":8080")
}

func coinsAreMoreThan(utxos []*UTXO, amount uint64) bool {
	// Calculate the total value of UTXOs
	totalValue := uint64(0)
	for _, utxo := range utxos {
		totalValue += utxo.Value
	}

	return totalValue >= amount
}

func executeTrades(order *Order, unspents []*UTXO, oceanURL string) error {
	walletSvc, err := NewWalletService(oceanURL)
	if err != nil {
		return err
	}

	for _, unspent := range unspents {
		prevout, err := fetchPrevout(unspent.Txid, unspent.Index)
		if err != nil {
			return err
		}
		unspent.Prevout = prevout
		trade := FromFundedOrder(
			walletSvc,
			order,
			unspent,
		)

		if trade.Status != Funded {
			return fmt.Errorf("trade is not funded: %v", err)
		}

		// Execute the trade
		err = trade.ExecuteTrade()
		if err != nil {
			return err
		}
	}

	return nil
}

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

func startPolling(address string, amount uint64, assetID string) {
	for {
		utxos, err := fetchUnspents(address)
		if err != nil {
			fmt.Printf("Error fetching UTXOs: %v\n", err)
			continue
		}

		if coinsAreMoreThan(utxos, amount) {
			fmt.Println(address + " being funded")
			// TODO Update the global state
		}

		// Wait for a defined interval before polling again
		time.Sleep(1 * time.Second)
	}
}
