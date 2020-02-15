package worker

import (
	"context"
	"fmt"
	"github.com/cloudflare/cfssl/log"
	api "github.com/tonradar/ton-api/proto"
	store "github.com/tonradar/ton-dice-web-server/proto"
	"google.golang.org/grpc"
	"io/ioutil"
	"strconv"
	"sync"
	"time"
	"ton-dice-web-worker/config"
)

type WorkerService struct {
	conf          config.Config
	ton           *TonService
	mutex         *sync.RWMutex
	bets          map[int]*Bet
	storageClient store.BetsClient
	apiClient     api.TonApiClient
}

func NewWorkerService(conf config.Config) *WorkerService {
	opts := []grpc.DialOption{
		grpc.WithInsecure(),
	}

	conn, err := grpc.Dial("127.0.0.1:5300", opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}

	client := store.NewBetsClient(conn)

	conn, err = grpc.Dial("127.0.0.1:5400", opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}

	client2 := api.NewTonApiClient(conn)

	ton, err := NewTonService(conf)
	if err != nil {
		log.Errorf("error initialize TON service: %v", err)
	}

	return &WorkerService{
		conf:          conf,
		ton:           ton,
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

func (s *WorkerService) ResolveBet(resolved *Bet) (*Bet, bool) {
	isResolved := true
	bet := s.GetBet(resolved.ID)
	if bet == nil {
		isResolved = false
		bet = resolved
	}
	t := time.Now().UTC()
	bet.TimeResolved = &t
	bet.RandomRoll = resolved.RandomRoll
	s.mutex.Lock()
	s.bets[resolved.ID] = bet
	s.mutex.Unlock()

	return s.GetBet(resolved.ID), isResolved
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
				log.Errorf("Error get account state: %v", err)
				return
			}

			lt := getAccountStateResponse.LastTransactionId.Lt
			hash := getAccountStateResponse.LastTransactionId.Hash

			savedTrxLt, err := SavedTrxLt(s.conf.Service.SavedTrxLt)
			if err != nil {
				log.Errorf("Error get read saved trx time: %v", err)
				return
			}

			if lt <= int64(savedTrxLt) {
				log.Warningf("saved trx time (%d) is greater than current (%d)", savedTrxLt, lt)
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
				log.Warningf("failed to fetch transactions: %v", err)
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
					// storing bet results information in-memory
					inMemoryBet, isResolved := s.ResolveBet(betInfo)

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
					bet.TimeCreated = inMemoryBet.TimeCreated
				}

				playerAddress := inMsg.Source
				bet.PlayerAddress = playerAddress

				amount := inMsg.Value
				bet.Amount = int(amount)

				req := &api.GetBetSeedRequest{
					BetId: int64(bet.ID),
				}

				getBetSeedResponse, err := s.apiClient.GetBetSeed(ctx, req)
				if err != nil {
					log.Errorf("failed to run GetBetSeed method: %v", err)
					continue
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

				err = s.ton.ResolveBet(bet.ID, seed)
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
