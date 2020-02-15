package main

import (
	"fmt"
	"ton-dice-web-worker/config"
	"ton-dice-web-worker/worker"
)

func main() {
	cfg := config.GetConfig("config")
	fmt.Println(cfg)
	service := worker.NewWorkerService(cfg)
	service.Run()
}
