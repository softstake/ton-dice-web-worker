package config

import (
	"log"

	"github.com/caarlos0/env/v6"
)

type TonWebWorkerConfig struct {
	ContractAddr string `env:"CONTRACT_ADDR,required"`
	KeyFileBase  string `env:"KEY_FILE_BASE" envDefault:"owner"`
	StorageHost  string `env:"STORAGE_HOST,required"`
	StoragePort  int32  `env:"STORAGE_PORT" envDefault:"5300"`
	TonAPIHost   string `env:"TON_API_HOST,required"`
	TonAPIPort   int32  `env:"TON_API_PORT" envDefault:"5400"`
}

func GetConfig() TonWebWorkerConfig {
	cfg := &TonWebWorkerConfig{}
	if err := env.Parse(cfg); err != nil {
		log.Fatal("Cannot parse initial ENV vars: ", err)
	}
	return *cfg
}
