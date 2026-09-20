package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sort"
)

type instance struct {
	Name     string `json:"name"`
	Workflow struct {
		Specification struct {
			Tasks []struct {
				ID          string   `json:"id"`
				Parents     []string `json:"parents"`
				InputFiles  []string `json:"inputFiles"`
				OutputFiles []string `json:"outputFiles"`
			} `json:"tasks"`
			Files []struct {
				ID          string `json:"id"`
				SizeInBytes int64  `json:"sizeInBytes"`
			} `json:"files"`
		} `json:"specification"`
		Execution struct {
			Tasks []struct {
				ID               string   `json:"id"`
				RuntimeInSeconds float64  `json:"runtimeInSeconds"`
				AvgCPU           float64  `json:"avgCPU"`
				Machines         []string `json:"machines"`
			} `json:"tasks"`
			Machines []struct {
				NodeName string `json:"nodeName"`
				CPU      struct {
					SpeedInMHz float64 `json:"speedInMHz"`
				} `json:"cpu"`
			} `json:"machines"`
		} `json:"execution"`
	} `json:"workflow"`
}

type portableWorkflow struct {
	Name string       `json:"name"`
	Spec portableSpec `json:"spec"`
}

type portableSpec struct {
	Namespace        string                   `json:"namespace"`
	Activities       []portableActivity       `json:"activities"`
	DataDependencies []portableDataDependency `json:"dataDependencies,omitempty"`
}

type portableActivity struct {
	Name        string             `json:"name"`
	Runtime     string             `json:"runtime"`
	Run         string             `json:"run"`
	CPULimit    string             `json:"cpuLimit"`
	MemoryLimit string             `json:"memoryLimit"`
	DependsOn   []string           `json:"dependsOn,omitempty"`
	Simulation  portableSimulation `json:"simulation"`
}

type portableSimulation struct {
	Model           string         `json:"model"`
	DurationSeconds float64        `json:"durationSeconds"`
	Parameters      map[string]any `json:"parameters"`
}

type portableDataDependency struct {
	ProducerActivity string `json:"producerActivity"`
	ConsumerActivity string `json:"consumerActivity"`
	LogicalName      string `json:"logicalName"`
	SizeBytes        int64  `json:"sizeBytes"`
}

func main() {
	source := flag.String("source", "", "WfCommons JSON file or HTTPS URL")
	name := flag.String("name", "", "AkôFlow workflow name")
	output := flag.String("output", "-", "output JSON file or -")
	flopsPerCycle := flag.Float64("source-flops-per-cycle", 16, "assumed source CPU FP64 operations per cycle")
	memoryMultiplier := flag.Float64("memory-multiplier", 4, "working-set multiplier over input bytes")
	minimumMemory := flag.Int64("minimum-memory", 256<<20, "minimum modeled memory in bytes")
	maximumMemory := flag.Int64("maximum-memory", 16<<30, "maximum modeled memory in bytes")
	flag.Parse()
	if *source == "" || *name == "" {
		fail(fmt.Errorf("-source and -name are required"))
	}
	reader, closeReader, err := open(*source)
	if err != nil {
		fail(err)
	}
	defer closeReader()
	var value instance
	if err := json.NewDecoder(reader).Decode(&value); err != nil {
		fail(fmt.Errorf("decode WfCommons instance: %w", err))
	}
	workflow, err := convert(value, *name, *flopsPerCycle, *memoryMultiplier, *minimumMemory, *maximumMemory)
	if err != nil {
		fail(err)
	}
	writer := os.Stdout
	if *output != "-" {
		writer, err = os.Create(*output)
		if err != nil {
			fail(err)
		}
		defer writer.Close()
	}
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(workflow); err != nil {
		fail(err)
	}
}

func convert(value instance, name string, flopsPerCycle, memoryMultiplier float64, minimumMemory, maximumMemory int64) (portableWorkflow, error) {
	executions := map[string]struct {
		runtime float64
		avgCPU  float64
		machine string
	}{}
	for _, task := range value.Workflow.Execution.Tasks {
		machine := ""
		if len(task.Machines) > 0 {
			machine = task.Machines[0]
		}
		executions[task.ID] = struct {
			runtime float64
			avgCPU  float64
			machine string
		}{task.RuntimeInSeconds, task.AvgCPU, machine}
	}
	clockMHz := map[string]float64{}
	averageClock := 0.0
	for _, machine := range value.Workflow.Execution.Machines {
		clockMHz[machine.NodeName] = machine.CPU.SpeedInMHz
		averageClock += machine.CPU.SpeedInMHz
	}
	if len(clockMHz) > 0 {
		averageClock /= float64(len(clockMHz))
	}
	fileSizes := map[string]int64{}
	for _, file := range value.Workflow.Specification.Files {
		fileSizes[file.ID] = file.SizeInBytes
	}
	tasks := map[string]struct {
		inputs  []string
		outputs []string
	}{}
	workflow := portableWorkflow{Name: name, Spec: portableSpec{Namespace: "wfcommons-pegasus"}}
	for _, task := range value.Workflow.Specification.Tasks {
		execution, exists := executions[task.ID]
		if !exists || execution.runtime <= 0 {
			return portableWorkflow{}, fmt.Errorf("task %q has no positive observed runtime", task.ID)
		}
		clock := clockMHz[execution.machine]
		if clock <= 0 {
			clock = averageClock
		}
		sourceGFLOPS := clock / 1000 * flopsPerCycle
		normalizedRuntime := execution.runtime * sourceGFLOPS
		inputBytes := int64(0)
		for _, file := range task.InputFiles {
			inputBytes += fileSizes[file]
		}
		memory := int64(math.Ceil(float64(inputBytes) * memoryMultiplier))
		memory = max(memory, minimumMemory)
		memory = min(memory, maximumMemory)
		cpu := max(1, int(math.Ceil(execution.avgCPU/100)))
		workflow.Spec.Activities = append(workflow.Spec.Activities, portableActivity{
			Name:        task.ID,
			Runtime:     "simgrid",
			Run:         "simulate",
			CPULimit:    fmt.Sprintf("%d", cpu),
			MemoryLimit: fmt.Sprintf("%d", memory),
			DependsOn:   task.Parents,
			Simulation: portableSimulation{
				Model:           "wfcommons-observed-runtime-normalized",
				DurationSeconds: normalizedRuntime,
				Parameters: map[string]any{
					"sourceRuntimeSeconds": execution.runtime,
					"sourceMachine":        execution.machine,
					"sourceClockMHz":       clock,
					"sourceGFLOPSPerCore":  sourceGFLOPS,
					"memoryEstimation":     "clamp(4*inputBytes,256MiB,16GiB)",
					"inputBytes":           inputBytes,
				},
			},
		})
		tasks[task.ID] = struct {
			inputs  []string
			outputs []string
		}{task.InputFiles, task.OutputFiles}
	}
	for _, task := range value.Workflow.Specification.Tasks {
		consumerFiles := stringSet(task.InputFiles)
		for _, parent := range task.Parents {
			sharedBytes := int64(0)
			for _, file := range tasks[parent].outputs {
				if consumerFiles[file] {
					sharedBytes += fileSizes[file]
				}
			}
			if sharedBytes > 0 {
				workflow.Spec.DataDependencies = append(workflow.Spec.DataDependencies, portableDataDependency{
					ProducerActivity: parent,
					ConsumerActivity: task.ID,
					LogicalName:      parent + "-to-" + task.ID,
					SizeBytes:        sharedBytes,
				})
			}
		}
	}
	sort.SliceStable(workflow.Spec.Activities, func(i, j int) bool {
		return workflow.Spec.Activities[i].Name < workflow.Spec.Activities[j].Name
	})
	return workflow, nil
}

func stringSet(values []string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}

func open(source string) (io.Reader, func(), error) {
	if len(source) >= 8 && source[:8] == "https://" {
		response, err := http.Get(source)
		if err != nil {
			return nil, func() {}, err
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			response.Body.Close()
			return nil, func() {}, fmt.Errorf("GET %s returned %s", source, response.Status)
		}
		return response.Body, func() { response.Body.Close() }, nil
	}
	file, err := os.Open(source)
	if err != nil {
		return nil, func() {}, err
	}
	return file, func() { file.Close() }, nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "wfcommons-convert:", err)
	os.Exit(1)
}
