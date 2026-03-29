package provenance_graph_service

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ovvesley/akoflow/pkg/server/database/repository/activity_repository"
	"github.com/ovvesley/akoflow/pkg/server/database/repository/storages_repository"
)

type Node struct {
	Id    string `json:"id"`
	Label string `json:"label"`
	Type  string `json:"type"` // "activity", "file", or "preExisting"
}
type Edge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label"`
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

// BuildGraph builds a provenance graph for the given workflow.
//
// Algorithm:
//   - For each activity storage snapshot, compute the diff between EndFileList and InitialFileList.
//   - Files present in EndFileList but NOT in InitialFileList were CREATED by that activity
//     → edge: activity → file  (labeled "wasGeneratedBy")
//   - Files present in InitialFileList were CONSUMED/USED as input by that activity
//     → edge: file → activity  (labeled "used")
func (s *ProvenanceGraphService) BuildGraph(workflowId int) (*ProvenanceGraph, error) {
	storages := s.storageRepository.FindByWorkflow(workflowId)

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
	activityNodeSet := map[string]bool{}
	edgeSet := map[string]bool{}
	createdByWorkflow := map[string]bool{} // fileNodeIds created by some activity

	addEdge := func(from, to, label string) {
		k := from + "->" + to
		if !edgeSet[k] {
			edgeSet[k] = true
			edges = append(edges, Edge{From: from, To: to, Label: label})
		}
	}

	isDir := func(f map[string]interface{}) bool {
		perm, _ := f["Permissions"].(string)
		return len(perm) > 0 && perm[0] == 'd'
	}

	fileKey := func(f map[string]interface{}) string {
		p, _ := f["Path"].(string)
		n, _ := f["Name"].(string)
		return p + "/" + n
	}

	ensureFileNode := func(key string) {
		fileNodeId := "file:" + key
		if !fileNodeSet[fileNodeId] {
			parts := strings.Split(key, "/")
			name := parts[len(parts)-1]
			nodes = append(nodes, Node{Id: fileNodeId, Label: name, Type: "file"})
			fileNodeSet[fileNodeId] = true
		}
	}

	for _, storage := range storages {
		actName := activitiesMap[storage.ActivityId]
		if actName == "" {
			actName = fmt.Sprintf("activity_%d", storage.ActivityId)
		}
		actNodeId := "activity:" + actName

		if !activityNodeSet[actNodeId] {
			nodes = append(nodes, Node{Id: actNodeId, Label: actName, Type: "activity"})
			activityNodeSet[actNodeId] = true
		}

		var initialFiles, endFiles []map[string]interface{}
		_ = json.Unmarshal([]byte(storage.InitialFileList), &initialFiles)
		_ = json.Unmarshal([]byte(storage.EndFileList), &endFiles)

		// Build set of initial (pre-existing) files
		initialSet := map[string]bool{}
		for _, f := range initialFiles {
			if isDir(f) {
				continue
			}
			initialSet[fileKey(f)] = true
		}

		// Input files: existed before activity ran → file → activity
		for _, f := range initialFiles {
			if isDir(f) {
				continue
			}
			key := fileKey(f)
			ensureFileNode(key)
			addEdge("file:"+key, actNodeId, "used")
		}

		// Created files: present in end but not in initial → activity → file
		for _, f := range endFiles {
			if isDir(f) {
				continue
			}
			key := fileKey(f)
			if initialSet[key] {
				continue // pre-existing, not created by this activity
			}
			ensureFileNode(key)
			createdByWorkflow["file:"+key] = true
			addEdge(actNodeId, "file:"+key, "wasGeneratedBy")
		}
	}

	// Post-process: file nodes that were never created by any activity are pre-existing entities
	for i, n := range nodes {
		if n.Type == "file" && !createdByWorkflow[n.Id] {
			nodes[i].Type = "preExisting"
		}
	}

	return &ProvenanceGraph{Nodes: nodes, Edges: edges}, nil
}
