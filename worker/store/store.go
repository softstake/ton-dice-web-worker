package store

import "time"

type Store interface {
	Get(string) (*Bet, error)
	Set(string, *Bet) error
}
