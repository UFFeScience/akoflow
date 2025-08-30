package list_schedules_api_service

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/schedule_repository"
	"github.com/ovvesley/akoflow/pkg/server/types/types_api"
)

type ListSchedulesApiService struct {
	scheduleRepository schedule_repository.IScheduleRepository
}

func New() *ListSchedulesApiService {
	return &ListSchedulesApiService{
		scheduleRepository: config.App().Repository.ScheduleRepository,
	}
}

func (h *ListSchedulesApiService) ListAllSchedules() ([]types_api.ApiScheduleType, error) {
	schedulesEngine, err := h.scheduleRepository.ListAllSchedules()

	if err != nil {
		return nil, err
	}

	schedulesApi := make([]types_api.ApiScheduleType, 0, len(schedulesEngine))
	for _, schedule := range schedulesEngine {
		schedulesApi = append(schedulesApi, types_api.ApiScheduleType{
			ID:        schedule.ID,
			Type:      schedule.Type,
			Code:      schedule.Code,
			Name:      schedule.Name,
			CreatedAt: schedule.CreatedAt,
			UpdatedAt: schedule.UpdatedAt,
		})
	}

	return schedulesApi, nil
}
