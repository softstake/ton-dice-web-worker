package worker

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/joaojeronimo/go-crc16"
	api "github.com/tonradar/ton-api/proto"
	pb "github.com/tonradar/ton-dice-web-server/proto"
	"io/ioutil"
	"log"
	"math/big"
	"os"
	"regexp"
	"strconv"
)

func parseOutMessage(m string) (*GameResult, error) {
	log.Println("Start parsing an outgoing message...")

	msg, err := base64.StdEncoding.DecodeString(m)
	if err != nil {
		log.Printf("Мessage decode failed: %v\n", err)
		return nil, err
	}

	log.Printf("Decoded message - '%s'", string(msg))

	if len(msg) > 0 {
		r, _ := regexp.Compile(`TONBET.IO - lucky number (\d+) fell for betting with id (\d+)`)
		matches := r.FindStringSubmatch(string(msg))

		if len(matches) > 0 {
			randomRoll, _ := strconv.Atoi(matches[1])
			betID, _ := strconv.Atoi(matches[2])

			return &GameResult{Id: betID, RandomRoll: randomRoll}, nil
		}
		log.Println("Message does not match expected pattern")
	} else {
		log.Println("Message is empty")
	}

	return nil, fmt.Errorf("message is not valid")
}

func BuildSaveBetRequest(bet *api.ActiveBet) (*pb.SaveBetRequest, error) {
	i := new(big.Int)
	i.SetString(bet.PlayerAddress.Address, 10)
	hexPlayerAddr := fmt.Sprintf("%x", i)

	playerAddr, err := packSmcAddr(int8(bet.PlayerAddress.Workchain), hexPlayerAddr, true, true)
	if err != nil {
		return nil, err
	}

	i = new(big.Int)
	i.SetString(bet.RefAddress.Address, 10)
	hexRefAddr := fmt.Sprintf("%x", i)

	refAddr, err := packSmcAddr(int8(bet.RefAddress.Workchain), hexRefAddr, true, true)
	if err != nil {
		return nil, err
	}

	if bet.Amount == 0 || bet.RollUnder == 0 || bet.Seed == "" {
		return nil, fmt.Errorf("invalid bet")
	}

	return &pb.SaveBetRequest{
		Id:            bet.Id,
		PlayerAddress: playerAddr,
		RefAddress:    refAddr,
		Amount:        bet.Amount,
		RollUnder:     bet.RollUnder,
		Seed:          bet.Seed,
	}, nil
}

func packSmcAddr(wc int8, hexAddr string, bounceble bool, testnet bool) (string, error) {
	_hexAddr := hexAddr
	if len(hexAddr) < 64 {
		_hexAddr = fmt.Sprintf("%064s", hexAddr)
	}

	tag := 0x11 // for "bounceable" addresses
	if !bounceble {
		tag = 0x51 // for "non-bounceable"
	}
	if testnet {
		tag += 0x80 // if the address should not be accepted by software running in the production network
	}

	var x []byte
	_tag, err := hex.DecodeString(fmt.Sprintf("%x", tag)) // one tag byte
	if err != nil {
		panic(err)
	}
	x = append(x, _tag...)

	_wc := []byte{0x00} // for the basic workchain
	if wc != 0 {
		// one byte containing a signed 8-bit integer with the workchain_id
		tmp, err := hex.DecodeString(strconv.FormatUint(uint64(wc), 16))
		if err != nil {
			panic(err)
		}
		_wc = []byte{tmp[0]}
	}
	x = append(x, _wc[0])

	_addr, err := hex.DecodeString(_hexAddr) // 32 bytes containing 256 bits of the smart-contract address inside the workchain (big-endian)
	if err != nil {
		panic(err)
	}
	x = append(x, _addr...)

	crc := crc16.Crc16(x) // 2 bytes containing CRC16-CCITT of the previous 34 bytes

	crcFix := fmt.Sprintf("%x", crc)
	_crc, err := hex.DecodeString(fmt.Sprintf("%04s", crcFix))
	if err != nil {
		panic(err)
	}
	x = append(x, _crc...)

	encoded := base64.URLEncoding.EncodeToString(x)
	return encoded, nil
}

func GetSavedTrxLt(fn string) (int, error) {
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

func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
