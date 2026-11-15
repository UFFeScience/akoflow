package environment

import (
	"fmt"
	"math"
)

type LocalCapacity struct {
	MaxCPUs                int    `json:"maxCpus"`
	MinCPUs                int    `json:"minCpus"`
	MaxMemoryBytes         int64  `json:"maxMemoryBytes"`
	MinMemoryBytes         int64  `json:"minMemoryBytes"`
	RecommendedCPUs        int    `json:"recommendedCpus"`
	RecommendedMemoryBytes int64  `json:"recommendedMemoryBytes"`
	Source                 string `json:"source"`
}

func NewLocalCapacity(cpus int, memory int64, source string) (*LocalCapacity, error) {
	if cpus < 1 || memory < 1 {
		return nil, fmt.Errorf("local CPU and memory capacity could not be detected")
	}
	minimum := min(int64(256<<20), memory)
	recommended := max(minimum, (memory/10*7)/(1<<20)*(1<<20))
	return &LocalCapacity{
		MinCPUs: 1, MaxCPUs: cpus, MinMemoryBytes: minimum, MaxMemoryBytes: memory,
		RecommendedCPUs: max(1, cpus*7/10), RecommendedMemoryBytes: recommended, Source: source,
	}, nil
}

func (c LocalCapacity) Validate(cpu float64, memory int64) error {
	if math.IsNaN(cpu) || math.IsInf(cpu, 0) || math.Trunc(cpu) != cpu || cpu < float64(c.MinCPUs) || cpu > float64(c.MaxCPUs) {
		return fmt.Errorf("local CPU allocation must be a whole number between %d and %d", c.MinCPUs, c.MaxCPUs)
	}
	if memory < c.MinMemoryBytes || memory > c.MaxMemoryBytes {
		return fmt.Errorf("local memory allocation must be between %d and %d bytes", c.MinMemoryBytes, c.MaxMemoryBytes)
	}
	return nil
}
