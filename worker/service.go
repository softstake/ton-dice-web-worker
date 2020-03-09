package worker

import (
	"bytes"
	"context"
	"fmt"
	"github.com/cloudflare/cfssl/log"
	api "github.com/tonradar/ton-api/proto"
	store "github.com/tonradar/ton-dice-web-server/proto"
	"google.golang.org/grpc"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
	"ton-dice-web-worker/config"
)

var (
	storageHost string
	storagePort string
	tonApiHost  string
	tonApiPort  string
)

func init() {
	storageHost = os.Getenv("STORAGE_HOST")
	storagePort = os.Getenv("STORAGE_PORT")
	tonApiHost = os.Getenv("TON_API_HOST")
	tonApiPort = os.Getenv("TON_API_PORT")

	if storageHost == "" || storagePort == "" || tonApiHost == "" || tonApiPort == "" {
		log.Fatal("Some of required ENV vars are empty. The vars are: STORAGE_HOST, STORAGE_PORT, TON_API_HOST, TON_API_PORT")
	}
}

type WorkerService struct {
	conf          config.Config
	mutex         *sync.RWMutex
	bets          map[int]*Bet
	storageClient store.BetsClient
	apiClient     api.TonApiClient
}

func NewWorkerService(conf config.Config) *WorkerService {
	opts := []grpc.DialOption{
		grpc.WithInsecure(),
	}

	conn, err := grpc.Dial(fmt.Sprintf("%s:%s", storageHost, storagePort), opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}

	client := store.NewBetsClient(conn)

	conn, err = grpc.Dial(fmt.Sprintf("%s:%s", tonApiHost, tonApiPort), opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}

	client2 := api.NewTonApiClient(conn)

	return &WorkerService{
		conf:          conf,
		mutex:         &sync.RWMutex{},
		bets:          make(map[int]*Bet, 100),
		storageClient: client,
		apiClient:     client2,
	}
}

func (s *WorkerService) GetBet(ID int) *Bet {
	s.mutex.RLock()
	bet, ok := s.bets[ID]
	s.mutex.RUnlock()
	if !ok {
		return nil
	}
	return bet
}

func (s *WorkerService) UpdateBet(bet *Bet) *Bet {
	fmt.Println("updating bet: id: %v", bet.ID)

	s.mutex.Lock()
	s.bets[bet.ID] = bet
	s.mutex.Unlock()

	return bet
}

func (s *WorkerService) resolveBet(resolved *Bet) (*Bet, bool) {
	isResolved := true
	bet := s.GetBet(resolved.ID)
	if bet == nil {
		isResolved = false
		bet = resolved
	}
	t := time.Now().UTC()
	bet.TimeResolved = &t
	bet.RandomRoll = resolved.RandomRoll
	bet.PlayerPayout = resolved.PlayerPayout
	s.mutex.Lock()
	s.bets[resolved.ID] = bet
	s.mutex.Unlock()

	return s.GetBet(resolved.ID), isResolved
}

func (s *WorkerService) ResolveBet(betId int, seqno string, seed string) error {
	fileNameWithPath := s.conf.Service.ResolveQuery
	fileNameStart := strings.LastIndex(fileNameWithPath, "/")
	fileName := fileNameWithPath[fileNameStart+1:]

	bocFile := strings.Replace(fileName, ".fif", ".boc", 1)

	_ = os.Remove(bocFile)

	var out bytes.Buffer
	cmd := exec.Command("fift", "-s", fileNameWithPath, s.conf.Service.KeyFileBase, s.conf.Service.ContractAddress, seqno, strconv.Itoa(betId), seed)

	cmd.Stderr = &out
	err := cmd.Run()
	if err != nil {
		log.Errorf("cmd.Run() failed with %s\n", err)
		return err
	}

	if FileExists(bocFile) {
		data, err := ioutil.ReadFile(bocFile)
		if err != nil {
			log.Error(err)
		}

		sendMessageRequest := &api.SendMessageRequest{
			Body: data,
		}

		sendMessageResponse, err := s.apiClient.SendMessage(context.Background(), sendMessageRequest)
		if err != nil {
			// need restart container
			panic(err)
		}

		fmt.Printf("ResolveBet: send message status: %v", sendMessageResponse.Ok)

		return nil
	}

	return fmt.Errorf("File not found, maybe fift compile failed?")
}

func (s *WorkerService) Run() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		for {
			getAccountStateRequest := &api.GetAccountStateRequest{
				AccountAddress: s.conf.Service.ContractAddress,
			}
			getAccountStateResponse, err := s.apiClient.GetAccountState(ctx, getAccountStateRequest)
			if err != nil {
				// need restart container
				panic(fmt.Sprintf("Error get account state: %v", err))
			}

			lt := getAccountStateResponse.LastTransactionId.Lt
			hash := getAccountStateResponse.LastTransactionId.Hash

			savedTrxLt, err := GetSavedTrxLt(s.conf.Service.SavedTrxLt)
			if err != nil {
				log.Errorf("Error get read saved trx time: %v", err)
				return
			}

			if lt <= int64(savedTrxLt) {
				//log.Warningf("saved trx time (%d) is greater than current (%d)", savedTrxLt, lt)
				continue
			}

			err = ioutil.WriteFile(s.conf.Service.SavedTrxLt, []byte(strconv.Itoa(int(lt))), 0644)
			if err != nil {
				log.Errorf("Error write trx time to file: %v", err)
				return
			}

			fetchTransactionsRequest := &api.FetchTransactionsRequest{
				Address: s.conf.Service.ContractAddress,
				Lt:      lt,
				Hash:    hash,
			}

			fetchTransactionsResponse, err := s.apiClient.FetchTransactions(ctx, fetchTransactionsRequest)
			if err != nil {
				// need restart container
				panic(fmt.Sprintf("failed to fetch transactions: %v", err))
			}
			transactions := fetchTransactionsResponse.Items

			for _, trx := range transactions {
				for _, outMsg := range trx.OutMsgs {
					// getting information about the results of the bet
					betInfo, err := parseOutMessage(outMsg.Message)
					if err != nil {
						log.Errorf("output message parse failed with %s\n", err)
						continue
					}
					betInfo.PlayerPayout = outMsg.Value
					// storing bet results information in-memory
					inMemoryBet, isResolved := s.resolveBet(betInfo)

					// if the bet information is not complete, skip it
					if inMemoryBet.RollUnder == 0 || inMemoryBet.Amount == 0 {
						continue
					}

					if !isSavedInStorage(inMemoryBet) && isResolved {
						// saving bet to the persistent storage
						req := BuildCreateBetRequest(inMemoryBet)
						resp, err := s.storageClient.CreateBet(ctx, req)
						if err != nil {
							log.Errorf("save bet in DB failed with %s\n", err)
							continue
						}
						fmt.Printf("bet with id %d successfully saved (date: %s)", resp.Id, resp.CreatedAt)
						inMemoryBet.IDInStorage = resp.Id
						s.UpdateBet(inMemoryBet)
					}
				}

				inMsg := trx.InMsg

				// getting information about a new bet
				bet, err := parseInMessage(inMsg.Message)
				if err != nil {
					log.Errorf("input message parse failed with %s\n", err)
					continue
				}

				// if there is bet information in-memory
				inMemoryBet := s.GetBet(bet.ID)
				if inMemoryBet != nil {
					// if the bet is stored in persistent storage, then skip it
					if isSavedInStorage(inMemoryBet) {
						continue
					}
					bet.RandomRoll = inMemoryBet.RandomRoll
					bet.PlayerPayout = inMemoryBet.PlayerPayout
					bet.TimeCreated = inMemoryBet.TimeCreated
				}

				bet.TrxHash = hash
				bet.TrxLt = lt

				playerAddress := inMsg.Source
				bet.PlayerAddress = playerAddress

				amount := inMsg.Value
				bet.Amount = int(amount)

				req := &api.GetBetSeedRequest{
					BetId: int64(bet.ID),
				}

				getBetSeedResponse, err := s.apiClient.GetBetSeed(ctx, req)
				if err != nil {
					// need restart container
					panic(fmt.Sprintf("failed to run GetBetSeed method: %v", err))
				}
				seed := getBetSeedResponse.Seed

				bet.Seed = seed
				bet = s.UpdateBet(bet)

				// if there is complete information about the bet, save it in a persistent storage
				if bet.RandomRoll > 0 {
					req := BuildCreateBetRequest(bet)
					resp, err := s.storageClient.CreateBet(ctx, req)
					if err != nil {
						log.Errorf("save bet in DB failed with %s\n", err)
						continue
					}
					fmt.Printf("bet with id %d successfully saved (date: %s)", resp.Id, resp.CreatedAt)

					bet.IDInStorage = resp.Id
					s.UpdateBet(bet)
				}

				getSeqnoResponse, err := s.apiClient.GetSeqno(ctx, &api.GetSeqnoRequest{})
				if err != nil {
					// need restart container
					panic(fmt.Sprintf("Error get seqno: %v", err))
				}

				err = s.ResolveBet(bet.ID, getSeqnoResponse.Seqno, seed)
				if err != nil {
					log.Errorf("failed to resolve bet with %s\n", err)
					continue
				}
			}

			time.Sleep(1000 * time.Millisecond)
		}
	}()

	select {
	case <-ctx.Done():
		fmt.Println("work is done")
		return
	}
}
