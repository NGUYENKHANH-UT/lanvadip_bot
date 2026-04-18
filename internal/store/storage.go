package store

import "github.com/redis/go-redis/v9"

type Storage struct {
	FSM FSMStore
}

func NewStorage(client *redis.Client) Storage {
	return Storage{
		FSM: NewRedisFSMStore(client),
	}
}
