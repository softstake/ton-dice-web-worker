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
)

const (
	ResolveQueryFileName = "resolve-query.fif"
)

type Resolver struct {
	worker *WorkerService
}

func NewResolver(worker *WorkerService) *Resolver {
	log.Println("Resolver init...")
	return &Resolver{
		worker: worker,
	}
}

func (r *Resolver) ResolveQuery(betId int, seed string) error {
	log.Printf("Resolving bet with id %d...", betId)
	fileNameWithPath := ResolveQueryFileName
	fileNameStart := strings.LastIndex(fileNameWithPath, "/")
	fileName := fileNameWithPath[fileNameStart+1:]

	bocFile := strings.Replace(fileName, ".fif", ".boc", 1)

	_ = os.Remove(bocFile)

	var out bytes.Buffer
	cmd := exec.Command("fift", "-s", fileNameWithPath, r.worker.conf.KeyFileBase, r.worker.conf.ContractAddr, strconv.Itoa(betId), seed)

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

		sendMessageResponse, err := r.worker.apiClient.SendMessage(context.Background(), sendMessageRequest)
		if err != nil {
			log.Printf("failed send message: %v\n", err)
			return err
		}

		log.Printf("send message status: %v\n", sendMessageResponse.Ok)

		return nil
	}

	return fmt.Errorf("file not found, maybe fift compile failed")
}

func (r *Resolver) Start() {
	log.Println("Resolver start")
	for {
		ctx := context.Background()

		getActiveBetsReq := &api.GetActiveBetsRequest{}
		getActiveBetsResp, err := r.worker.apiClient.GetActiveBets(ctx, getActiveBetsReq)
		if err != nil {
			log.Printf("failed to get active bets: %v\n", err)
			continue
		}
		bets := getActiveBetsResp.GetBets()

		log.Printf("%d active bets received from smart-contract", len(bets))

		for _, bet := range bets {
			isBetSaved, err := r.worker.isBetSaved(ctx, bet.Id)
			if err != nil {
				log.Println(err)
				continue
			}

			if !isBetSaved.IsSaved {
				req, err := BuildSaveBetRequest(bet)
				if err != nil {
					log.Printf("failed to build create bet request: %v\n", err)
					continue
				}

				_, err = r.worker.storageClient.SaveBet(ctx, req)
				if err != nil {
					log.Printf("save bet in DB failed: %v\n", err)
					continue
				}
			} else {
				log.Println("the bet is already in storage")
			}

			isBetResolved, err := r.worker.isBetResolved(ctx, bet.Id)
			if err != nil {
				log.Println(err)
				continue
			}

			if !isBetResolved.IsResolved {
				err = r.ResolveQuery(int(bet.Id), bet.Seed)
				if err != nil {
					log.Printf("failed to resolve bet: %v\n", err)
				}
			} else {
				log.Println("the bet is already resolved")
			}
		}

		time.Sleep(timeout * time.Millisecond)
	}
}
