package main

import (
	"ton-dice-web-worker/config"
	"ton-dice-web-worker/worker"
)

func main() {
	cfg := config.GetConfig()
	service := worker.NewWorkerService(&cfg)
	service.Run()
}
