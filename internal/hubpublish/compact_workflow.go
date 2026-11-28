package hubpublish

import (
	"encoding/json"
	"reflect"
	"strconv"

	"gopkg.in/yaml.v3"
)

// compactWorkflowYAML keeps the full build origin and inspection on every
// activity after decoding, while storing repeated payloads only once.
func compactWorkflowYAML(data []byte) ([]byte, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	if len(document.Content) == 0 {
		return data, nil
	}
	root := document.Content[0]
	activities := yamlField(yamlField(root, "spec"), "activities")
	if activities == nil {
		return data, nil
	}
	shared := make(map[string]*yaml.Node)
	anchors := 0
	for _, activity := range activities.Content {
		command := yamlField(activity, "command")
		source := yamlField(yamlField(command, "executable"), "source")
		if source == nil || source.Kind != yaml.MappingNode {
			continue
		}
		for index := 0; index < len(source.Content); index += 2 {
			key, value := source.Content[index], source.Content[index+1]
			if key.Value != "origin" && key.Value != "inspection" {
				continue
			}
			if value.Kind != yaml.MappingNode {
				continue
			}
			encoded, err := yaml.Marshal(value)
			if err != nil {
				return nil, err
			}
			identity := key.Value + string(encoded)
			first, exists := shared[identity]
			if !exists {
				shared[identity] = value
				continue
			}
			if first.Anchor == "" {
				anchors++
				first.Anchor = "artifact" + strconv.Itoa(anchors)
			}
			source.Content[index+1] = &yaml.Node{Kind: yaml.AliasNode, Value: first.Anchor, Alias: first}
		}
	}
	if anchors == 0 {
		return data, nil
	}
	return yaml.Marshal(&document)
}

func yamlField(node *yaml.Node, name string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for index := 0; index < len(node.Content); index += 2 {
		if node.Content[index].Value == name {
			return node.Content[index+1]
		}
	}
	return nil
}

// compactWorkflowSnapshot removes duplicate payloads from the JSON views only.
// The authored workflow is stored separately and can still reconstruct every
// activity. Artifact origins in these views retain their recipe and digests.
func compactWorkflowSnapshot(version map[string]any) bool {
	activities, _ := version["activities"].([]any)
	archives := make(map[string]bool)
	inspections := make(map[string]bool)
	changed := false
	for _, value := range activities {
		activity, _ := value.(map[string]any)
		command, _ := activity["command"].(map[string]any)
		executable, _ := command["executable"].(map[string]any)
		source, _ := executable["source"].(map[string]any)
		origin, _ := source["origin"].(map[string]any)
		build, _ := origin["build"].(map[string]any)
		archive, _ := build["contextArchive"].(string)
		if source["type"] != "build" || archive == "" {
			continue
		}
		if archives[archive] {
			delete(build, "contextArchive")
			changed = true
		}
		archives[archive] = true
		if inspection := source["inspection"]; inspection != nil {
			encoded, err := json.Marshal(inspection)
			if err != nil {
				continue
			}
			key := string(encoded)
			if inspections[key] {
				delete(source, "inspection")
				changed = true
			}
			inspections[key] = true
		}
	}
	return changed
}

// The projected activity rows contain the same attempt data when the arrays
// are identical. Preserve a distinct attempt history whenever it exists.
func compactIdenticalActivityAttempts(execution map[string]any) bool {
	activities, ok := execution["activities"].([]any)
	if !ok || len(activities) == 0 || !reflect.DeepEqual(activities, execution["activityAttempts"]) {
		return false
	}
	delete(execution, "activityAttempts")
	return true
}
