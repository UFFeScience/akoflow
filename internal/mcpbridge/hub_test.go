package mcpbridge

import "testing"

func TestHubCatalogContractsAndPermissions(t *testing.T) {
	operations := map[string]Operation{}
	for _, operation := range Catalog() {
		operations[operation.Name] = operation
	}
	expected := map[string]Permission{
		"hub_status": Read, "hub_list_publications": Read, "hub_get_publication": Read,
		"hub_list_versions": Read, "hub_list_results": Read, "hub_compare_versions": Read,
		"hub_get_workflow_definition": Read, "inspect_artifact_image": Read,
		"hub_update_publication": Write, "hub_import_workflow": Write,
		"hub_preview_workflow": Write, "hub_preview_execution": Write, "hub_confirm_publication": Write,
		"hub_delete_publication": Admin, "hub_disconnect": Admin, "hub_update_settings": Admin,
		"get_execution_status": Read, "get_execution_logs": Read, "get_execution_events": Read,
		"get_execution_artifacts": Read, "get_execution_activity": Read,
	}
	for name, permission := range expected {
		operation, ok := operations[name]
		if !ok || operation.Permission() != permission {
			t.Errorf("%s missing or permission mismatch: %+v", name, operation)
		}
	}
	patch := operations["hub_update_publication"].Body["properties"].(map[string]any)
	for _, field := range []string{"title", "visibility", "acknowledgePublic", "imageTags"} {
		if patch[field] == nil {
			t.Errorf("patch omitted %s", field)
		}
	}
	recipe := operations["register_artifact_recipe"].Body["properties"].(map[string]any)
	if recipe["dockerfile"] == nil || recipe["contextArchive"] == nil {
		t.Fatal("recipe tool cannot register portable JSON sources")
	}
}
