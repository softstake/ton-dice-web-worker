package worker

import (
	"context"
	"github.com/golang/protobuf/ptypes"
	"log"
	"time"

	api "github.com/tonradar/ton-api/proto"
	store "github.com/tonradar/ton-dice-web-server/proto"
)

type Fetcher struct {
	worker *WorkerService
}

func NewFetcher(worker *WorkerService) *Fetcher {
	log.Println("Fetcher init...")
	return &Fetcher{
		worker: worker,
	}
}

func (f *Fetcher) FetchResults(lt int64, hash string, depth int) (int64, string) {
	ctx := context.Background()

	fetchTransactionsRequest := &api.FetchTransactionsRequest{
		Address: f.worker.conf.ContractAddr,
		Lt:      lt,
		Hash:    hash,
	}

	fetchTransactionsResponse, err := f.worker.apiClient.FetchTransactions(ctx, fetchTransactionsRequest)
	if err != nil {
		log.Println(err)
		return lt, hash
	}

	transactions := fetchTransactionsResponse.Items
	var trx *api.Transaction

	log.Printf("Fetched %d transactions", len(transactions))

	for _, trx = range transactions {
		log.Printf("Processing a transaction with lt %d and hash %s", trx.TransactionId.Lt, trx.TransactionId.Hash)
		for _, outMsg := range trx.OutMsgs {
			gameResult, err := parseOutMessage(outMsg.Message)
			if err != nil {
				log.Printf("Parse output message failed: %v\n", err)
				continue
			}
			log.Printf("Game with id %d and random roll %d is defined", gameResult.Id, gameResult.RandomRoll)

			isBetResolved, err := f.worker.isBetResolved(ctx, int32(gameResult.Id))
			if err != nil {
				log.Println(err)
				continue
			}

			if isBetResolved.IsResolved {
				log.Println("The bet is already resolved, proceed to the next transaction...")
				continue
			}

			playerPayout := outMsg.Value
			resolveTrxHash := trx.TransactionId.Hash
			resolveTrxLt := trx.TransactionId.Lt

			resolvedAt := ptypes.TimestampNow()

			req := &store.UpdateBetRequest{
				Id:             int32(gameResult.Id),
				RandomRoll:     int32(gameResult.RandomRoll),
				PlayerPayout:   playerPayout,
				ResolvedAt:     resolvedAt,
				ResolveTrxHash: resolveTrxHash,
				ResolveTrxLt:   resolveTrxLt,
			}

			_, err = f.worker.storageClient.UpdateBet(ctx, req)
			if err != nil {
				log.Printf("Update bet in DB failed: %v\n", err)
				continue
			}
			log.Printf("Bet with id %d successfully updated", gameResult.Id)
		}
	}

	_lt := lt
	_hash := hash
	if len(transactions) > 0 {
		_lt = trx.TransactionId.Lt
		_hash = trx.TransactionId.Hash
		if depth > 0 {
			depth -= 1
			time.Sleep(timeout * time.Millisecond)
			return f.FetchResults(_lt, _hash, depth)
		}
	}

	return _lt, _hash
}

func (f *Fetcher) Start() {
	log.Println("Start fetching game results...")
	for {
		getAccountStateRequest := &api.GetAccountStateRequest{
			AccountAddress: f.worker.conf.ContractAddr,
		}
		getAccountStateResponse, err := f.worker.apiClient.GetAccountState(context.Background(), getAccountStateRequest)
		if err != nil {
			log.Printf("failed get account state: %v\n", err)
			continue
		}

		lt := getAccountStateResponse.LastTransactionId.Lt
		hash := getAccountStateResponse.LastTransactionId.Hash

		log.Printf("current hash: %s, current lt: %d", hash, lt)

		f.FetchResults(lt, hash, 3)

		time.Sleep(timeout * time.Millisecond)
	}
}
