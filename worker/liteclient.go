package worker

import (
	"fmt"
	"github.com/cloudflare/cfssl/log"
	"os/exec"

	"ton-dice-web-worker/config"
)

type LiteClient struct {
	conf config.Config
}

func NewLiteClient(conf config.Config) (*LiteClient, error) {
	return &LiteClient{conf}, nil
}

func (s *LiteClient) GetAccount(accAddr string) ([]byte, error) {
	if !FileExists(s.conf.Service.LiteClient) || !FileExists(s.conf.Service.LiteClientConfig) {
		return nil, fmt.Errorf("File not found")
	}
	command := fmt.Sprintf("getaccount %s", accAddr)
	output, err := s.RunCommand(command)
	if err != nil {
		return nil, err
	}
	log.Infof("Result of the get account: %s", string(output))
	return output, nil
}

func (s *LiteClient) RunMethod(smcAddr string, method string) ([]byte, error) {
	if !FileExists(s.conf.Service.LiteClient) || !FileExists(s.conf.Service.LiteClientConfig) {
		return nil, fmt.Errorf("File not found")
	}
	command := fmt.Sprintf("runmethod %s %s", smcAddr, method)
	output, err := s.RunCommand(command)
	if err != nil {
		return nil, err
	}
	log.Infof("Result of the runmethod: %s", string(output))
	return output, nil
}

func (s *LiteClient) RunCommand(command string) ([]byte, error) {
	if !FileExists(s.conf.Service.LiteClient) || !FileExists(s.conf.Service.LiteClientConfig) {
		return nil, fmt.Errorf("File not found")
	}
	if command == "" {
		return nil, fmt.Errorf("command should not be empty")
	}
	output, err := exec.Command(s.conf.Service.LiteClient, "-C", s.conf.Service.LiteClientConfig, "-c", command).CombinedOutput()
	if err != nil {
		return nil, err
	}
	return output, nil
}
