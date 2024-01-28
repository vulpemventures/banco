package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"html/template"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/vulpemventures/go-elements/address"
	_ "modernc.org/sqlite"
)

func main() {
	// Set up Viper for configuration
	viper.AutomaticEnv()

	// Set default values
	viper.SetDefault("WEB_DIR", "web")
	viper.SetDefault("OCEAN_URL", "localhost:18000")
	viper.SetDefault("OCEAN_ACCOUNT_NAME", "default")
	viper.SetDefault("WATCH_INTERVAL_SECONDS", "-1")
	viper.SetDefault("NETWORK", "liquid")

	// Set up Logrus for logging
	log.SetFormatter(&log.TextFormatter{})
	log.SetOutput(os.Stdout)

	// Parse environment variables
	webDir := viper.GetString("WEB_DIR")
	oceanURL := viper.GetString("OCEAN_URL")
	oceanAccountName := viper.GetString("OCEAN_ACCOUNT_NAME")
	networkName := viper.GetString("NETWORK")
	watchInterval := viper.GetInt("WATCH_INTERVAL_SECONDS")

	// validate network
	net, ok := SupportedNetworks[networkName]
	if !ok {
		log.Fatalf("invalid network: %s", networkName)
	}

	// initialize database
	_, err := initDB()
	if err != nil {
		log.Fatal("connect to db: ", err)
	}

	// setup connection with wallet
	walletSvc, err := NewWalletService(oceanURL, oceanAccountName)
	if err != nil {
		log.Fatal("start wallet service: %w", err)
	}

	// new instance of an Esplora HTTP client
	esplora, err := NewEsplora(networkName)
	if err != nil {
		log.Fatal("esplora initialization error: %w", err)
	}

	// Start processing pending trades
	if watchInterval > 0 {
		// start watching
		go startWatching(func() {
			orders, err := fetchOrdersToFulfill()
			if err != nil {
				log.Error(fmt.Errorf("error in fetching orders: %w", err))
				return
			}

			for _, order := range orders {
				err = watchForTrades(order, walletSvc, esplora)
				if err != nil {
					log.Error(fmt.Errorf("error in fulfilling order of %f %s: ID %s : %w", float64(order.Output.Amount), order.Output.Asset, order.ID, err))
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
	router.LoadHTMLGlob(webDir + "/*")

	// API
	router.GET("/pair", func(c *gin.Context) {
		pair := c.Query("pair")
		tradeType := c.Query("type")

		log.Info(pair, tradeType)

		// Get the conversion rate and fee
		markets, err := GetMarketsWithLimits(c.Request.Context(), walletSvc)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": err.Error()})
			return
		}
		mkt := getTradingPair(markets, pair)
		if mkt == nil {
			c.String(http.StatusBadRequest, fmt.Sprintf("Invalid trading pair %s", pair))
			return
		}

		//rate, err := rates.MarketPrice(mkt.BaseAsset, mkt.QuoteAsset)

		limit := mkt.BuyLimit
		currency := mkt.BaseAsset
		precision := currencyToAsset[mkt.BaseAsset].Precision
		limitFractional := float64(limit) / math.Pow(10, float64(precision))
		if tradeType == "Sell" {
			limit = mkt.SellLimit
			currency = mkt.QuoteAsset
			precision = currencyToAsset[mkt.QuoteAsset].Precision
			limitFractional = float64(limit) / math.Pow(10, float64(precision))
		}

		// Return the output amount
		outputValueHTML := fmt.Sprintf(`<div id="pairBox" class="p-4 rounded-lg">
		<div class="mb-4">
			<label id="limitText" class="block text-sm font-medium text-gray-700">Limit <strong>%s</strong> %s</label>
		</div>
		<div class="mb-4">
			<label id="rateText" class="block text-sm font-medium text-gray-700">Rate <strong>%s</strong> %s</label>
		</div>
	</div>`, fmt.Sprint(limitFractional), currency, "N/A", "")

		// Return the HTML string
		c.String(http.StatusOK, outputValueHTML)
	})

	router.GET("/trade/preview", func(c *gin.Context) {
		// Get the input ticker, output ticker, and amount from the query parameters
		amountStr := c.Query("amount")
		pair := c.Query("pair")
		tradeType := c.Query("type")

		// Convert the input value to a float
		amount, err := strconv.ParseFloat(amountStr, 64)
		if err != nil {
			c.String(http.StatusBadRequest, "Invalid input value")
			return
		}

		// Get the conversion rate and fee
		markets := GetMarkets()
		mkt := getTradingPair(markets, pair)
		if mkt == nil {
			c.String(http.StatusBadRequest, "Invalid trading pair")
			return
		}

		rate, err := rates.MarketPrice(mkt.BaseAsset, mkt.QuoteAsset)
		if err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("error getting price from stream: %v", err))
			return
		}

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

		log.Infof("tradingPair: %s, tradeType: %s, amountStr: %s", tradingPair, tradeType, amountStr)

		// Convert the input value to a float
		amount, err := strconv.ParseFloat(amountStr, 64)
		if err != nil {
			c.String(http.StatusBadRequest, "Invalid input value")
			return
		}

		// Get the conversion rate and fee
		markets := GetMarkets()
		mkt := getTradingPair(markets, tradingPair)
		if mkt == nil {
			c.String(http.StatusBadRequest, "Invalid trading pair")
			return
		}

		price, err := rates.MarketPrice(mkt.BaseAsset, mkt.QuoteAsset)
		if err != nil {
			c.String(http.StatusInternalServerError, fmt.Sprintf("error getting price from stream: %v", err))
			return
		}

		feePercentage := rates.FeePercentage(mkt.BaseAsset, mkt.QuoteAsset)
		// Determine the input value, input currency, output value, and output currency based on the trade type
		var inputValue, outputValue float64
		var inputCurrency, outputCurrency string
		if tradeType == "Buy" {
			inputValue = amount * price * (1 + feePercentage/100) // Add the fee to the input value
			inputCurrency = mkt.QuoteAsset
			outputValue = amount
			outputCurrency = mkt.BaseAsset
		} else if tradeType == "Sell" { // Sell
			inputValue = amount
			inputCurrency = mkt.BaseAsset
			outputValue = amount * price * (1 - feePercentage/100) // Subtract the fee from the output value
			outputCurrency = mkt.QuoteAsset
		}

		log.Infof("inputValue: %v, outputValue: %v", inputValue, outputValue)

		order, err := NewOrder(traderScriptHex, inputCurrency, fmt.Sprintf("%v", inputValue), outputCurrency, fmt.Sprintf("%v", outputValue), price, net)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": err.Error()})
			return
		}

		err = saveOrder(order)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": err.Error()})
			return
		}

		script, err := address.ToOutputScript(order.Address)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": err.Error()})
			return
		}
		// Start watching the script and get the notification channel
		err = walletSvc.WatchScript(c.Request.Context(), hex.EncodeToString(script))
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": err.Error()})
			return
		}

		c.Redirect(http.StatusSeeOther, "/offer/"+order.ID)
	})

	router.GET("/", func(c *gin.Context) {
		markets, err := GetMarketsWithLimits(c.Request.Context(), walletSvc)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": err.Error()})
			return
		}
		c.HTML(http.StatusOK, "trade.html", gin.H{
			"markets": markets,
			"network": networkName,
		})
	})

	router.GET("/offer/address/:address", func(c *gin.Context) {
		addr := c.Params.ByName("address")

		ID, err := fetchOrderIDByAddress(addr)
		if err != nil {
			log.Error(err)
			c.HTML(http.StatusNotFound, "404.html", gin.H{})
			return
		}

		c.Redirect(http.StatusSeeOther, "/offer/"+ID)
	})

	router.GET("/offer/:id", func(c *gin.Context) {
		id := c.Params.ByName("id")

		order, status, err := fetchOrderByID(id)
		if err != nil {
			c.HTML(http.StatusNotFound, "404.html", gin.H{})
			return
		}

		script, err := address.ToOutputScript(order.Address)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		// Start watching the script and get the notification channel
		err = walletSvc.WatchScript(c.Request.Context(), hex.EncodeToString(script))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		transactions, err := getTransactionsForAddress(order.Address, networkName)
		if err != nil {
			c.HTML(http.StatusInternalServerError, "error.html", gin.H{"error": err.Error()})
			return
		}

		// Split transactions into confirmed and pending
		var confirmedTransactions, pendingTransactions []map[string]interface{}
		for _, tx := range transactions {
			transaction := map[string]interface{}{
				"txID":          tx.TxID,
				"txIDShort":     tx.TxID[:6] + "..." + tx.TxID[len(tx.TxID)-6:],
				"confirmed":     tx.Status.Confirmed,
				"date":          time.Unix(int64(tx.Status.BlockTime), 0).Format("2006-01-02 15:04:05"),
				"explorerTxURL": fmt.Sprintf("%s/tx/%s", EsploraURLs[networkName], tx.TxID),
			}

			if tx.Status.Confirmed {
				confirmedTransactions = append(confirmedTransactions, transaction)
			} else {
				pendingTransactions = append(pendingTransactions, transaction)
			}
		}

		inputCurrency := assetToCurrency[order.Input.Asset]
		outputCurrency := assetToCurrency[order.Output.Asset]
		date := order.Timestamp.Format("2006-01-02 15:04:05")
		c.HTML(http.StatusOK, "offer.html", gin.H{
			"id":                    order.ID,
			"address":               order.Address,
			"inputValue":            order.InputValue(),
			"inputCurrency":         inputCurrency,
			"outputValue":           order.OutputValue(),
			"outputCurrency":        outputCurrency,
			"confirmedTransactions": confirmedTransactions,
			"pendingTransactions":   pendingTransactions,
			"inputAssetHash":        order.Input.Asset,
			"inputAmount":           order.Input.Amount,
			"status":                status,
			"date":                  date,
		})
	})

	router.GET("/offer/:id/status", func(c *gin.Context) {

	})

	router.GET("/offer/:id/transactions", func(c *gin.Context) {
		notifChan, err := walletSvc.TransactionNotifications(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.Stream(func(w io.Writer) bool {
			var confirmedTransactions, pendingTransactions []map[string]interface{}

			// listen to the tx notification
			select {
			case <-c.Request.Context().Done():
				return false
			case event := <-notifChan:
				txid := event.TxId
				confirmed := event.Confirmed
				timestamp := event.Timestamp

				transaction := map[string]interface{}{
					"txID":          txid,
					"txIDShort":     txid[:6] + "..." + txid[len(txid)-6:],
					"confirmed":     confirmed,
					"date":          time.Unix(timestamp, 0).Format("2006-01-02 15:04:05"),
					"explorerTxURL": fmt.Sprintf("%s/tx/%s", EsploraURLs[networkName], txid),
				}

				if confirmed {
					confirmedTransactions = append(confirmedTransactions, transaction)
				} else {
					pendingTransactions = append(pendingTransactions, transaction)
				}

				data := map[string]interface{}{
					"confirmedTransactions": confirmedTransactions,
					"pendingTransactions":   pendingTransactions,
				}

				// Create a new template
				tmpl, err := template.ParseFiles(webDir + "/transactions.html")
				if err != nil {
					c.SSEvent("error", err.Error())
					return false
				}
				// Execute the template with the data and write the result to a string
				var html bytes.Buffer
				err = tmpl.Execute(&html, data)
				if err != nil {
					c.SSEvent("error", err.Error())
					return false
				}
				htmlStr := strings.ReplaceAll(html.String(), "\n", " ")
				// Send the notification to the client as SSE
				c.SSEvent("transactions", htmlStr)
			}
			return true
		})
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
