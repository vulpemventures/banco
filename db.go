package main

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/vulpemventures/go-elements/network"
)

const (
	sqliteAdapter  = "sqlite"
	sqliteFilename = "banco.db"
)

type OrderPostgresRow struct {
	ID            string `json:"id"`
	Timestamp     string `json:"timestamp"`
	FulfillScript []byte `json:"fulfill_script"`
	RefundScript  []byte `json:"refund_script"`
	TraderScript  []byte `json:"trader_script"`
	InputAsset    string `json:"input_asset"`
	InputAmount   uint64 `json:"input_amount"`
	OutputAsset   string `json:"output_asset"`
	OutputAmount  uint64 `json:"output_amount"`
	Address       string `json:"address"`
}

func initDB() (*sql.DB, error) {
	db, err := sql.Open(sqliteAdapter, sqliteFilename)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	_, err = db.Exec(`
			CREATE TABLE IF NOT EXISTS orders (
				id TEXT PRIMARY KEY,
				timestamp TEXT,
				address TEXT,
				fulfill_script BLOB,
				refund_script BLOB,
				trader_script BLOB,
				input_asset TEXT,
				input_amount INTEGER UNSIGNED,
				output_asset TEXT,
				output_amount INTEGER UNSIGNED
			)
		`)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func saveOrderToDB(order *Order) error {
	db, err := sql.Open(sqliteAdapter, sqliteFilename)
	if err != nil {
		return err
	}
	defer db.Close()
	// Convert time.Time to string
	timestampStr := order.Timestamp.Format("2006-01-02 15:04:05")

	_, err = db.Exec(`
			INSERT INTO orders (id, timestamp, fulfill_script, refund_script, trader_script, input_asset, input_amount, output_asset, output_amount, address)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, order.ID, timestampStr, order.FulfillScript, order.RefundScript, order.TraderScript, order.Input.Asset, order.Input.Amount, order.Output.Asset, order.Output.Amount, order.Address)
	if err != nil {
		return err
	}
	return nil
}

func fetchOrderFromDB(id string) (*Order, error) {
	db, err := sql.Open(sqliteAdapter, sqliteFilename)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	var order OrderPostgresRow
	row := db.QueryRow("SELECT id, timestamp, fulfill_script, refund_script, trader_script, input_asset, input_amount, output_asset, output_amount, address FROM orders WHERE id = ?", id)

	println(row)

	err = row.Scan(&order.ID, &order.Timestamp, &order.FulfillScript, &order.RefundScript, &order.TraderScript, &order.InputAsset, &order.InputAmount, &order.OutputAsset, &order.OutputAmount, &order.Address)
	if err != nil {
		return nil, err
	}

	timestamp, err := time.Parse("2006-01-02 15:04:05", order.Timestamp)
	if err != nil {
		return nil, err
	}

	paymentData, err := CreateFundingOutput(order.FulfillScript, order.RefundScript, &network.Testnet)
	if err != nil {
		return nil, fmt.Errorf("failed to create funding output: %w", err)
	}
	return &Order{
		ID:            order.ID,
		Timestamp:     timestamp,
		PaymentData:   paymentData,
		FulfillScript: order.FulfillScript,
		RefundScript:  order.RefundScript,
		TraderScript:  order.TraderScript,
		Input: struct {
			Asset  string
			Amount uint64
		}{
			Asset:  order.InputAsset,
			Amount: order.InputAmount,
		},
		Output: struct {
			Asset  string
			Amount uint64
		}{
			Asset:  order.OutputAsset,
			Amount: order.OutputAmount,
		},
		Address: order.Address,
	}, nil
}
