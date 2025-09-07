package schedule_api_handler

import (
	"net/http"

	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/services/create_schedule_api_service"
	"github.com/ovvesley/akoflow/pkg/server/services/get_schedule_api_service"
	"github.com/ovvesley/akoflow/pkg/server/services/list_schedules_api_service"
)

type ScheduleApiHandler struct {
	listApiSchedulesService  *list_schedules_api_service.ListSchedulesApiService
	createApiScheduleService *create_schedule_api_service.CreateScheduleApiService
	getApiScheduleService    *get_schedule_api_service.GetScheduleApiService
}

func New() *ScheduleApiHandler {
	return &ScheduleApiHandler{
		listApiSchedulesService:  list_schedules_api_service.New(),
		createApiScheduleService: create_schedule_api_service.New(),
		getApiScheduleService:    get_schedule_api_service.New(),
	}
}

func (h *ScheduleApiHandler) ListAllSchedules(w http.ResponseWriter, r *http.Request) {
	schedules, err := h.listApiSchedulesService.ListAllSchedules()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	config.App().HttpHelper.WriteJson(w, schedules)
}

type CreateScheduleApiRequest struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Code string `json:"code"`
}

func (h *ScheduleApiHandler) CreateSchedule(w http.ResponseWriter, r *http.Request) {

	var request CreateScheduleApiRequest
	if err := config.App().HttpHelper.ReadJson(r, &request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if request.Name == "" || request.Type == "" || request.Code == "" {
		http.Error(w, "Name, Type, and Code are required fields", http.StatusBadRequest)
		return
	}

	schedule, err := h.createApiScheduleService.CreateSchedule(request.Name, request.Type, request.Code)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	config.App().HttpHelper.WriteJson(w, schedule)
}

func (h *ScheduleApiHandler) GetSchedule(w http.ResponseWriter, r *http.Request) {
	scheduleName := config.App().HttpHelper.GetUrlParam(r, "scheduleName")

	schedule, err := h.getApiScheduleService.GetScheduleByName(scheduleName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	config.App().HttpHelper.WriteJson(w, schedule)
}
