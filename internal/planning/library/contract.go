package library

// Contract is compiled alongside the user function. It only uses the standard library.
const Contract = `package main
import "encoding/json"
type Input struct {
 Request Request ` + "`json:\"request\"`" + `
 Configuration map[string]any ` + "`json:\"configuration\"`" + `
}
type Request struct {
 Workflow Workflow ` + "`json:\"workflow\"`" + `
 Resources []Resource ` + "`json:\"resources\"`" + `
 ExecutionScope Scope ` + "`json:\"executionScope\"`" + `
 NetworkTopology json.RawMessage ` + "`json:\"networkTopology\"`" + `
 ActivityProfiles []json.RawMessage ` + "`json:\"activityProfiles\"`" + `
 DeadlineSeconds float64 ` + "`json:\"deadlineSeconds\"`" + `
 Budget float64 ` + "`json:\"budget\"`" + `
 Interference json.RawMessage ` + "`json:\"interference\"`" + `
}
type Workflow struct {
 ID string ` + "`json:\"id\"`" + `
 Activities []Activity ` + "`json:\"activities\"`" + `
 Dependencies []Dependency ` + "`json:\"dependencies\"`" + `
 DataDependencies []json.RawMessage ` + "`json:\"dataDependencies\"`" + `
}
type Activity struct {
 ID string ` + "`json:\"id\"`" + `
 Name string ` + "`json:\"name\"`" + `
 ActivityTypeID string
 Resources Requirements ` + "`json:\"resources\"`" + `
 Simulation *Simulation ` + "`json:\"simulation\"`" + `
 Metadata map[string]any ` + "`json:\"metadata\"`" + `
}
type Requirements struct { CPU float64; MemoryBytes int64; StorageBytes int64 }
type Simulation struct { Model string; DurationSeconds float64; FLOPs float64; Parameters map[string]any }
type Dependency struct { ActivityID string; DependsOnActivityID string }
type Scope struct { ID string; EnvironmentVersionIDs []string }
type Resource struct {
 ID string; Name string; EnvironmentVersionID string; ExecutionTarget string; BootOverheadSeconds float64; ContainerOverheadSeconds float64; Type string; ProviderID string; Tier string; Region string; CPUCores int; CPUCapacity float64; MemoryBytes int64; StorageBytes int64
 Metadata map[string]any
 ComputeSpeedup float64; PricePerSecond float64; Schedulable bool
}
type Plan struct {
 ID string ` + "`json:\"id\"`" + `
 WorkflowVersionID string ` + "`json:\"workflowVersionId\"`" + `
 ExecutionScopeID string ` + "`json:\"executionScopeId\"`" + `
 Algorithm string ` + "`json:\"algorithm\"`" + `
 Objective string ` + "`json:\"objective\"`" + `
 DeadlineSeconds float64; Budget float64; NetworkTopologyID string
 Predicted Metrics ` + "`json:\"predicted\"`" + `
 Assignments []Assignment ` + "`json:\"assignments\"`" + `
 Metadata map[string]any ` + "`json:\"metadata,omitempty\"`" + `
}
type Metrics struct { MakespanSeconds float64; Cost float64; Feasible bool }
type Assignment struct {
 Metadata map[string]any
 ID string; PlanID string; ActivityID string; ResourceID string; CoreID string
 OrderOnResource int; Priority int
 PredictedReadyAt float64; PredictedStartAt float64; PredictedFinishAt float64
 PredictedRuntimeSeconds float64; PredictedTransferSeconds float64; PredictedCost float64
}
`

const runner = `package main
import (
 "encoding/json"
 "fmt"
 "os"
)
func main() {
 var input Input
 if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
 plans, err := Schedule(input)
 if err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
 if err := json.NewEncoder(os.Stdout).Encode(plans); err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
}
`

const Template = `package main

import "fmt"

// Schedule receives a frozen workflow, resources, scope and configuration.
// Return complete plans: every activity needs exactly one assignment.
// Times are in seconds; memory is in bytes. All plans are validated by AkôFlow.
func Schedule(input Input) ([]Plan, error) {
 return nil, fmt.Errorf("implement your scheduling strategy here")
}
`
