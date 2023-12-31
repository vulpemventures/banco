package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
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

	if watchInterval > 0 {
		// start watching
		log.Println("Watcher not implemented yet")
	}

	_, err := initDB()
	if err != nil {
		log.Fatal("connectToDB: ", err)
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
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": err.Error()})
			return
		}

		err = saveOrderToDB(order)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": err.Error()})
			return
		}

		c.Redirect(http.StatusSeeOther, "/offer/"+order.ID)
	})

	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "trade.html", gin.H{})
	})

	router.GET("/offer/:id", func(c *gin.Context) {
		id := c.Params.ByName("id")

		order, err := fetchOrderFromDB(id)
		if err != nil {
			log.Println(err.Error())
			c.HTML(http.StatusNotFound, "404.html", gin.H{"error": err.Error()})
			return
		}

		err = watchForTrades(order, oceanURL)
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
