package create_schedule_api_service

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"plugin"

	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/schedule_repository"
	"github.com/ovvesley/akoflow/pkg/server/types/types_api"
)

type CreateScheduleApiService struct {
	scheduleRepository schedule_repository.IScheduleRepository
}

func New() *CreateScheduleApiService {
	return &CreateScheduleApiService{
		scheduleRepository: config.App().Repository.ScheduleRepository,
	}
}

func (h *CreateScheduleApiService) ValidateUserCode(userCode string) bool {
	hash := sha256.Sum256([]byte(userCode))
	hashStr := hex.EncodeToString(hash[:])
	baseName := "ako_plugin_" + hashStr

	goFile := baseName + ".go"
	soFile := baseName + ".so"

	if _, err := os.Stat(soFile); os.IsNotExist(err) {
		if err := os.WriteFile(goFile, []byte(userCode), 0644); err != nil {
			panic(err)
		}

		cmd := exec.Command("go", "build", "-buildmode=plugin", "-o", soFile, goFile)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		fmt.Println("Compilando plugin:", soFile)
		if err := cmd.Run(); err != nil {
			fmt.Println("Erro ao compilar plugin:", err)
			return false
		} else {
			fmt.Println("Plugin compilado com sucesso:", soFile)
		}
	} else {
		fmt.Println("Usando plugin já compilado:", soFile)
	}

	p, err := plugin.Open(filepath.Clean(soFile))
	if err != nil {
		fmt.Println("Erro ao abrir plugin:", err)
		return false
	}

	sym, err := p.Lookup("AkoScore")
	if err != nil {
		fmt.Println("Erro ao procurar símbolo 'AkoScore':", err)
		return false
	}

	scoreFunc := sym.(func(int) int)
	fmt.Println("Score:", scoreFunc(23424))
	return true
}

func (h *CreateScheduleApiService) CreateSchedule(name string, scheduleType string, code string) (types_api.ApiScheduleType, error) {

	// convert base64 to string if needed
	codeDecoded, err := base64.StdEncoding.DecodeString(code)
	if err != nil {
		return types_api.ApiScheduleType{}, fmt.Errorf("invalid base64 code: %v", err)
	}
	code = string(codeDecoded)

	if !h.ValidateUserCode(code) {
		return types_api.ApiScheduleType{}, fmt.Errorf("invalid user code")
	}

	scheduleEngine, err := h.scheduleRepository.CreateSchedule(name, scheduleType, code)
	if err != nil {
		return types_api.ApiScheduleType{}, err
	}

	return types_api.ApiScheduleType{
		ID:        scheduleEngine.ID,
		Type:      scheduleEngine.Type,
		Code:      scheduleEngine.Code,
		Name:      scheduleEngine.Name,
		CreatedAt: scheduleEngine.CreatedAt,
		UpdatedAt: scheduleEngine.UpdatedAt,
	}, nil
}
