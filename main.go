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
	isPolling  = false
	errCh      = make(chan error)
	orderQueue = make(chan *Order)
)

type OrderBook struct {
	OrderByID        map[string]*Order
	OrderIDByAddress map[string]string
	mu               sync.Mutex
}

var orderBook OrderBook = OrderBook{
	OrderByID: make(map[string]*Order),
}

var oceanURL string = os.Getenv("OCEAN_URL")
var watchIntervalStr string = os.Getenv("WATCH_INTERVAL_SECONDS")

func main() {
	// Parse environment variables
	if oceanURL == "" {
		oceanURL = "localhost:18000"
	}
	watchInterval := -1
	if watchIntervalStr != "" {
		var err error
		watchInterval, err = strconv.Atoi(watchIntervalStr)
		if err != nil {
			log.Fatal("watchInterval: ", err)
		}
	}

	//processOrderQueue(oceanURL)
	if watchInterval > 0 {
		// start watching
		/* startWatching(func() {
			processOrderQueue(oceanURL)
		}, watchInterval) */
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

		println("new order", order)
		orderBook.mu.Lock()
		orderBook.OrderByID[order.ID] = order
		//orderQueue <- order
		orderBook.mu.Unlock()

		c.Redirect(http.StatusSeeOther, "/offer/"+order.ID)
	})

	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "trade.html", gin.H{
			"IsDisabled": false,
		})
	})

	router.GET("/offer/:id", func(c *gin.Context) {
		id := c.Params.ByName("id")
		order, ok := orderBook.OrderByID[id]
		if !ok {
			c.HTML(http.StatusNotFound, "404.html", gin.H{})
			return
		}

		err := watchForTrades(order, oceanURL)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": err.Error()})
			return
		}

		transactions, err := fetchTransactionHistory(order.Address)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{})
			return
		}

		println("fetched txs", len(transactions))

		// manipulate template data and render page
		transactionHistory := make([]map[string]interface{}, len(transactions))
		for i, tx := range transactions {
			transactionHistory[i] = map[string]interface{}{
				"Txid":      tx.TxID,
				"TxidShort": tx.TxID[:6] + "..." + tx.TxID[len(tx.TxID)-6:],
				"Confirmed": tx.Status.Confirmed,
				"Date":      time.Unix(int64(tx.Status.BlockTime), 0).Format("2006-01-02 15:04:05"),
				"BlockHash": tx.Status.BlockHash,
				"BlockTime": tx.Status.BlockTime,
			}
		}
		inputCurrency := assetToCurrency[order.Input.Asset]
		outputCurrency := assetToCurrency[order.Output.Asset]

		c.HTML(http.StatusOK, "offer.html", gin.H{
			"address":        order.Address,
			"inputValue":     order.InputValue(),
			"inputCurrency":  inputCurrency,
			"outputValue":    order.OutputValue(),
			"outputCurrency": outputCurrency,
			"transactions":   transactionHistory,
			"inputAssetHash": order.Input.Asset,
			"inputAmount":    order.Input.Amount,
		})
	})

	router.GET("/login", func(c *gin.Context) {
		c.HTML(http.StatusOK, "login.html", gin.H{})
	})

	router.Run(":8080")
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
