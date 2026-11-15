package resource

func (r Resource) HasLocalAllocation() bool {
	enabled, _ := r.Metadata["localAllocation"].(bool)
	return r.Type == ResourceLocalMachine && enabled
}

func (r Resource) LocalActivityBudget(cpu float64, memory int64) (float64, int64) {
	if cpu < 1 {
		cpu = 1
	}
	if memory <= 0 {
		memory = r.MemoryBytes / int64(max(1, r.SchedulingCPUCores()))
	}
	return cpu, memory
}
