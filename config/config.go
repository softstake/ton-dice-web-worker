package config

import (
	"github.com/cloudflare/cfssl/log"
	"github.com/spf13/viper"
)

var configType = "yml"

type TonServiceConfig struct {
	TonConfig        string
	ContractAddress  string
	LiteClient       string
	LiteClientConfig string
	ResolveQuery     string
	KeyFileBase      string
	SavedTrxLt       string
}

type Config struct {
	Service TonServiceConfig
}

var Configuration Config

func GetConfig(configName string) Config {

	viper.SetConfigName(configName)
	viper.AddConfigPath("./")
	viper.SetConfigType(configType)

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file, %s", err)
	}

	err := viper.Unmarshal(&Configuration)

	if err != nil {
		log.Fatalf("unable to decode into struct, %v", err)
	}

	return Configuration
}
