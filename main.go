package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/vulpemventures/go-elements/address"
	_ "modernc.org/sqlite"
)

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

	// DB
	_, err := initDB()
	if err != nil {
		log.Fatal("connectToDB: ", err)
	}

	// Start processing pending trades
	if watchInterval > 0 {
		// start watching
		go startWatching(func() {
			orders, err := fetchOrdersToFulfill()
			if err != nil {
				log.Println("error in fetchPendingOrders", err)
				return
			}
			log.Println("Pending orders", len(orders))
			for _, order := range orders {
				err = watchForTrades(order, oceanURL)
				if err != nil {
					log.Println(fmt.Errorf("error in fulfilling order with ID %s: %v", order.ID, err))
					continue
				}
			}

		}, watchInterval)
	}

	// rates client
	rates := NewKrakenClient()
	if rates == nil {
		log.Fatalf("cant be nil")
	}

	err = rates.Subscribe()
	if err != nil {
		log.Fatalf("failed to subscribe to kraken ws: %v", err)
	}

	router := gin.Default()
	router.LoadHTMLGlob("web/*")

	// API
	router.GET("/rates", func(c *gin.Context) {
		// params
		input := c.Query("inputCurrency")
		output := c.Query("outputCurrency")

		mkt := getMarket(input, output)
		if mkt == nil {
			c.String(http.StatusBadRequest, "Invalid currency pair")
			return
		}
		var operation string
		var limit uint64
		if mkt.BaseAsset == output {
			operation = "Buy"
			limit = mkt.BuyLimit
		} else if mkt.QuoteAsset == input {
			operation = "Sell"
			limit = mkt.SellLimit
		}

		// Set the necessary headers for SSE
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")

		// Create a new ticker that fires every second
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		// Loop that sends a message every second until the client disconnects
		for {
			select {
			case <-c.Done():
				// The client has disconnected, stop sending messages
				return
			case <-ticker.C:
				// The ticker has fired, send a message
				price, err := rates.MarketPrice(mkt.BaseAsset, mkt.QuoteAsset)
				if err != nil {
					log.Println(err.Error())
					continue
				}

				html := fmt.Sprintf("<div>1 %s = %f %s</div>", input, price, output)
				html += fmt.Sprintf("<div>%s limit: %d</div>", operation, limit)

				// Create the SSE data string
				sse := fmt.Sprintf("event: rate\ndata: %s\n\n", html)

				// Write the SSE data string to the response
				_, err = c.Writer.Write([]byte(sse))
				if err != nil {
					log.Println(err.Error())
					return
				}

				// Flush the response writer to send the data immediately
				c.Writer.Flush()
			}
		}
	})

	router.GET("/trade/preview", func(c *gin.Context) {
		// Get the input ticker, output ticker, and amount from the query parameters
		amountStr := c.Query("amount")
		pair := c.Query("pair")
		tradeType := c.Query("type")

		log.Println(amountStr, pair, tradeType)
		// Convert the input value to a float
		amount, err := strconv.ParseFloat(amountStr, 64)
		if err != nil {
			c.String(http.StatusBadRequest, "Invalid input value")
			return
		}

		// Get the conversion rate and fee
		mkt := getTradingPair(pair)
		if mkt == nil {
			c.String(http.StatusBadRequest, "Invalid trading pair")
			return
		}

		rate, err := rates.MarketPrice(mkt.BaseAsset, mkt.QuoteAsset)
		if err != nil {
			c.String(http.StatusInternalServerError, "error getting price from stream")
			return
		}

		// TODO check if need to be inverse
		feePercentage := rates.FeePercentage(mkt.BaseAsset, mkt.QuoteAsset)

		// Calculate the output amount
		previewAmt := rate * amount

		// Adjust the output amount based on the trade type and determine the action
		var action string
		if tradeType == "Buy" {
			previewAmt *= (1 + feePercentage/100) // Add the fee to the output amount
			action = "You Send"
		} else { // Sell
			previewAmt *= (1 - feePercentage/100) // Subtract the fee from the output amount
			action = "You Receive"
		}

		// Return the output amount
		outputValueHTML := fmt.Sprintf(`<div id="recapBox" class="p-4 bg-gray-100 rounded-lg">
    	<label id="recapText" class="block text-sm font-medium text-gray-700">%s</label>
    <p id="recapAmount" class="text-lg font-semibold">%f %s</p>
</div>`, action, previewAmt, mkt.QuoteAsset)

		// Return the HTML string
		c.String(http.StatusOK, outputValueHTML)
	})

	router.POST("/trade", func(c *gin.Context) {

		traderScriptHex := c.PostForm("traderScript")
		tradingPair := c.PostForm("pair")
		amountStr := c.PostForm("amount")
		tradeType := c.PostForm("type")

		if amountStr == "" || tradingPair == "" || tradeType == "" || traderScriptHex == "" {
			c.HTML(http.StatusBadRequest, "404.html", gin.H{"error": "missing form data"})
			return
		}

		// Convert the input value to a float
		amount, err := strconv.ParseFloat(amountStr, 64)
		if err != nil {
			c.String(http.StatusBadRequest, "Invalid input value")
			return
		}

		// Get the conversion rate and fee
		mkt := getTradingPair(tradingPair)
		if mkt == nil {
			c.String(http.StatusBadRequest, "Invalid trading pair")
			return
		}

		price, err := rates.MarketPrice(mkt.BaseAsset, mkt.QuoteAsset)
		if err != nil {
			c.String(http.StatusInternalServerError, "error getting price from stream")
			return
		}
		feePercentage := rates.FeePercentage(mkt.BaseAsset, mkt.QuoteAsset)
		// Determine the input value, input currency, output value, and output currency based on the trade type
		var inputValue, outputValue float64
		var inputCurrency, outputCurrency string
		if tradeType == "Buy" {
			inputValue = amount * (1 + feePercentage/100) // Add the fee to the input value
			inputCurrency = mkt.QuoteAsset
			outputValue = amount
			outputCurrency = mkt.BaseAsset
		} else if tradeType == "Sell" { // Sell
			inputValue = amount
			inputCurrency = mkt.BaseAsset
			outputValue = amount * price * (1 - feePercentage/100) // Subtract the fee from the output value
			outputCurrency = mkt.QuoteAsset
		}

		order, err := NewOrder(traderScriptHex, inputCurrency, fmt.Sprintf("%v", inputValue), outputCurrency, fmt.Sprintf("%v", outputValue), price)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": err.Error()})
			return
		}

		err = saveOrder(order)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": err.Error()})
			return
		}

		c.Redirect(http.StatusSeeOther, "/offer/"+order.ID)
	})

	router.GET("/", func(c *gin.Context) {
		markets := getMarkets()
		c.HTML(http.StatusOK, "trade.html", gin.H{
			"markets": markets,
		})
	})

	router.GET("/offer/address/:address", func(c *gin.Context) {
		addr := c.Params.ByName("address")

		ID, err := fetchOrderIDByAddress(addr)
		if err != nil {
			log.Println(err.Error())
			c.HTML(http.StatusNotFound, "404.html", gin.H{"error": err.Error()})
			return
		}

		c.Redirect(http.StatusSeeOther, "/offer/"+ID)
	})

	router.GET("/offer/:id", func(c *gin.Context) {
		id := c.Params.ByName("id")

		order, status, err := fetchOrderByID(id)
		if err != nil {
			log.Println(err.Error())
			c.HTML(http.StatusNotFound, "404.html", gin.H{"error": err.Error()})
			return
		}

		transactions, err := fetchTransactionHistory(order.Address)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{})
			return
		}

		// manipulate template data and render page
		transactionHistory := make([]map[string]interface{}, len(transactions))
		for i, tx := range transactions {
			transactionHistory[i] = map[string]interface{}{
				"txID":      tx.TxID,
				"txIDShort": tx.TxID[:6] + "..." + tx.TxID[len(tx.TxID)-6:],
				"confirmed": tx.Status.Confirmed,
				"date":      time.Unix(int64(tx.Status.BlockTime), 0).Format("2006-01-02 15:04:05"),
			}
		}
		inputCurrency := assetToCurrency[order.Input.Asset]
		outputCurrency := assetToCurrency[order.Output.Asset]
		date := order.Timestamp.Format("2006-01-02 15:04:05")
		c.HTML(http.StatusOK, "offer.html", gin.H{
			"id":             order.ID,
			"address":        order.Address,
			"inputValue":     order.InputValue(),
			"inputCurrency":  inputCurrency,
			"outputValue":    order.OutputValue(),
			"outputCurrency": outputCurrency,
			"transactions":   transactionHistory,
			"inputAssetHash": order.Input.Asset,
			"inputAmount":    order.Input.Amount,
			"status":         status,
			"date":           date,
		})
	})

	router.GET("/offer/:id/events", func(c *gin.Context) {
		id := c.Params.ByName("id")

		// Set the necessary headers for SSE
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")

		// Create a new channel, over which we will send the events to the client
		messageChan := make(chan string)

		// Create a new goroutine
		go func() {
			for {
				order, status, err := fetchOrderByID(id)
				if err != nil {
					log.Println("error fetching order by ID:", err)
					continue
				}

				transactions, err := fetchTransactionHistory(order.Address)
				if err != nil {
					log.Println("Error fetching transaction history:", err)
					continue
				}

				transactionHistory := make([]map[string]interface{}, len(transactions))
				for i, tx := range transactions {
					transactionHistory[i] = map[string]interface{}{
						"txID":      tx.TxID,
						"txIDShort": tx.TxID[:6] + "..." + tx.TxID[len(tx.TxID)-6:],
						"confirmed": tx.Status.Confirmed,
						"date":      time.Unix(int64(tx.Status.BlockTime), 0).Format("2006-01-02 15:04:05"),
					}
				}

				// Prepare the data
				data := map[string]interface{}{
					"status":       status,
					"transactions": transactionHistory,
				}

				// Create a new template
				tmpl, err := template.ParseFiles("web/transactions.html")
				if err != nil {
					log.Println(err.Error())
					return
				}

				// Execute the template with the data and write the result to a string
				var html bytes.Buffer
				if err := tmpl.Execute(&html, data); err != nil {
					log.Println(err.Error())
					return
				}

				htmlStr := strings.ReplaceAll(html.String(), "\n", " ")
				messageChan <- htmlStr
				time.Sleep(3 * time.Second)
			}
		}()

		// Create a loop that will continuously write new events to the stream
		for {
			select {
			case html := <-messageChan:
				// Write the HTML string to the response writer
				c.Writer.Write([]byte(fmt.Sprintf("data: %v\n\n", html)))
				c.Writer.Flush()
			case <-c.Done():
				// If the client has disconnected, we can stop sending events
				return
			}
		}
	})

	router.GET("/address-to-script/:address", func(c *gin.Context) {
		// Extract the address from the URL parameter
		addr := c.Param("address")

		// Decode the address using go-elements
		script, err := address.ToOutputScript(addr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid address"})
			return
		}

		// Convert the script to a hex string
		scriptHex := hex.EncodeToString(script)

		// Return the scriptHex as a string
		c.String(http.StatusOK, scriptHex)
	})

	router.Run(":8080")
}

func startWatching(fn func(), watchInterval int) {
	for {
		fn()
		// Wait for a defined interval before polling again
		time.Sleep(time.Duration(watchInterval) * time.Second)
	}
}
