package main

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
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

	router := gin.Default()
	router.LoadHTMLGlob("web/*")

	// API
	router.POST("/trade", func(c *gin.Context) {

		// Extract values from the request
		inputValue := c.PostForm("inputValue")
		outputValue := c.PostForm("outputValue")
		inputCurrency := c.PostForm("inputCurrency")
		outputCurrency := c.PostForm("outputCurrency")
		traderScriptHex := c.PostForm("traderScript")
		if inputValue == "" || outputValue == "" || inputCurrency == "" || outputCurrency == "" || traderScriptHex == "" {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{"error": "missing form data"})
			return
		}

		order, err := NewOrder(traderScriptHex, inputCurrency, inputValue, outputCurrency, outputValue)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "404.html", gin.H{"error": err.Error()})
			return
		}

		err = saveOrder(order)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": err.Error()})
			return
		}

		c.Redirect(http.StatusSeeOther, "/offer/"+order.ID)
	})

	router.GET("/trade/preview", func(c *gin.Context) {
		inputCurrency := c.Query("inputCurrency")
		outputCurrency := c.Query("outputCurrency")
		inputValue := c.Query("inputValue")
		outputValue := c.Query("outputValue")

		if inputCurrency == "" || outputCurrency == "" {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{"error": "Missing inputCurrency or outputCurrency parameter"})
			return
		}

		rate, err := getConversionRate(inputCurrency, outputCurrency)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": err.Error()})
			return
		}

		var html string

		if inputValue != "" {
			floatValue, _ := strconv.ParseFloat(inputValue, 64)
			outputValue = fmt.Sprintf("%.2f", floatValue*rate)
			html = fmt.Sprintf(`<input type="text" name="outputValue" value="%s" class="text-right font-semibold bg-transparent outline-none">`, outputValue)
		} else if outputValue != "" {
			floatValue, _ := strconv.ParseFloat(outputValue, 64)
			inputValue = fmt.Sprintf("%.2f", floatValue/rate)
			html = fmt.Sprintf(`<input type="text" name="inputValue" value="%s" class="text-right font-semibold bg-transparent outline-none">`, inputValue)
		} else {
			c.HTML(http.StatusBadRequest, "error.html", gin.H{"error": "Missing inputValue or outputValue parameter"})
			return
		}

		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
	})

	// Web
	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "trade.html", gin.H{
			"inputAssets":  tradableAssets("USDT"),
			"outputAssets": tradableAssets("FUSD"),
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

	/* router.POST("/connect", func(c *gin.Context) {
		// Handle the connection request here
		// You can perform any necessary validation or processing
		address := c.PostForm("address")

		// Return the HTML directly as a string
		c.String(http.StatusOK, `<button id="connectButton">Connected</button>`)
	}) */

	router.GET("/login", func(c *gin.Context) {
		c.HTML(http.StatusOK, "login.html", gin.H{})
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
