package flag_validator_service

import (
	"os"
	"strconv"
)

type FlagValidatorService struct{}

func New() *FlagValidatorService {
	return &FlagValidatorService{}
}

func (fvs *FlagValidatorService) ValidateFile(file string) bool {
	if file == "" {
		return false
	}

	if _, err := os.Stat(file); os.IsNotExist(err) {
		return false
	}

	return true
}

func (fvs *FlagValidatorService) ValidateHost(host string) bool {
	return host != ""
}

func (fvs *FlagValidatorService) ValidatePort(port string) bool {
	if port == "" {
		return false
	}

	portNumber, err := strconv.Atoi(port)
	if err != nil {
		return false
	}

	return portNumber > 0 && portNumber < 65535
}
