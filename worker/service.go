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
	fetcher       *Fetcher
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

	fetcher := NewFetcher(conf)

	return &WorkerService{
		conf:          conf,
		fetcher:       fetcher,
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

func (s *WorkerService) RemoveBet(ID int) {
	fmt.Println("removing bet: id: %v", ID)

	s.mutex.Lock()
	delete(s.bets, ID)
	s.mutex.Unlock()
}

func (s *WorkerService) isBetFetched(ctx context.Context, bet *Bet) (*store.IsBetFetchedResponse, error) {
	isBetFetchedReq := &store.IsBetFetchedRequest{
		GameId:        int32(bet.ID),
		CreateTrxHash: bet.CreateTrxHash,
		CreateTrxLt:   bet.CreateTrxLt,
	}

	resp, err := s.storageClient.IsBetFetched(ctx, isBetFetchedReq)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *WorkerService) isBetResolved(ctx context.Context, bet *Bet) (*store.IsBetResolvedResponse, error) {
	isBetResolvedReq := &store.IsBetResolvedRequest{
		GameId:         int32(bet.ID),
		ResolveTrxHash: bet.ResolveTrxHash,
		ResolveTrxLt:   bet.ResolveTrxLt,
	}

	resp, err := s.storageClient.IsBetResolved(ctx, isBetResolvedReq)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *WorkerService) ResolveQuery(betId int, seed string) error {
	fileNameWithPath := s.conf.Service.ResolveQuery
	fileNameStart := strings.LastIndex(fileNameWithPath, "/")
	fileName := fileNameWithPath[fileNameStart+1:]

	bocFile := strings.Replace(fileName, ".fif", ".boc", 1)

	_ = os.Remove(bocFile)

	var out bytes.Buffer
	cmd := exec.Command("fift", "-s", fileNameWithPath, s.conf.Service.KeyFileBase, s.conf.Service.ContractAddress, strconv.Itoa(betId), seed)

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
			//panic(err)
			log.Errorf("failed ResolveQuery method with: %v", err)
			return err
		}

		fmt.Printf("ResolveBet: send message status: %v", sendMessageResponse.Ok)

		return nil
	}

	return fmt.Errorf("File not found, maybe fift compile failed?")
}

func (s *WorkerService) ProcessBets(ctx context.Context, lt int64, hash string, depth int) (int64, string) {
	fetchTransactionsRequest := &api.FetchTransactionsRequest{
		Address: s.conf.Service.ContractAddress,
		Lt:      lt,
		Hash:    hash,
	}

	fetchTransactionsResponse, err := s.apiClient.FetchTransactions(ctx, fetchTransactionsRequest)
	if err != nil {
		panic(fmt.Sprintf("failed to fetch transactions: %v", err))
	}

	transactions := fetchTransactionsResponse.Items
	var trx *api.Transaction

	for _, trx = range transactions {
		for _, outMsg := range trx.OutMsgs {
			// getting information about the results of the bet
			bet, err := parseOutMessage(outMsg.Message)
			if err != nil {
				log.Errorf("output message parse failed with %s\n", err)
				continue
			}
			bet.PlayerPayout = outMsg.Value
			bet.ResolveTrxHash = trx.TransactionId.Hash
			bet.ResolveTrxLt = trx.TransactionId.Lt

			isResolved, err := s.isBetResolved(ctx, bet)
			if err != nil {
				log.Error(err)
				continue
			}

			if !isResolved.Yes {
				// If the game params are already known, then we update bet in the database
				// Otherwise, save to memory

				inMemoryBet := s.GetBet(bet.ID)
				if inMemoryBet != nil && inMemoryBet.IDInStorage != 0 {
					bet.IDInStorage = inMemoryBet.IDInStorage
					req, err := BuildUpdateBetRequest(bet)
					if err != nil {
						log.Errorf("Fetch method failed: %v", err)
						continue
					}
					resp, err := s.storageClient.UpdateBet(ctx, req)
					if err != nil {
						log.Errorf("update bet in DB failed with %s\n", err)
						continue
					}
					fmt.Printf("bet with id %d successfully updated (date: %s)", bet.ID, resp.ResolvedAt)

					s.RemoveBet(bet.ID)
				} else {
					s.UpdateBet(bet)
				}
			}
		}

		inMsg := trx.InMsg

		// getting information about a new bet
		bet, err := parseInMessage(inMsg.Message)
		if err != nil {
			log.Errorf("input message parse failed with %s\n", err)
			continue
		}

		bet.CreateTrxHash = trx.TransactionId.Hash
		bet.CreateTrxLt = trx.TransactionId.Lt
		bet.PlayerAddress = inMsg.Source
		bet.Amount = int(inMsg.Value)

		isFetchedResponse, err := s.isBetFetched(ctx, bet)
		if err != nil {
			log.Error(err)
			continue
		}

		if !isFetchedResponse.Yes {
			// Create bet in DB:

			req, err := BuildCreateBetRequest(bet)
			if err != nil {
				log.Errorf("Fetch method failed: %v", err)
				continue
			}
			resp, err := s.storageClient.CreateBet(ctx, req)
			if err != nil {
				log.Errorf("save bet in DB failed with %s\n", err)
				continue
			}

			bet.IDInStorage = resp.Id
			// Get bet seed from TON:

			getBetSeedReq := &api.GetBetSeedRequest{
				BetId: int64(bet.ID),
			}
			getBetSeedResponse, err := s.apiClient.GetBetSeed(ctx, getBetSeedReq)
			if err != nil {
				log.Errorf("failed to run GetBetSeed method: %v", err)
				continue
				//panic(fmt.Sprintf("failed to run GetBetSeed method: %v", err))
			}
			bet.Seed = getBetSeedResponse.Seed

			// Resolve bet in TON:

			err = s.ResolveQuery(bet.ID, bet.Seed)
			if err != nil {
				log.Errorf("failed to resolve bet with %s\n", err)
				continue
			}
		} else {
			bet.IDInStorage = isFetchedResponse.Id
		}

		// If the game results are already known, then we update bet in the database
		// Otherwise, save to memory
		inMemoryBet := s.GetBet(bet.ID)
		if inMemoryBet != nil {
			inMemoryBet.IDInStorage = bet.IDInStorage
			req, err := BuildUpdateBetRequest(inMemoryBet)
			if err != nil {
				log.Errorf("Fetch method failed: %v", err)
				continue
			}
			resp, err := s.storageClient.UpdateBet(ctx, req)
			if err != nil {
				log.Errorf("update bet in DB failed with %s\n", err)
				continue
			}
			fmt.Printf("bet with id %d successfully updated (date: %s)", resp.Id, resp.ResolvedAt)

			s.RemoveBet(inMemoryBet.ID)
		} else {
			s.UpdateBet(bet)
		}
	}

	_lt := lt
	_hash := hash
	if len(transactions) > 0 {
		_lt = trx.TransactionId.Lt
		_hash = trx.TransactionId.Hash
		if depth > 0 {
			depth -= 1
			time.Sleep(1000 * time.Millisecond)
			return s.ProcessBets(ctx, _lt, _hash, depth)
		}
	}

	return _lt, _hash
}

func (s *WorkerService) Run() {
	go s.fetcher.Start()
	//for {
	//	getAccountStateRequest := &api.GetAccountStateRequest{
	//		AccountAddress: s.conf.Service.ContractAddress,
	//	}
	//	getAccountStateResponse, err := s.apiClient.GetAccountState(ctx, getAccountStateRequest)
	//	if err != nil {
	//		log.Errorf("failed GetAccountState with error: %v", err)
	//		continue
	//		// need restart container
	//		// panic(fmt.Sprintf("Error get account state: %v", err))
	//	}
	//
	//	lt := getAccountStateResponse.LastTransactionId.Lt
	//	hash := getAccountStateResponse.LastTransactionId.Hash
	//
	//	savedTrxLt, err := GetSavedTrxLt(s.conf.Service.SavedTrxLt)
	//	if err != nil {
	//		log.Errorf("Error get read saved trx time: %v", err)
	//		return
	//	}
	//
	//	if lt > int64(savedTrxLt) {
	//		err = ioutil.WriteFile(s.conf.Service.SavedTrxLt, []byte(strconv.Itoa(int(lt))), 0644)
	//		if err != nil {
	//			log.Errorf("Error write trx time to file: %v", err)
	//			return
	//		}
	//
	//		go s.ProcessBets(ctx, lt, hash, 10)
	//	}
	//
	//	time.Sleep(1000 * time.Millisecond)
	//}
	//ctx := context.Background()
	//for {
	//	getAccountStateRequest := &api.GetAccountStateRequest{
	//		AccountAddress: s.conf.Service.ContractAddress,
	//	}
	//	getAccountStateResponse, err := s.apiClient.GetAccountState(ctx, getAccountStateRequest)
	//	if err != nil {
	//		log.Errorf("failed GetAccountState with error: %v", err)
	//		continue
	//		// need restart container
	//		// panic(fmt.Sprintf("Error get account state: %v", err))
	//	}
	//
	//	lt := getAccountStateResponse.LastTransactionId.Lt
	//	hash := getAccountStateResponse.LastTransactionId.Hash
	//
	//	savedTrxLt, err := GetSavedTrxLt(s.conf.Service.SavedTrxLt)
	//	if err != nil {
	//		log.Errorf("Error get read saved trx time: %v", err)
	//		return
	//	}
	//
	//	if lt > int64(savedTrxLt) {
	//		err = ioutil.WriteFile(s.conf.Service.SavedTrxLt, []byte(strconv.Itoa(int(lt))), 0644)
	//		if err != nil {
	//			log.Errorf("Error write trx time to file: %v", err)
	//			return
	//		}
	//
	//		go s.ProcessBets(ctx, lt, hash, 10)
	//	}
	//
	//	time.Sleep(1000 * time.Millisecond)
	//}
}
