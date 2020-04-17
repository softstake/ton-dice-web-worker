package worker

import (
	"fmt"
	"github.com/cloudflare/cfssl/log"
	"google.golang.org/grpc"
	"os"
	"sync"

	api "github.com/tonradar/ton-api/proto"
	store "github.com/tonradar/ton-dice-web-server/proto"
	"ton-dice-web-worker/config"
)

var (
	storageHost string
	storagePort string
	tonApiHost  string
	tonApiPort  string
)

const (
	timeout = 60 // seconds
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
	conf     config.Config
	resolver *Resolver
	fetcher  *Fetcher
}

func NewWorkerService(conf config.Config) *WorkerService {
	opts := []grpc.DialOption{
		grpc.WithInsecure(),
	}

	conn, err := grpc.Dial(fmt.Sprintf("%s:%s", storageHost, storagePort), opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}
	storageClient := store.NewBetsClient(conn)

	conn, err = grpc.Dial(fmt.Sprintf("%s:%s", tonApiHost, tonApiPort), opts...)
	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}
	apiClient := api.NewTonApiClient(conn)

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

	go s.fetcher.Start()
	s.resolver.Start()

	wg.Wait()
}
