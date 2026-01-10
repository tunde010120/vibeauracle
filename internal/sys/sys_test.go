package sys

import (
	"testing"
)

func TestMonitor_GetSnapshot(t *testing.T) {
	m := NewMonitor()
	snapshot, err := m.GetSnapshot()
	if err != nil {
		t.Fatalf("Failed to get snapshot: %v", err)
	}

	if snapshot.CPUUsage < 0 || snapshot.CPUUsage > 100 {
		t.Errorf("Invalid CPU usage: %f", snapshot.CPUUsage)
	}

	if snapshot.MemoryUsage < 0 || snapshot.MemoryUsage > 100 {
		t.Errorf("Invalid Memory usage: %f", snapshot.MemoryUsage)
	}
}

