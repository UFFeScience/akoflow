package list_runtimes_api_service

import (
	"github.com/ovvesley/akoflow/pkg/server/config"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/runtime_repository"
	"github.com/ovvesley/akoflow/pkg/server/mapper/mapper_engine_api"
	"github.com/ovvesley/akoflow/pkg/server/types/types_api"
)

type ListRuntimesApiService struct {
	runtimeRepository runtime_repository.IRuntimeRepository
}

func New() *ListRuntimesApiService {
	return &ListRuntimesApiService{
		runtimeRepository: config.App().Repository.RuntimeRepository,
	}
}

func (h *ListRuntimesApiService) ListAllRuntimes() ([]types_api.ApiRuntimeType, error) {
	runtimesEngine, err := h.runtimeRepository.GetAll()

	if err != nil {
		return nil, err
	}

	runtimeApi := mapper_engine_api.MapEngineRuntimeEntityToApiRuntimeEntityList(runtimesEngine)
	return runtimeApi, nil
}
