package create_schedule_api_service

import (
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

func (h *CreateScheduleApiService) CreateSchedule(name string, scheduleType string, code string) (types_api.ApiScheduleType, error) {

	scheduleEngine, err := h.scheduleRepository.CreateSchedule(name, scheduleType, code)
	if err != nil {
		return types_api.ApiScheduleType{}, err
	}

	return types_api.ApiScheduleType{
		ID:        scheduleEngine.ID,
		Type:      scheduleEngine.Type,
		Code:      scheduleEngine.Code,
		Name:      scheduleEngine.Name,
		CreatedAt: "",
		UpdatedAt: "",
	}, nil
}
