package worker

import (
	"context"
	"github.com/cloudflare/cfssl/log"
	"io/ioutil"
	"strconv"
	"time"

	api "github.com/tonradar/ton-api/proto"
	store "github.com/tonradar/ton-dice-web-server/proto"
	"ton-dice-web-worker/config"
)

const (
	SavedTrxLtFileName = "trxlt.save"
)

type Fetcher struct {
	conf          *config.TonWebWorkerConfig
	apiClient     api.TonApiClient
	storageClient store.BetsClient
}

func NewFetcher(conf *config.TonWebWorkerConfig, apiClient api.TonApiClient, storageClient store.BetsClient) *Fetcher {
	return &Fetcher{
		conf:          conf,
		apiClient:     apiClient,
		storageClient: storageClient,
	}
}

func (f *Fetcher) isBetResolved(ctx context.Context, id int32) (*store.IsBetResolvedResponse, error) {
	isBetResolvedReq := &store.IsBetResolvedRequest{
		Id: id,
	}

	resp, err := f.storageClient.IsBetResolved(ctx, isBetResolvedReq)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (f *Fetcher) FetchResults(ctx context.Context, lt int64, hash string, depth int) (int64, string) {
	fetchTransactionsRequest := &api.FetchTransactionsRequest{
		Address: f.conf.ContractAddr,
		Lt:      lt,
		Hash:    hash,
	}

	fetchTransactionsResponse, err := f.apiClient.FetchTransactions(ctx, fetchTransactionsRequest)
	if err != nil {
		log.Error(err)
	}

	transactions := fetchTransactionsResponse.Items
	var trx *api.Transaction

	for _, trx = range transactions {
		for _, outMsg := range trx.OutMsgs {
			gameResult, err := parseOutMessage(outMsg.Message)
			if err != nil {
				log.Errorf("output message parse failed with %s\n", err)
				continue
			}

			isBetResolved, err := f.isBetResolved(ctx, int32(gameResult.Id))
			if err != nil {
				log.Error(err)
				continue
			}

			if isBetResolved.Yes {
				log.Info("the bet is already resolved")
				continue
			}

			playerPayout := outMsg.Value
			resolveTrxHash := trx.TransactionId.Hash
			resolveTrxLt := trx.TransactionId.Lt

			req := &store.UpdateBetRequest{
				Id:             int32(gameResult.Id),
				RandomRoll:     int32(gameResult.RandomRoll),
				PlayerPayout:   playerPayout,
				ResolveTrxHash: resolveTrxHash,
				ResolveTrxLt:   resolveTrxLt,
			}

			_, err = f.storageClient.UpdateBet(ctx, req)
			if err != nil {
				log.Errorf("update bet in DB failed with %s\n", err)
				continue
			}
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
			return f.FetchResults(ctx, _lt, _hash, depth)
		}
	}

	return _lt, _hash
}

func (f *Fetcher) Start() {
	ctx := context.Background()
	for {
		getAccountStateRequest := &api.GetAccountStateRequest{
			AccountAddress: f.conf.ContractAddr,
		}
		getAccountStateResponse, err := f.apiClient.GetAccountState(ctx, getAccountStateRequest)
		if err != nil {
			log.Errorf("failed GetAccountState with error: %v", err)
			continue
		}

		lt := getAccountStateResponse.LastTransactionId.Lt
		hash := getAccountStateResponse.LastTransactionId.Hash

		savedTrxLt, err := GetSavedTrxLt(SavedTrxLtFileName)
		if err != nil {
			log.Errorf("Error get read saved trx time: %v", err)
			return
		}

		if lt > int64(savedTrxLt) {
			err = ioutil.WriteFile(SavedTrxLtFileName, []byte(strconv.Itoa(int(lt))), 0644)
			if err != nil {
				log.Errorf("Error write trx time to file: %v", err)
				return
			}

			f.FetchResults(ctx, lt, hash, 3)
		}

		time.Sleep(timeout * time.Millisecond)
	}
}
