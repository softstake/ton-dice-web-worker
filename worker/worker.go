package worker

import (
	"fmt"
	"google.golang.org/grpc"
	"log"
	"sync"

	api "github.com/tonradar/ton-api/proto"
	store "github.com/tonradar/ton-dice-web-server/proto"
	"ton-dice-web-worker/config"
)

const (
	timeout = 60 // seconds
)

type WorkerService struct {
	conf     *config.TonWebWorkerConfig
	resolver *Resolver
	fetcher  *Fetcher
}

func NewWorkerService(conf *config.TonWebWorkerConfig) *WorkerService {
	opts := []grpc.DialOption{
		grpc.WithInsecure(),
		withClientUnaryInterceptor(),
	}

	log.Println("Worker init...")

	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", conf.StorageHost, conf.StoragePort), opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}
	storageClient := store.NewBetsClient(conn)

	log.Println("Storage client init:", storageClient)

	conn, err = grpc.Dial(fmt.Sprintf("%s:%d", conf.TonAPIHost, conf.TonAPIPort), opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}
	apiClient := api.NewTonApiClient(conn)

	log.Println("Api client init:", apiClient)

	resolver := NewResolver(conf, apiClient, storageClient)
	fetcher := NewFetcher(conf, apiClient, storageClient)

	return &WorkerService{
		conf:     conf,
		resolver: resolver,
		fetcher:  fetcher,
	}
}

func (s *WorkerService) Run() {
	var wg sync.WaitGroup
	wg.Add(1)

	log.Println("Worker starting...")

	go s.fetcher.Start()
	s.resolver.Start()

	wg.Wait()
}
