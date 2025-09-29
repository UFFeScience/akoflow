package provenance_graph_service

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

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

	for _, storage := range storages {
		activityName := activitiesMap[storage.ActivityId]
		if activityName == "" {
			activityName = fmt.Sprintf("activity_%d", storage.ActivityId)
		}
		actId := "activity:" + activityName
		if !activityNodeSet[actId] {
			nodes = append(nodes, Node{Id: actId, Label: activityName, Type: "activity"})
			activityNodeSet[actId] = true
		}
	}

	getOwner := func(f map[string]interface{}) string {
		if v, ok := f["Owner"]; ok && v != nil {
			switch t := v.(type) {
			case string:
				return t
			case float64:
				return fmt.Sprintf("%.0f", t)
			default:
				return fmt.Sprintf("%v", t)
			}
		}
		if v, ok := f["owner"]; ok && v != nil {
			return fmt.Sprintf("%v", v)
		}
		if v, ok := f["OwnerId"]; ok && v != nil {
			return fmt.Sprintf("%v", v)
		}
		return ""
	}

	generatorMap := map[string]string{}

	for _, storage := range storages {
		var initialFiles []map[string]interface{}
		_ = json.Unmarshal([]byte(storage.InitialFileList), &initialFiles)

		for _, f := range initialFiles {
			perm, _ := f["Permissions"].(string)
			if len(perm) > 0 && perm[0] == 'd' {
				continue
			}
			path, _ := f["Path"].(string)
			name, _ := f["Name"].(string)
			key := path + "/" + name
			fileNodeId := "file:" + key
			if !fileNodeSet[fileNodeId] {
				nodes = append(nodes, Node{Id: fileNodeId, Label: name, Type: "file"})
				fileNodeSet[fileNodeId] = true
			}
			owner := getOwner(f)
			if owner != "" {
				ownerId, err := strconv.Atoi(owner)
				var ownerActName string
				if err == nil {
					ownerActName = activitiesMap[ownerId]
				}
				if ownerActName == "" {
					ownerActName = "activity_" + owner
				}
				actNodeId := "activity:" + ownerActName
				if !activityNodeSet[actNodeId] {
					nodes = append(nodes, Node{Id: actNodeId, Label: ownerActName, Type: "activity"})
					activityNodeSet[actNodeId] = true
				}
				if _, exists := generatorMap[key]; !exists {
					generatorMap[key] = actNodeId
				}
			}
		}
	}

	for _, storage := range storages {
		var initialFiles, endFiles []map[string]interface{}
		_ = json.Unmarshal([]byte(storage.InitialFileList), &initialFiles)
		_ = json.Unmarshal([]byte(storage.EndFileList), &endFiles)

		initialSet := map[string]bool{}
		for _, f := range initialFiles {
			p, _ := f["Path"].(string)
			n, _ := f["Name"].(string)
			initialSet[p+"/"+n] = true
		}

		actName := activitiesMap[storage.ActivityId]
		if actName == "" {
			actName = fmt.Sprintf("activity_%d", storage.ActivityId)
		}
		actNodeId := "activity:" + actName
		if !activityNodeSet[actNodeId] {
			nodes = append(nodes, Node{Id: actNodeId, Label: actName, Type: "activity"})
			activityNodeSet[actNodeId] = true
		}

		for _, f := range endFiles {
			perm, _ := f["Permissions"].(string)
			if len(perm) > 0 && perm[0] == 'd' {
				continue
			}
			p, _ := f["Path"].(string)
			n, _ := f["Name"].(string)
			key := p + "/" + n
			fileNodeId := "file:" + key
			if !fileNodeSet[fileNodeId] {
				nodes = append(nodes, Node{Id: fileNodeId, Label: n, Type: "file"})
				fileNodeSet[fileNodeId] = true
			}
			if !initialSet[key] {
				if _, exists := generatorMap[key]; !exists {
					generatorMap[key] = actNodeId
				}
			}
		}
	}

	edgeSet := map[string]bool{}
	addEdge := func(from, to string) {
		k := from + "->" + to
		if !edgeSet[k] {
			edgeSet[k] = true
			edges = append(edges, Edge{From: from, To: to})
		}
	}

	for fileKey, actNode := range generatorMap {
		fileNodeId := "file:" + fileKey
		if !fileNodeSet[fileNodeId] {
			parts := strings.Split(fileKey, "/")
			name := parts[len(parts)-1]
			nodes = append(nodes, Node{Id: fileNodeId, Label: name, Type: "file"})
			fileNodeSet[fileNodeId] = true
		}
		addEdge(fileNodeId, actNode)
	}

	for _, storage := range storages {
		activityName := activitiesMap[storage.ActivityId]
		if activityName == "" {
			activityName = "unknown"
		}
		var initialFiles []map[string]interface{}
		_ = json.Unmarshal([]byte(storage.InitialFileList), &initialFiles)
		actName := activitiesMap[storage.ActivityId]
		if actName == "" {
			actName = fmt.Sprintf("activity_%d", storage.ActivityId)
		}
		actNodeId := "activity:" + actName
		for _, f := range initialFiles {
			perm, _ := f["Permissions"].(string)
			if len(perm) > 0 && perm[0] == 'd' {
				continue
			}
			p, _ := f["Path"].(string)
			n, _ := f["Name"].(string)
			key := p + "/" + n
			fileNodeId := "file:" + key
			if fileNodeSet[fileNodeId] {
				if generatorMap[key] == actNodeId {
					continue
				}
				addEdge(actNodeId, fileNodeId)
			}
		}
	}

	return &ProvenanceGraph{Nodes: nodes, Edges: edges}, nil
}

func ExportProvenanceGraphToDot(graph *ProvenanceGraph) string {
	var b strings.Builder
	b.WriteString("digraph prov {\n")
	b.WriteString("  rankdir=LR;\n")
	for _, node := range graph.Nodes {
		shape := "ellipse"
		color := "#facc15"
		if node.Type == "activity" {
			shape = "box"
			color = "#38bdf8"
		}
		label := strings.ReplaceAll(node.Label, "\"", "\\\"")
		b.WriteString(fmt.Sprintf("  \"%s\" [label=\"%s\", shape=%s, style=filled, fillcolor=\"%s\"];\n", node.Id, label, shape, color))
	}
	for _, edge := range graph.Edges {
		label := ""
		if strings.HasPrefix(edge.From, "file:") && strings.HasPrefix(edge.To, "activity:") {
			label = "wasGeneratedBy"
		} else if strings.HasPrefix(edge.From, "activity:") && strings.HasPrefix(edge.To, "file:") {
			label = "used"
		}
		if label != "" {
			b.WriteString(fmt.Sprintf("  \"%s\" -> \"%s\" [label=\"%s\"];\n", edge.From, edge.To, label))
		} else {
			b.WriteString(fmt.Sprintf("  \"%s\" -> \"%s\";\n", edge.From, edge.To))
		}
	}
	b.WriteString("}\n")
	return b.String()
}
