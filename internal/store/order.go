package store

import (
	"context"
	"database/sql"
	"fmt"
	"lanvadip-bot/internal/model"
	"time"
)

type OrderStore interface {
	CreateOrder(ctx context.Context, orderCode int64, userID int64, total int, items []model.OrderItem) error
	UpdateOrderStatus(ctx context.Context, orderCode int64, status string) error
}

type orderStore struct {
	db *sql.DB
}

func NewOrderStore(db *sql.DB) OrderStore {
	return &orderStore{db: db}
}

func (s *orderStore) CreateOrder(ctx context.Context, orderCode int64, userID int64, total int, items []model.OrderItem) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	queryOrder := `INSERT INTO orders (order_code, user_id, total_amount, status, created_at) VALUES (?, ?, ?, 'PENDING', ?)`
	result, err := tx.ExecContext(ctx, queryOrder, orderCode, userID, total, time.Now())
	if err != nil {
		return fmt.Errorf("Error create order: %w", err)
	}

	orderID, _ := result.LastInsertId()

	queryItem := `INSERT INTO order_items (order_id, item_code, size, quantity, price) VALUES (?, ?, ?, ?, ?)`
	for _, item := range items {
		_, err = tx.ExecContext(ctx, queryItem, orderID, item.ItemCode, item.Size, item.Quantity, item.Price)
		if err != nil {
			return fmt.Errorf("Error create order item: %w", err)
		}
	}

	return tx.Commit()
}

func (s *orderStore) UpdateOrderStatus(ctx context.Context, orderCode int64, status string) error {
	query := `UPDATE orders SET status = ? WHERE order_code = ?`
	_, err := s.db.ExecContext(ctx, query, status, orderCode)
	return err
}
