package store

import (
	"context"
	"database/sql"
	"lanvadip-bot/internal/model"
)

type MenuStore interface {
	GetAvailableMenu(ctx context.Context) ([]model.MenuItem, error)
}

type menuStore struct {
	db *sql.DB
}

func NewMenuStore(db *sql.DB) MenuStore {
	return &menuStore{db: db}
}

func (s *menuStore) GetAvailableMenu(ctx context.Context) ([]model.MenuItem, error) {
	query := `
		SELECT c.name, m.item_code, m.name, m.description, m.price_m, m.price_l 
		FROM menu_items m
		JOIN categories c ON m.category_id = c.id
		WHERE m.available = 1
	`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.MenuItem
	for rows.Next() {
		var item model.MenuItem
		var desc sql.NullString

		err := rows.Scan(&item.CategoryName, &item.ItemCode, &item.Name, &desc, &item.PriceM, &item.PriceL)
		if err != nil {
			return nil, err
		}

		item.Description = desc.String
		items = append(items, item)
	}

	return items, nil
}
