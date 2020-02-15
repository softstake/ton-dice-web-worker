package store

import (
	"encoding/json"
	"fmt"

	"github.com/go-redis/redis/v7"
	log "github.com/sirupsen/logrus"
)

type redisStore struct {
	client *redis.Client
}

func NewRedisStore() Store {
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	_, err := client.Ping().Result()
	if err != nil {
		log.Fatalf("Failed to ping Redis: %v", err)
	}

	return &redisStore{
		client: client,
	}
}

func (r redisStore) Set(identity string, bet *Bet) error {
	bs, err := json.Marshal(&bet)
	if err != nil {
		return fmt.Errorf("failed to save bet to redis: %s", err)
	}

	if err := r.client.Set(identity, bs, 0).Err(); err != nil {
		return fmt.Errorf("failed to save bet to redis: %s", err)
	}

	return nil
}

func (r redisStore) Get(identity string) (*Bet, error) {
	var bet Bet

	bs, err := r.client.Get(identity).Bytes()
	if err != nil {
		return &bet, fmt.Errorf("failed to get bet from redis: %s", err)
	}

	if err := json.Unmarshal(bs, &bet); err != nil {
		return &bet, fmt.Errorf("failed to unmarshall bet data: %s", err)
	}

	return &bet, nil
}
