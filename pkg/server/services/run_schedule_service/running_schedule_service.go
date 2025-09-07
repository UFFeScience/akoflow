package run_schedule_service

import (
	"fmt"
	"path/filepath"
	"plugin"

	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/schedule_repository"
)

type RunScheduleService struct {
	scheduleRepository schedule_repository.IScheduleRepository
}

func New() RunScheduleService {
	return RunScheduleService{
		scheduleRepository: config.App().Repository.ScheduleRepository,
	}
}

func (r *RunScheduleService) StartRunningSchedule(scheduleName string, input map[string]any) (float64, error) {
	// Here you would implement the logic to start running the schedule
	// For example, you might want to fetch the schedule by name and then execute it with the provided input

	schedule, err := r.scheduleRepository.GetScheduleByName("schedule1")

	if err != nil {
		config.App().Logger.Error("Error getting schedule: " + err.Error())
		return 0, err
	}

	println("Schedule found: ", schedule.Name)

	p, err := plugin.Open(filepath.Clean(schedule.PluginSoPath))

	if err != nil {
		fmt.Println("Erro ao abrir plugin:", err)
		return 0, err
	}

	sym, err := p.Lookup("AkoScore")
	if err != nil {
		fmt.Println("Erro ao procurar símbolo 'AkoScore':", err)
		return 0, err
	}

	akoScoreFunc, ok := sym.(func(any) float64)

	if !ok {
		fmt.Println("Símbolo 'AkoScore' não é uma função válida")
		return 0, fmt.Errorf("invalid AkoScore function")
	}

	// input := map[string]any{
	// 	"time_estimate":   1,
	// 	"memory_required": 512.0,
	// 	"memory_free":   e  1024.0,
	// 	"memory_max":      2048.0,
	// 	"affinity":        0.8,
	// 	"alpha":           0.5,
	// }

	result := akoScoreFunc(input)

	return result, nil
}
