package main

import (
	"fmt"
	"log"
	"time"

	ws "github.com/aopoltorzhicky/go_kraken/websocket"
)

var CurrencyToSymbol = map[string]string{
	"USDT":  "USD",
	"FUSD":  "USD",
	"L-BTC": "BTC",
}

type MarketWithStream struct {
	market    Market
	priceChan chan float64
}
type RatesClient interface {
	Subscribe() error
	MarketPrice(base, quote string) (float64, error)
	FeePercentage(base, quote string) float64
}

type Rates struct {
	Client RatesClient
}

type KrakenClient struct {
	ws           *ws.Kraken
	quitChan     chan bool
	PriceStreams map[string]chan float64
}

func NewKrakenClient() *KrakenClient {
	kraken := ws.NewKraken(ws.ProdBaseURL)
	if err := kraken.Connect(); err != nil {
		log.Fatalf("Error connecting to web socket: %s", err.Error())
	}
	return &KrakenClient{ws: kraken}
}

func (kc *KrakenClient) FeePercentage(base, quote string) float64 {
	// Set a fixed fee percentage
	const feePercentage = 1 // 1% fee
	return feePercentage
}

func (kc *KrakenClient) MarketPrice(base, quote string) (float64, error) {

	priceStream, ok := kc.PriceStreams[base+"/"+quote]
	if !ok {
		return 0, fmt.Errorf("no price stream available for market pair: %s/%s", base, quote)
	}
	// Read the first price from the stream with a timeout
	select {
	case price := <-priceStream:
		return price, nil
	case <-time.After(5 * time.Second): // adjust the timeout as needed
		return 0, fmt.Errorf("timeout waiting for price %s", base+"/"+quote)
	}
}

func (kc *KrakenClient) Subscribe() error {
	// Initialize the map
	kc.PriceStreams = make(map[string]chan float64)

	// Create channels to stream the market prices
	kc.PriceStreams["FUSD/USDT"] = make(chan float64)
	kc.PriceStreams["L-BTC/USDT"] = make(chan float64)

	// FUSD/USDT
	// For now we fix the exchange rate to 1:1
	go func() {
		for {
			kc.PriceStreams["FUSD/USDT"] <- 1.00
			time.Sleep(1 * time.Second)
		}
	}()

	// XBT/USDT
	// Subscribe to ticker information for the trading pair
	if err := kc.ws.SubscribeTicker([]string{ws.BTCUSDT}); err != nil {
		return fmt.Errorf("SubscribeTicker error: %s", err.Error())
	}

	go func() {
		defer close(kc.PriceStreams["L-BTC/USDT"])

		for range kc.ws.Listen() {
			select {
			case <-kc.quitChan:
				return
			case update := <-kc.ws.Listen():
				switch data := update.Data.(type) {
				case ws.TickerUpdate:
					price, err := data.Close.Today.Float64()
					if err != nil {
						log.Println("Error parsing price:", err)
						continue
					}
					kc.PriceStreams["L-BTC/USDT"] <- price
				default:
					return
				}
			}
		}
	}()

	return nil
}
