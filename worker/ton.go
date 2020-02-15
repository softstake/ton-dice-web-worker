package worker

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/cloudflare/cfssl/log"
	"github.com/mercuryoio/tonlib-go"
	"io/ioutil"
	"os"
	_ "os"
	"os/exec"
	"strconv"
	"strings"

	"ton-dice-web-worker/config"
)

type TonService struct {
	conf config.Config
	api  *tonlib.Client
	lc   *LiteClient
}

func NewTonService(conf config.Config) (*TonService, error) {
	options, err := tonlib.ParseConfigFile(conf.Service.TonConfig)
	if err != nil {
		return nil, fmt.Errorf("Config file not found, error: %v", err)
	}

	req := tonlib.TonInitRequest{
		"init",
		*options,
	}

	client, err := tonlib.NewClient(&req, tonlib.Config{}, 5)
	if err != nil {
		return nil, fmt.Errorf("Init client error, error: %v", err)
	}
	liteClient, _ := NewLiteClient(conf)

	return &TonService{
		conf: conf,
		api:  client,
		lc:   liteClient,
	}, nil
}

// deprecated, see apiClient
func (s *TonService) FetchTransactions(lt int, hash string) []tonlib.RawTransaction {
	resp, err := s.api.RawGetTransactions(tonlib.NewAccountAddress(s.conf.Service.ContractAddress), tonlib.NewInternalTransactionId(hash, tonlib.JSONInt64(lt)))
	if err != nil {
		log.Errorf("get account trxs failed with %s\n", err)
		return nil
	}

	return resp.Transactions
}

func (s *TonService) ResolveBet(betId int, seed string) error {
	fileNameWithPath := s.conf.Service.ResolveQuery
	fileNameStart := strings.LastIndex(fileNameWithPath, "/")
	fileName := fileNameWithPath[fileNameStart+1:]

	bocFile := strings.Replace(fileName, ".fif", ".boc", 1)

	_ = os.Remove(bocFile)

	seqno, err := s.GetSeqno()
	if err != nil {
		return err
	}

	var out bytes.Buffer
	cmd := exec.Command("fift", "-s", fileNameWithPath, s.conf.Service.KeyFileBase, s.conf.Service.ContractAddress, strconv.Itoa(seqno), strconv.Itoa(betId), seed)

	cmd.Stderr = &out
	err = cmd.Run()
	if err != nil {
		log.Errorf("cmd.Run() failed with %s\n", err)
		return err
	}

	if FileExists(bocFile) {
		data, err := ioutil.ReadFile(bocFile)
		if err != nil {
			log.Error(err)
		}

		_, err = s.api.RawSendMessage(data)
		if err != nil {
			log.Error(err)
		}

		return nil
	}

	return fmt.Errorf("File not found, maybe fift compile failed?")

}

// deprecated, see apiClient
func (s *TonService) GetLastTrxLt() (int, error) {
	raw, err := s.lc.GetAccount(s.conf.Service.ContractAddress)
	if err != nil {
		return 0, err
	}
	str := string(raw)
	i := strings.Index(str, "last transaction lt")
	str = str[i:]
	str = strings.TrimPrefix(str, "last transaction lt = ")
	parts := strings.Split(str, " ")

	var lt int
	if parts[0] != "" {
		lt, err = strconv.Atoi(parts[0])
		if err != nil {
			return 0, err
		}
	}

	return lt, nil
}

// deprecated, see apiClient
func (s *TonService) GetLastTrxHash() (string, error) {
	raw, err := s.lc.GetAccount(s.conf.Service.ContractAddress)
	if err != nil {
		return "", err
	}
	str := string(raw)
	i := strings.Index(str, "hash")
	str = str[i:]
	str = strings.TrimPrefix(str, "hash = ")

	decoded, err := hex.DecodeString(str)

	var hash string
	if str != "" {
		hash = base64.StdEncoding.EncodeToString(decoded)
	}

	return hash, nil
}

// deprecated, see apiClient
func (s *TonService) GetSeqno() (int, error) {
	raw, err := s.lc.RunMethod(s.conf.Service.ContractAddress, "get_seqno")
	if err != nil {
		return 0, err
	}
	str := string(raw)
	i := strings.Index(str, "result")
	str = str[i:]
	str = strings.TrimPrefix(str, "result:")
	str = strings.Replace(str, "[", "", 1)
	str = strings.Replace(str, "]", "", 1)
	str = strings.TrimSpace(str)

	var seqno int
	if str != "" {
		seqno, err = strconv.Atoi(str)
		if err != nil {
			return 0, err
		}
	}

	return seqno, nil
}

// deprecated, see apiClient
func (s *TonService) GetSeed(betID int) (string, error) {
	raw, err := s.lc.RunMethod(s.conf.Service.ContractAddress, fmt.Sprintf("get_bet_seed %d", betID))
	if err != nil {
		return "", err
	}
	str := string(raw)
	i := strings.Index(str, "result")
	str = str[i:]
	str = strings.TrimPrefix(str, "result:")
	str = strings.Replace(str, "[", "", 1)
	str = strings.Replace(str, "]", "", 1)
	str = strings.TrimSpace(str)

	//var seed int
	//if str != "" {
	//	seed, err = strconv.Atoi(str)
	//	if err != nil {
	//		return 0, err
	//	}
	//}

	return str, nil
}
