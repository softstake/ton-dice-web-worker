package worker

import (
	"bytes"
	"context"
	"fmt"
	"github.com/cloudflare/cfssl/log"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	api "github.com/tonradar/ton-api/proto"
	store "github.com/tonradar/ton-dice-web-server/proto"
	"ton-dice-web-worker/config"
)

type Resolver struct {
	conf          config.Config
	apiClient     api.TonApiClient
	storageClient store.BetsClient
}

func NewResolver(conf config.Config, apiClient api.TonApiClient, storageClient store.BetsClient) *Resolver {
	return &Resolver{
		conf:          conf,
		apiClient:     apiClient,
		storageClient: storageClient,
	}
}

func (f *Resolver) ResolveQuery(betId int, seed string) error {
	fileNameWithPath := f.conf.Service.ResolveQuery
	fileNameStart := strings.LastIndex(fileNameWithPath, "/")
	fileName := fileNameWithPath[fileNameStart+1:]

	bocFile := strings.Replace(fileName, ".fif", ".boc", 1)

	_ = os.Remove(bocFile)

	var out bytes.Buffer
	cmd := exec.Command("fift", "-s", fileNameWithPath, f.conf.Service.KeyFileBase, f.conf.Service.ContractAddress, strconv.Itoa(betId), seed)

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

		sendMessageResponse, err := f.apiClient.SendMessage(context.Background(), sendMessageRequest)
		if err != nil {
			log.Errorf("failed ResolveQuery method with: %v", err)
			return err
		}

		fmt.Printf("ResolveBet: send message status: %v", sendMessageResponse.Ok)

		return nil
	}

	return fmt.Errorf("File not found, maybe fift compile failed?")
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
	ctx := context.Background()
	for {
		getActiveBetsReq := &api.GetActiveBetsRequest{}
		getActiveBetsResp, err := f.apiClient.GetActiveBets(ctx, getActiveBetsReq)
		if err != nil {
			log.Errorf("failed to get active bets: %v", err)
			continue
		}
		bets := getActiveBetsResp.GetBets()

		for _, bet := range bets {
			isBetCreated, err := f.isBetCreated(ctx, bet.Id)
			if err != nil {
				log.Error(err)
				continue
			}

			if isBetCreated.Yes {
				log.Info("the bet is already in storage")
				continue
			}

			req, err := BuildCreateBetRequest(bet)
			if err != nil {
				log.Errorf("failed to build create bet request: %v", err)
				continue
			}

			_, err = f.storageClient.CreateBet(ctx, req)
			if err != nil {
				log.Errorf("save bet in DB failed with %s\n", err)
				continue
			}

			err = f.ResolveQuery(int(bet.Id), bet.Seed)
			if err != nil {
				log.Errorf("failed to resolve bet with %s\n", err)
				continue
			}
		}

		time.Sleep(timeout * time.Millisecond)
	}
}
