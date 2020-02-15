package worker

import "time"

type Bet struct {
	ID            int
	PlayerAddress string
	RefAddress    string
	Amount        int
	RollUnder     int
	RandomRoll    int
	Seed          string
	Signature     string
	PlayerPayout  float32
	RefPayout     float32
	TimeCreated   *time.Time
	TimeResolved  *time.Time
	IDInStorage   int64
}
