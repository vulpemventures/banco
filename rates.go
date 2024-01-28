package main

import (
	"fmt"
	"log"
	"time"

	ws "github.com/aopoltorzhicky/go_kraken/websocket"
)

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
	PriceStreams map[string]chan float64
	LastPrices   map[string]float64
}

func NewKrakenClient() *KrakenClient {
	kraken := ws.NewKraken(ws.ProdBaseURL)
	if err := kraken.Connect(); err != nil {
		log.Fatalf("Error connecting to web socket: %s", err.Error())
	}
	return &KrakenClient{
		ws:           kraken,
		PriceStreams: make(map[string]chan float64),
		LastPrices:   make(map[string]float64),
	}
}

func (kc *KrakenClient) FeePercentage(base, quote string) float64 {
	// Set a fixed fee percentage
	const feePercentage = 1 // 1% fee
	return feePercentage
}

func (kc *KrakenClient) MarketPrice(base, quote string) (float64, error) {
	marketPair := base + "/" + quote
	price, ok := kc.LastPrices[marketPair]
	if !ok {
		return 0, fmt.Errorf("no price available for market pair: %s", marketPair)
	}
	return price, nil
}

func (kc *KrakenClient) Subscribe() error {
	// Initialize the map
	kc.PriceStreams = make(map[string]chan float64)

	// Create channels to stream the market prices
	kc.PriceStreams["L-BTC/USDT"] = make(chan float64)
	kc.PriceStreams["L-BTC/L-BTC"] = make(chan float64)

	// Start a goroutine for each market pair to read from the WebSocket and update the last price
	for marketPair, priceStream := range kc.PriceStreams {
		go func(marketPair string, priceStream chan float64) {
			for price := range priceStream {
				kc.LastPrices[marketPair] = price
			}
		}(marketPair, priceStream)
	}

	// XBT/USDT
	// Subscribe to ticker information for the trading pair
	if err := kc.ws.SubscribeTicker([]string{ws.BTCUSDT}); err != nil {
		return fmt.Errorf("SubscribeTicker error: %s", err.Error())
	}

	go func() {
		defer close(kc.PriceStreams["L-BTC/USDT"])

		for update := range kc.ws.Listen() {
			switch data := update.Data.(type) {
			case ws.TickerUpdate:
				price, err := data.Ask.Price.Float64()
				if err != nil {
					log.Println("Error parsing price:", err)
					continue
				}
				kc.PriceStreams["L-BTC/USDT"] <- price
			default:
				return
			}
		}
	}()

	// L-BTC/L-BTC
	go func() {
		for {
			kc.PriceStreams["L-BTC/L-BTC"] <- 1.00
			time.Sleep(1 * time.Second)
		}
	}()
	return nil
}
