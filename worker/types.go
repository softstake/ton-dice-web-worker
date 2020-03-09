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
	PlayerPayout  int64
	RefPayout     int64
	TimeCreated   *time.Time
	TimeResolved  *time.Time
	IDInStorage   int64
	TrxHash       string
	TrxLt         int64
}
