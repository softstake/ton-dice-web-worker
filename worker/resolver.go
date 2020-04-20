package worker

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	api "github.com/tonradar/ton-api/proto"
	store "github.com/tonradar/ton-dice-web-server/proto"
	"ton-dice-web-worker/config"
)

const (
	ResolveQueryFileName = "resolve-query.fif"
)

type Resolver struct {
	conf          *config.TonWebWorkerConfig
	apiClient     api.TonApiClient
	storageClient store.BetsClient
}

func NewResolver(conf *config.TonWebWorkerConfig, apiClient api.TonApiClient, storageClient store.BetsClient) *Resolver {
	return &Resolver{
		conf:          conf,
		apiClient:     apiClient,
		storageClient: storageClient,
	}
}

func (f *Resolver) ResolveQuery(betId int, seed string) error {
	log.Printf("Resolving bet with id %d...", betId)
	fileNameWithPath := ResolveQueryFileName
	fileNameStart := strings.LastIndex(fileNameWithPath, "/")
	fileName := fileNameWithPath[fileNameStart+1:]

	bocFile := strings.Replace(fileName, ".fif", ".boc", 1)

	_ = os.Remove(bocFile)

	var out bytes.Buffer
	cmd := exec.Command("fift", "-s", fileNameWithPath, f.conf.KeyFileBase, f.conf.ContractAddr, strconv.Itoa(betId), seed)

	cmd.Stderr = &out
	err := cmd.Run()
	if err != nil {
		log.Printf("cmd.Run() failed with: %v\n", err)
		return err
	}

	if FileExists(bocFile) {
		data, err := ioutil.ReadFile(bocFile)
		if err != nil {
			log.Println(err)
		}

		sendMessageRequest := &api.SendMessageRequest{
			Body: data,
		}

		sendMessageResponse, err := f.apiClient.SendMessage(context.Background(), sendMessageRequest)
		if err != nil {
			log.Printf("failed send message: %v\n", err)
			return err
		}

		log.Printf("send message status: %v\n", sendMessageResponse.Ok)

		return nil
	}

	return fmt.Errorf("file not found, maybe fift compile failed")
}

func (f *Resolver) isBetCreated(ctx context.Context, id int32) (*store.IsBetCreatedResponse, error) {
	isBetFetchedReq := &store.IsBetCreatedRequest{
		Id: id,
	}

	resp, err := f.storageClient.IsBetCreated(ctx, isBetFetchedReq)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (f *Resolver) Start() {
	log.Println("Resolver start")
	for {
		ctx := context.Background()

		getActiveBetsReq := &api.GetActiveBetsRequest{}
		getActiveBetsResp, err := f.apiClient.GetActiveBets(ctx, getActiveBetsReq)
		if err != nil {
			log.Printf("failed to get active bets: %v\n", err)
			continue
		}
		bets := getActiveBetsResp.GetBets()

		log.Printf("%d active bets received from smart-contract", len(bets))

		for _, bet := range bets {
			isBetCreated, err := f.isBetCreated(ctx, bet.Id)
			if err != nil {
				log.Println(err)
				continue
			}

			if isBetCreated.IsCreated {
				log.Println("the bet is already in storage")
				continue
			}

			req, err := BuildCreateBetRequest(bet)
			if err != nil {
				log.Printf("failed to build create bet request: %v\n", err)
				continue
			}

			_, err = f.storageClient.CreateBet(ctx, req)
			if err != nil {
				log.Printf("save bet in DB failed: %v\n", err)
				continue
			}

			err = f.ResolveQuery(int(bet.Id), bet.Seed)
			if err != nil {
				log.Printf("failed to resolve bet: %v\n", err)
				continue
			}
		}

		time.Sleep(timeout * time.Millisecond)
	}
}
