package parser_pegasus_workflow

import (
	"crypto/tls"
	"strings"

	"github.com/ovvesley/akoflow/pkg/server/entities/workflow_activity_entity"
	workflow "github.com/ovvesley/akoflow/pkg/server/entities/workflow_entity"

	"io"
	"net/http"
	"os"

	"github.com/ovvesley/akoflow/pkg/client/entities/pegasus_workflow"
	"github.com/ovvesley/akoflow/pkg/client/utils/utils_read_file"
	"gopkg.in/yaml.v3"
)

type ParserPegasusWorkflowService struct {
	inputFile  string
	outputFile string
}

func NewService() *ParserPegasusWorkflowService {
	return &ParserPegasusWorkflowService{}
}

func (p *ParserPegasusWorkflowService) SetInputFile(inputFile string) *ParserPegasusWorkflowService {
	p.inputFile = inputFile
	return p
}

func (p *ParserPegasusWorkflowService) SetOutputFile(outputFile string) *ParserPegasusWorkflowService {
	p.outputFile = outputFile
	return p
}

func (p *ParserPegasusWorkflowService) GetInputFile() string {
	return p.inputFile
}

func (p *ParserPegasusWorkflowService) GetOutputFile() string {
	return p.outputFile
}

func (p *ParserPegasusWorkflowService) Parser() {
	textInputFile := utils_read_file.New().ReadFile(p.inputFile)
	yamlWorkflow := pegasus_workflow.PegasusWorkflow{}
	err := yaml.Unmarshal([]byte(textInputFile), &yamlWorkflow)

	if err != nil {
		panic(err)
	}

	workflow := p.makeWorkflow(yamlWorkflow)

	//println("Workflow: ", string(workflow.GetBase64Workflow()))

	//	write to file

	writeFile, err := os.Create(p.outputFile)
	if err != nil {
		panic(err)
	}

	defer writeFile.Close()

	encoder := yaml.NewEncoder(writeFile)
	err = encoder.Encode(workflow)
	if err != nil {
		panic(err)
	}

	encoder.Close()

	println("Workflow written to file: ", p.outputFile)

}

func (p *ParserPegasusWorkflowService) makeWorkflow(yamlWorkflow pegasus_workflow.PegasusWorkflow) workflow.Workflow {

	actvities := []workflow_activity_entity.WorkflowActivities{}

	mapNameToID := p.makeMapNameToID(yamlWorkflow)

	for _, job := range yamlWorkflow.Jobs {

		commandToRun := job.Name + " " + strings.Join(job.Arguments, " ")
		//commandToRunBase64 := base64.StdEncoding.EncodeToString([]byte(commandToRun))

		activity := workflow_activity_entity.WorkflowActivities{
			Name:        mapNameToID[job.ID],
			Run:         commandToRun,
			MemoryLimit: "256Mi",
			CpuLimit:    "500m",
			DependsOn:   p.makeDependsOn(job.ID, yamlWorkflow, mapNameToID),
		}
		actvities = append(actvities, activity)
	}

	workflow := workflow.Workflow{}
	workflow.Name = "wf-montage"
	workflow.Spec.Namespace = "akoflow"
	workflow.Spec.MountPath = "/data"
	workflow.Spec.StorageClassName = "hostpath"
	workflow.Spec.StorageSize = "1Gi"
	workflow.Spec.Image = "ovvesley/akoflow-wf-montage:latest"

	workflow.Spec.Activities = actvities
	return workflow
}

func (p *ParserPegasusWorkflowService) makeMapNameToID(yamlWorkflow pegasus_workflow.PegasusWorkflow) map[string]string {
	mapNameToID := map[string]string{}

	for _, job := range yamlWorkflow.Jobs {
		mapNameToID[job.ID] = p.makeName(job.ID, job.Name)
	}

	return mapNameToID
}

func (p *ParserPegasusWorkflowService) makeName(jobID string, jobName string) string {
	return strings.ToLower(jobName + jobID)
}

func (p *ParserPegasusWorkflowService) makeDependsOn(jobID string, yamlWorkflow pegasus_workflow.PegasusWorkflow, mapNameToID map[string]string) []string {
	dependsOn := []string{}

	for _, job := range yamlWorkflow.JobDependencies {
		for _, dependency := range job.Children {
			if dependency == jobID {
				dependsOn = append(dependsOn, mapNameToID[job.ID])
			}
		}
	}

	return dependsOn

}

func (p *ParserPegasusWorkflowService) downloadCatalogFiles(filesReplicasCatalog map[string]string) {

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	for filename, pfn := range filesReplicasCatalog {
		println("Downloading file: ", filename)
		println("From: ", pfn)

		get, err := client.Get(pfn)
		if err != nil {
			return
		}

		defer get.Body.Close()
		// write the body to file

		// create directory if not exists
		if _, err := os.Stat("catalog-075"); os.IsNotExist(err) {
			os.Mkdir("catalog", os.ModePerm)
		}

		file, err := os.Create("catalog-075/" + filename)

		if err != nil {
			return
		}

		defer file.Close()

		_, err = io.Copy(file, get.Body)

		if err != nil {
			return
		}

		println("Downloaded file: ", filename)

	}
}

func (p *ParserPegasusWorkflowService) getFilesReplicasCatalog(yamlWorkflow pegasus_workflow.PegasusWorkflow) map[string]string {
	filesReplicasCatalog := map[string]string{}

	for _, replica := range yamlWorkflow.ReplicaCatalog.Replicas {
		if len(replica.Pfns) == 0 {
			continue
		}

		if replica.Pfns[0].Site == "ipac" {
			filename := replica.Lfn
			filesReplicasCatalog[filename] = replica.Pfns[0].Pfn
		}

	}

	return filesReplicasCatalog
}
