package worker

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"log"

	api "github.com/tonradar/ton-api/proto"
	store "github.com/tonradar/ton-dice-web-server/proto"
	"ton-dice-web-worker/config"
)

const (
	timeout = 60 // seconds
)

type WorkerService struct {
	conf          *config.TonWebWorkerConfig
	apiClient     api.TonApiClient
	storageClient store.BetsClient
}

func NewWorkerService(conf *config.TonWebWorkerConfig) *WorkerService {
	opts := []grpc.DialOption{
		grpc.WithInsecure(),
		withClientUnaryInterceptor(),
	}

	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", conf.StorageHost, conf.StoragePort), opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}
	storageClient := store.NewBetsClient(conn)

	conn, err = grpc.Dial(fmt.Sprintf("%s:%d", conf.TonAPIHost, conf.TonAPIPort), opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}
	apiClient := api.NewTonApiClient(conn)

	return &WorkerService{
		conf:          conf,
		apiClient:     apiClient,
		storageClient: storageClient,
	}
}

func (s *WorkerService) isBetSaved(ctx context.Context, id int32) (*store.IsBetSavedResponse, error) {
	isBetFetchedReq := &store.IsBetSavedRequest{
		Id: id,
	}

	resp, err := s.storageClient.IsBetSaved(ctx, isBetFetchedReq)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *WorkerService) isBetResolved(ctx context.Context, id int32) (*store.IsBetResolvedResponse, error) {
	isBetResolvedReq := &store.IsBetResolvedRequest{
		Id: id,
	}

	resp, err := s.storageClient.IsBetResolved(ctx, isBetResolvedReq)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
