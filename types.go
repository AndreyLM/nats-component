package component

import (
	"runtime"
	"time"
)

// Stats - component stats
type Stats struct {
	Kind     string           `json:"kind"`
	ID       string           `json:"id"`
	Cmd      []string         `json:"cmdline"`
	MemStats runtime.MemStats `json:"memstats"`
	System   SystemStats
}

// SystemStats - system stats
type SystemStats struct {
	MemoryTotal uint64 `json:"memory_total"`
	MemoryUsed  uint64 `json:"memory_used"`
	MemoryFree  uint64 `json:"memory_free"`
	CPUTotal    uint64 `json:"cpu_total"`
	CPUSystem   uint64 `json:"cpu_system"`
	CPUCount    int    `json:"cpu_count"`
}

// Info - main component info
type Info struct {
	ID      string    `json:"id"`
	Kind    string    `json:"kind"`
	Started time.Time `json:"time"`
}
