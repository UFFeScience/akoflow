package environment

import "testing"

func TestLocalCapacityRecommendationsAndBounds(t *testing.T) {
	capacity, err := NewLocalCapacity(8, 32<<30, "docker")
	if err != nil || capacity.RecommendedCPUs != 5 || capacity.RecommendedMemoryBytes > 32<<30 {
		t.Fatalf("capacity=%+v err=%v", capacity, err)
	}
	for _, cpu := range []float64{0, 0.5, 9} {
		if capacity.Validate(cpu, 1<<30) == nil {
			t.Fatalf("invalid CPU accepted: %v", cpu)
		}
	}
	for _, memory := range []int64{0, 128 << 20, 33 << 30} {
		if capacity.Validate(1, memory) == nil {
			t.Fatalf("invalid memory accepted: %v", memory)
		}
	}
	if err := capacity.Validate(8, 32<<30); err != nil {
		t.Fatal(err)
	}
	single, err := NewLocalCapacity(1, 256<<20, "native")
	if err != nil || single.RecommendedCPUs != 1 || single.RecommendedMemoryBytes != single.MinMemoryBytes {
		t.Fatalf("small device=%+v err=%v", single, err)
	}
	if _, err := NewLocalCapacity(0, 0, "native"); err == nil {
		t.Fatal("missing capacity accepted")
	}
}
