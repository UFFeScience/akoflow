package parser

import "encoding/base64"

func Base64ToWorkflow(workflowBase64 string) string {
	byteWorkflow, _ := base64.StdEncoding.DecodeString(workflowBase64)

	stringWorkflow := string(byteWorkflow)

	return stringWorkflow
}
