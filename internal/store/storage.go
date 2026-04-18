package store

import (
	"database/sql"

	"github.com/redis/go-redis/v9"
)

type Storage struct {
	FSM  FSMStore
	Menu MenuStore
}

func NewStorage(client *redis.Client, db *sql.DB) Storage {
	return Storage{
		FSM:  NewRedisFSMStore(client),
		Menu: NewMenuStore(db),
	}
}
