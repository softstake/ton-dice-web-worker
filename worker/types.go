package worker

import "time"

type Bet struct {
	ID             int
	PlayerAddress  string
	RefAddress     string
	Amount         int
	RollUnder      int
	RandomRoll     int
	Seed           string
	Signature      string
	PlayerPayout   int64
	RefPayout      int64
	IDInStorage    int64
	TimeCreated    *time.Time
	CreateTrxHash  string
	CreateTrxLt    int64
	TimeResolved   *time.Time
	ResolveTrxHash string
	ResolveTrxLt   int64
}
