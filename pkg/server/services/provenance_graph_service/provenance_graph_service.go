package provenance_graph_service

import (
	"encoding/json"

	"github.com/ovvesley/akoflow/pkg/server/database/repository/activity_repository"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/storages_repository"
)

type Node struct {
	Id    string `json:"id"`
	Label string `json:"label"`
	Type  string `json:"type"` // "activity" ou "file"
}
type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type ProvenanceGraph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

type ProvenanceGraphService struct {
	storageRepository  storages_repository.IStorageRepository
	activityRepository activity_repository.IActivityRepository
}

func New(storageRepo storages_repository.IStorageRepository, activityRepo activity_repository.IActivityRepository) *ProvenanceGraphService {
	return &ProvenanceGraphService{
		storageRepository:  storageRepo,
		activityRepository: activityRepo,
	}
}

func (s *ProvenanceGraphService) BuildGraph(workflowId int) (*ProvenanceGraph, error) {
	storages := s.storageRepository.FindByWorkflow(workflowId)

	// Mapeia activityId para nome
	activitiesMap := map[int]string{}
	for _, storage := range storages {
		activity, err := s.activityRepository.Find(storage.ActivityId)
		if err == nil {
			activitiesMap[storage.ActivityId] = activity.Name
		}
	}

	nodes := []Node{}
	edges := []Edge{}
	fileNodeSet := map[string]bool{}

	// 1. Nós de atividades
	for _, storage := range storages {
		activityName := activitiesMap[storage.ActivityId]
		if activityName == "" {
			activityName = "unknown"
		}
		nodes = append(nodes, Node{
			Id:    "activity:" + activityName,
			Label: activityName,
			Type:  "activity",
		})
	}

	// 2. Nós de arquivos gerados e edges activity->file
	for _, storage := range storages {
		activityName := activitiesMap[storage.ActivityId]
		if activityName == "" {
			activityName = "unknown"
		}
		var initialFiles, endFiles []map[string]interface{}
		_ = json.Unmarshal([]byte(storage.InitialFileList), &initialFiles)
		_ = json.Unmarshal([]byte(storage.EndFileList), &endFiles)

		initialSet := map[string]bool{}
		for _, f := range initialFiles {
			key := f["Path"].(string) + "/" + f["Name"].(string)
			initialSet[key] = true
		}

		for _, f := range endFiles {
			perm, _ := f["Permissions"].(string)
			if len(perm) > 0 && perm[0] == 'd' {
				continue
			}
			key := f["Path"].(string) + "/" + f["Name"].(string)
			if initialSet[key] {
				continue
			}
			fileNodeId := "file:" + key
			if !fileNodeSet[fileNodeId] {
				nodes = append(nodes, Node{
					Id:    fileNodeId,
					Label: f["Name"].(string),
					Type:  "file",
				})
				fileNodeSet[fileNodeId] = true
			}
			edges = append(edges, Edge{
				From: "activity:" + activityName,
				To:   fileNodeId,
			})
		}
	}

	// 3. Edges file->activity (consumo)
	for _, storage := range storages {
		activityName := activitiesMap[storage.ActivityId]
		if activityName == "" {
			activityName = "unknown"
		}
		var initialFiles []map[string]interface{}
		_ = json.Unmarshal([]byte(storage.InitialFileList), &initialFiles)
		for _, f := range initialFiles {
			perm, _ := f["Permissions"].(string)
			if len(perm) > 0 && perm[0] == 'd' {
				continue
			}
			key := f["Path"].(string) + "/" + f["Name"].(string)
			fileNodeId := "file:" + key
			if fileNodeSet[fileNodeId] {
				edges = append(edges, Edge{
					From: fileNodeId,
					To:   "activity:" + activityName,
				})
			}
		}
	}

	return &ProvenanceGraph{Nodes: nodes, Edges: edges}, nil
}
