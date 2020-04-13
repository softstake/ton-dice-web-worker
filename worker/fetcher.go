package worker

import (
	"context"
	"fmt"
	"github.com/cloudflare/cfssl/log"
	"google.golang.org/grpc"
	"time"

	api "github.com/tonradar/ton-api/proto"
	"ton-dice-web-worker/config"
)

const (
	timeout = 60 // seconds
)

type Fetcher struct {
	conf      config.Config
	apiClient api.TonApiClient
}

func NewFetcher(conf config.Config) *Fetcher {
	opts := []grpc.DialOption{
		grpc.WithInsecure(),
	}

	conn, err := grpc.Dial(fmt.Sprintf("%s:%s", tonApiHost, tonApiPort), opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}

	apiClient := api.NewTonApiClient(conn)

	return &Fetcher{
		conf:      conf,
		apiClient: apiClient,
	}
}

func (f *Fetcher) Start() {
	for {
		getActiveBetsReq := &api.GetActiveBetsRequest{}
		getActiveBetsResp, err := f.apiClient.GetActiveBets(context.Background(), getActiveBetsReq)
		if err != nil {
			log.Errorf("failed to fetch: %v", err)
			continue
		}
		bets := getActiveBetsResp.GetBets()

		fmt.Println("fetched:", bets)

		time.Sleep(timeout * time.Millisecond)
	}
}
