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

type OrderAndStatusRow struct {
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
	Status        string `json:"status"`
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

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS orders (
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
	)`)
	if err != nil {
		return nil, fmt.Errorf("create table orders: %w", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS order_statuses (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    order_id TEXT,
    status TEXT CHECK(status IN ('Pending', 'Funded', 'Fulfilled', 'Cancelled', 'Expired')),
    timestamp TEXT,
    tx_hash TEXT,
    FOREIGN KEY(order_id) REFERENCES orders(id)
	)`)
	if err != nil {
		return nil, fmt.Errorf("create table order_statuses: %w", err)
	}

	return db, nil
}

func saveOrder(order *Order) error {
	db, err := sql.Open(sqliteAdapter, sqliteFilename)
	if err != nil {
		return err
	}
	defer db.Close()

	// Start a transaction
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	// Convert time.Time to UTC string
	timestampStr := order.Timestamp.UTC().Format("2006-01-02 15:04:05")

	// Prepare the first INSERT statement
	insertOrderStmt, err := tx.Prepare(`
		INSERT INTO orders (id, timestamp, fulfill_script, refund_script, trader_script, input_asset, input_amount, output_asset, output_amount, address)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer insertOrderStmt.Close()

	// Execute the first INSERT statement
	_, err = insertOrderStmt.Exec(order.ID, timestampStr, order.FulfillScript, order.RefundScript, order.TraderScript, order.Input.Asset, order.Input.Amount, order.Output.Asset, order.Output.Amount, order.Address)
	if err != nil {
		tx.Rollback()
		return err
	}

	// Prepare the second INSERT statement
	insertStatusStmt, err := tx.Prepare(`
		INSERT INTO order_statuses (order_id, status, timestamp)
		VALUES (?, 'Pending', ?)
	`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer insertStatusStmt.Close()

	// Execute the second INSERT statement
	_, err = insertStatusStmt.Exec(order.ID, timestampStr)
	if err != nil {
		tx.Rollback()
		return err
	}

	// Commit the transaction
	err = tx.Commit()
	if err != nil {
		tx.Rollback()
		return err
	}

	return nil
}

func updateOrderStatus(id string, status string) error {
	db, err := sql.Open(sqliteAdapter, sqliteFilename)
	if err != nil {
		return err
	}
	defer db.Close()

	// Convert time.Now() to string
	timestampStr := time.Now().Format("2006-01-02 15:04:05")

	_, err = db.Exec(`
			UPDATE order_statuses
			SET status = ?, timestamp = ?
			WHERE order_id = ?
		`, status, timestampStr, id)
	if err != nil {
		return err
	}

	return nil
}

func fetchOrdersToFulfill() ([]*Order, error) {
	db, err := sql.Open(sqliteAdapter, sqliteFilename)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`
    SELECT o.id, o.timestamp, o.fulfill_script, o.refund_script, o.trader_script, o.input_asset, o.input_amount, o.output_asset, o.output_amount, o.address
    FROM orders o
    JOIN order_statuses os ON o.id = os.order_id
		WHERE os.status = 'Pending' OR os.status = 'Funded'
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*Order
	for rows.Next() {
		var order OrderAndStatusRow
		err := rows.Scan(&order.ID, &order.Timestamp, &order.FulfillScript, &order.RefundScript, &order.TraderScript, &order.InputAsset, &order.InputAmount, &order.OutputAsset, &order.OutputAmount, &order.Address)
		if err != nil {
			if err == sql.ErrNoRows {
				// Handle no rows in result set, for example by returning an empty slice
				return []*Order{}, nil
			} else {
				// Handle other errors
				return nil, err
			}
		}

		timestamp, err := time.Parse("2006-01-02 15:04:05", order.Timestamp)
		if err != nil {
			return nil, err
		}

		paymentData, err := CreateFundingOutput(order.FulfillScript, order.RefundScript, &network.Testnet)
		if err != nil {
			return nil, fmt.Errorf("failed to create funding output: %w", err)
		}

		orders = append(orders, &Order{
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
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return orders, nil
}

func fetchOrderIDByAddress(address string) (string, error) {
	db, err := sql.Open(sqliteAdapter, sqliteFilename)
	if err != nil {
		return "", err
	}
	defer db.Close()
	var ID string
	query := `SELECT o.id
		FROM orders o 
		WHERE o.address = ?`
	row := db.QueryRow(query, address)
	err = row.Scan(&ID)
	if err != nil {
		return "", err
	}

	return ID, nil
}

func fetchOrderByID(id string) (*Order, string, error) {
	db, err := sql.Open(sqliteAdapter, sqliteFilename)
	if err != nil {
		return nil, "", err
	}
	defer db.Close()
	var order OrderAndStatusRow
	query := `SELECT o.id, o.timestamp, o.fulfill_script, o.refund_script, o.trader_script, 
				o.input_asset, o.input_amount, o.output_asset, o.output_amount, o.address, s.status 
				FROM orders o 
				JOIN order_statuses s ON o.id = s.order_id 
				WHERE o.id = ?`
	row := db.QueryRow(query, id)
	err = row.Scan(&order.ID, &order.Timestamp, &order.FulfillScript, &order.RefundScript, &order.TraderScript, &order.InputAsset, &order.InputAmount, &order.OutputAsset, &order.OutputAmount, &order.Address, &order.Status)
	if err != nil {
		return nil, "", err
	}

	timestamp, err := time.Parse("2006-01-02 15:04:05", order.Timestamp)
	if err != nil {
		return nil, "", err
	}

	paymentData, err := CreateFundingOutput(order.FulfillScript, order.RefundScript, &network.Testnet)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create funding output: %w", err)
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
	}, order.Status, nil
}
