package get_schedule_api_service

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/schedule_repository"
	"github.com/ovvesley/akoflow/pkg/server/types/types_api"
)

type GetScheduleApiService struct {
	scheduleRepository schedule_repository.IScheduleRepository
}

func New() *GetScheduleApiService {
	return &GetScheduleApiService{
		scheduleRepository: config.App().Repository.ScheduleRepository,
	}
}

func (h *GetScheduleApiService) GetScheduleByName(scheduleId string) (*types_api.ApiScheduleType, error) {
	scheduleEngine, err := h.scheduleRepository.GetScheduleByName(scheduleId)

	if err != nil {
		return nil, err
	}

	scheduleApi := &types_api.ApiScheduleType{
		ID:        scheduleEngine.ID,
		Type:      scheduleEngine.Type,
		Code:      scheduleEngine.Code,
		Name:      scheduleEngine.Name,
		CreatedAt: "",
		UpdatedAt: "",
	}

	return scheduleApi, nil
}
