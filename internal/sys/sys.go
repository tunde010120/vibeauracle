package sys

import (
	"fmt"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

// Snapshot represents the current system state
type Snapshot struct {
	CPUUsage    float64
	MemoryUsage float64
}

// Monitor provides system awareness
type Monitor struct{}

func NewMonitor() *Monitor {
	return &Monitor{}
}

// GetSnapshot returns a current snapshot of system resources
func (m *Monitor) GetSnapshot() (Snapshot, error) {
	c, err := cpu.Percent(0, false)
	if err != nil {
		return Snapshot{}, fmt.Errorf("getting cpu percent: %w", err)
	}

	vm, err := mem.VirtualMemory()
	if err != nil {
		return Snapshot{}, fmt.Errorf("getting virtual memory: %w", err)
	}

	return Snapshot{
		CPUUsage:    c[0],
		MemoryUsage: vm.UsedPercent,
	}, nil
}

