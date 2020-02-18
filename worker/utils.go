package worker

import (
	"encoding/base64"
	"fmt"
	"github.com/cloudflare/cfssl/log"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"

	pb "github.com/tonradar/ton-dice-web-server/proto"
)

func parseInMessage(m string) (*Bet, error) {
	msg, err := base64.StdEncoding.DecodeString(m)
	if err != nil {
		log.Errorf("message decode failed with %s\n", err)
		return nil, err
	}

	fmt.Println("MESSAGE:", string(msg))

	if len(msg) > 0 {
		parts := strings.Split(string(msg), ",")
		if len(parts) < 2 || len(parts) > 3 {
			return nil, fmt.Errorf("message is not valid")
		}

		rollUnder, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, err
		}
		if rollUnder < 2 || rollUnder > 96 {
			return nil, fmt.Errorf("roll under is not valid")
		}

		betID, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, err
		}

		refAddress := ""
		if len(parts) == 3 {
			refAddress = parts[2]
		}

		return &Bet{
			ID:         betID,
			RollUnder:  rollUnder,
			RefAddress: refAddress,
		}, nil
	}

	return nil, fmt.Errorf("message is not valid")
}

func parseOutMessage(m string) (*Bet, error) {
	msg, err := base64.StdEncoding.DecodeString(m)
	if err != nil {
		log.Errorf("message decode failed with %s\n", err)
		return nil, err
	}

	if len(msg) > 0 {
		r, _ := regexp.Compile(`ton777.io - lucky number (\d+) fell for betting with id (\d+)`)
		matches := r.FindStringSubmatch(string(msg))

		fmt.Println("MATCHES:", matches)

		randomRoll, _ := strconv.Atoi(matches[1])
		betID, _ := strconv.Atoi(matches[2])

		fmt.Println("RANDOM_ROLL:", randomRoll)
		fmt.Println("BET_ID:", betID)

		return &Bet{
			ID:         betID,
			RandomRoll: randomRoll,
		}, nil
	}

	return nil, fmt.Errorf("message is not valid")
}

func SavedTrxLt(fn string) (int, error) {
	data, err := ioutil.ReadFile(fn)
	if err != nil {
		return 0, err
	}

	savedTrxLt, err := strconv.Atoi(string(data))
	if err != nil {
		return 0, err
	}

	return savedTrxLt, nil
}

func BuildCreateBetRequest(bet *Bet) *pb.CreateBetRequest {
	return &pb.CreateBetRequest{
		GameId:        int32(bet.ID),
		PlayerAddress: bet.PlayerAddress,
		RefAddress:    bet.RefAddress,
		Amount:        int64(bet.Amount),
		RollUnder:     int32(bet.RollUnder),
		RandomRoll:    int32(bet.RandomRoll),
		PlayerPayout:  bet.PlayerPayout,
		Seed:          bet.Seed,
	}
}

func isSavedInStorage(bet *Bet) bool {
	return bet.IDInStorage > 0
}

func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
