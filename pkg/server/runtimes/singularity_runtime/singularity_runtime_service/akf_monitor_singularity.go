package singularity_runtime_service

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

type AkfMonitorSingularity struct {
	FilePath string
	Pid      string
}

func NewAkfMonitorSingularity() *AkfMonitorSingularity {
	directory, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return &AkfMonitorSingularity{
		FilePath: filepath.Join(directory, "../../pkg/server/runtimes/singularity_runtime/singularity_runtime_service/akf_monitor_singularity.sh"),
	}
}

func (ams *AkfMonitorSingularity) ReadFile() ([]string, error) {
	file, err := os.Open(ams.FilePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return lines, nil
}

func (ams *AkfMonitorSingularity) ReadFileAsString() (string, error) {
	lines, err := ams.ReadFile()
	if err != nil {
		return "", err
	}
	return strings.Join(lines, "\n"), nil
}

func (ams *AkfMonitorSingularity) SetPid(pid string) *AkfMonitorSingularity {
	ams.Pid = pid
	return ams
}

func (ams *AkfMonitorSingularity) GetPid() string {
	return ams.Pid
}

func (ams *AkfMonitorSingularity) GetScript() (string, error) {
	script, err := ams.ReadFileAsString()

	if err != nil {
		return "", err
	}

	if ams.GetPid() == "" {

		return "", nil
	}

	script = strings.ReplaceAll(script, "##PARENT_PID##", ams.GetPid())

	if err != nil {
		return "", err
	}
	return script, nil
}
