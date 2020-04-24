package main

import (
	"ton-dice-web-worker/config"
	"ton-dice-web-worker/worker"
)

func main() {
	cfg := config.GetConfig()

	service := worker.NewWorkerService(&cfg)

	fetcher := worker.NewFetcher(service)
	resolver := worker.NewResolver(service)

	go fetcher.Start()
	resolver.Start()
}
