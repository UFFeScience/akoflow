package find_workflow_api_service

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"
	"github.com/ovvesley/akoflow/pkg/server/mapper/mapper_engine_api"
	"github.com/ovvesley/akoflow/pkg/server/repository/activity_repository"
	"github.com/ovvesley/akoflow/pkg/server/repository/workflow_repository"
	"github.com/ovvesley/akoflow/pkg/server/types/types_api"
	"github.com/ovvesley/akoflow/pkg/server/utils"
)

type FindWorkflowApiService struct {
	workflowRepository workflow_repository.IWorkflowRepository
	activityRepository activity_repository.IActivityRepository
}

func New() *FindWorkflowApiService {
	return &FindWorkflowApiService{
		workflowRepository: config.App().Repository.WorkflowRepository,
		activityRepository: config.App().Repository.ActivityRepository,
	}
}

func (h *FindWorkflowApiService) FindWorkflowById(id int) (types_api.ApiWorkflowType, error) {
	wf, err := h.workflowRepository.Find(id)

	wfas, err := h.activityRepository.GetActivitiesByWorkflowIds([]int{wf.Id})

	if err != nil {
		return types_api.ApiWorkflowType{}, err
	}

	wf = utils.HydrateWorkflow(wf, wfas)

	wfApi := mapper_engine_api.MapEngineWorkflowEntityToApiWorkflowEntity(wf)

	wfApi = calculateWorkflowMetrics(wfApi, wf)

	if err != nil {
		return types_api.ApiWorkflowType{}, err
	}

	return wfApi, nil
}

func ParseTimestamp(timestamp string) (time.Time, error) {
	layout := "2006-01-02 15:04:05"
	return time.Parse(layout, timestamp)
}

func calculateWorkflowMetrics(wfApi types_api.ApiWorkflowType, wfEngine workflow_entity.Workflow) types_api.ApiWorkflowType {
	wfApi.Spec.StartExecution = wfEngine.Spec.Activities[0].CreatedAt
	wfApi.Spec.EndExecution = wfEngine.Spec.Activities[0].CreatedAt
	wfApi.Spec.LongestActivity = types_api.ApiWorkflowActivityType{}
	wfApi.Spec.DiskUsage = "0"

	wfTotalDiskUsage := 0

	if !wfEngine.IsStoragePolicyStandalone() {
		wfTotalDiskUsage = parseDiskUsage(wfEngine.Spec.StoragePolicy.StorageSize)
	}

	totalDuration := 0

	for index, wfa := range wfApi.Spec.Activities {

		timestampWorkflowStart, _ := ParseTimestamp(wfApi.Spec.StartExecution)
		timestampWorkflowEnd, _ := ParseTimestamp(wfApi.Spec.EndExecution)

		timestampStartExecution, _ := ParseTimestamp(wfa.CreatedAt)
		timestampEndExecution, _ := ParseTimestamp(wfa.FinishedAt)

		if timestampStartExecution.Before(timestampWorkflowStart) {
			wfApi.Spec.StartExecution = wfa.CreatedAt
		}

		if timestampEndExecution.After(timestampWorkflowEnd) {
			wfApi.Spec.EndExecution = wfa.FinishedAt
		}

		wfaDuration := int(timestampEndExecution.Sub(timestampStartExecution).Seconds())
		wfApi.Spec.Activities[index].ExecutionTime = fmt.Sprintf("%d", wfaDuration)

		currentDuration := int(timestampEndExecution.Sub(timestampStartExecution).Seconds())

		timestampLongestActivityEnd, _ := ParseTimestamp(wfApi.Spec.LongestActivity.FinishedAt)
		timestampLongestActivityStart, _ := ParseTimestamp(wfApi.Spec.LongestActivity.CreatedAt)

		longestActivityDuration := int(timestampLongestActivityEnd.Sub(timestampLongestActivityStart).Seconds())

		if currentDuration > longestActivityDuration {
			wfApi.Spec.LongestActivity = wfApi.Spec.Activities[index]
		}
		if wfEngine.IsStoragePolicyStandalone() {
			wfTotalDiskUsage += parseDiskUsage(wfEngine.Spec.StoragePolicy.StorageSize)
		}

	}

	timestampWorkflowStart, _ := ParseTimestamp(wfApi.Spec.StartExecution)
	timestampWorkflowEnd, _ := ParseTimestamp(wfApi.Spec.EndExecution)

	totalDuration = int(timestampWorkflowEnd.Sub(timestampWorkflowStart).Seconds())

	wfApi.Spec.ExecutionTime = fmt.Sprintf("%d", totalDuration)

	wfApi.Spec.DiskUsage = fmt.Sprintf("%d", wfTotalDiskUsage)

	return wfApi
}

func parseDiskUsage(diskUsage string) int {
	// Converte a string de uso de disco para MB
	var usage int
	var multiplier int

	switch {
	case strings.HasSuffix(diskUsage, "Ki"):
		multiplier = 1 // 1 KiB = 1 KiB
		usage, _ = strconv.Atoi(strings.TrimSuffix(diskUsage, "Ki"))
	case strings.HasSuffix(diskUsage, "Mi"):
		multiplier = 1024 // 1 MiB = 1024 KiB
		usage, _ = strconv.Atoi(strings.TrimSuffix(diskUsage, "Mi"))
	case strings.HasSuffix(diskUsage, "Gi"):
		multiplier = 1024 * 1024 // 1 GiB = 1024 MiB = 1048576 KiB
		usage, _ = strconv.Atoi(strings.TrimSuffix(diskUsage, "Gi"))
	case strings.HasSuffix(diskUsage, "Ti"):
		multiplier = 1024 * 1024 * 1024 // 1 TiB = 1024 GiB = 1048576 MiB = 1073741824 KiB
		usage, _ = strconv.Atoi(strings.TrimSuffix(diskUsage, "Ti"))
	default:
		// Se nenhuma unidade for encontrada, tenta interpretar como MB (assumindo o valor diretamente em MB)
		usage, _ = strconv.Atoi(diskUsage)
		return usage
	}

	return usage * multiplier / 1024 // Converte para MB
}
