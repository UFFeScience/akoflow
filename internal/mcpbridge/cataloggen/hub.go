package main

func hubToolName(handler string) string {
	names := map[string]string{
		"serveStatus":                  "hub_status",
		"serveSettings":                "hub_update_settings",
		"serveStart":                   "hub_start_authorization",
		"servePoll":                    "hub_poll_authorization",
		"serveDisconnect":              "hub_disconnect",
		"servePublications":            "hub_list_publications",
		"serveCreate":                  "hub_create_publication",
		"serveGetPublication":          "hub_get_publication",
		"serveUpdatePublication":       "hub_update_publication",
		"serveDeletePublication":       "hub_delete_publication",
		"serveContributionDestination": "hub_get_contribution_destination",
		"serveListVersions":            "hub_list_versions",
		"serveListResults":             "hub_list_results",
		"serveCompareVersions":         "hub_compare_versions",
		"servePreview":                 "hub_preview_execution",
		"servePreviewWorkflow":         "hub_preview_workflow",
		"serveConfirm":                 "hub_confirm_publication",
		"serveWorkflowDefinition":      "hub_get_workflow_definition",
		"serveImportWorkflow":          "hub_import_workflow",
	}
	if name := names[handler]; name != "" {
		return name
	}
	panic("missing Hub MCP tool name: " + handler)
}
