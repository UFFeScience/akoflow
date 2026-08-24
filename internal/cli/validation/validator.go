package validation

import (
	"os"
	"strconv"
)

type Validator struct{}

func New() *Validator {
	return &Validator{}
}

func (fvs *Validator) ValidateFile(file string) bool {
	if file == "" {
		return false
	}

	if _, err := os.Stat(file); os.IsNotExist(err) {
		return false
	}

	return true
}

func (fvs *Validator) ValidateHost(host string) bool {
	return host != ""
}

func (fvs *Validator) ValidatePort(port string) bool {
	if port == "" {
		return false
	}

	portNumber, err := strconv.Atoi(port)
	if err != nil {
		return false
	}

	return portNumber > 0 && portNumber < 65535
}
