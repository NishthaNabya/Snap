package snap

import (
	"fmt"
	"sort"
	"sync"
)

// ConfigEntry represents a single driver+source pair from config.json.
type ConfigEntry struct {
	Driver string `json:"driver"`
	Source string `json:"source"`
}

// ResolvedDriver pairs a looked-up StateDriver with its source locator.
type ResolvedDriver struct {
	Driver    StateDriver
	Source    string
	ConfigIdx int // Original index in config, for stable ordering.
}

// DriverRegistry is a thread-safe registry of StateDriver
// implementations. Drivers self-register at init() time.
type DriverRegistry struct {
	mu      sync.RWMutex
	drivers map[string]StateDriver
}

// NewRegistry creates an empty DriverRegistry.
func NewRegistry() *DriverRegistry {
	return &DriverRegistry{
		drivers: make(map[string]StateDriver),
	}
}

// Registry is the global default driver registry.
var Registry = NewRegistry()

// Register adds a driver to the registry. Panics on duplicate names.
func (r *DriverRegistry) Register(d StateDriver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := d.Name()
	if _, exists := r.drivers[name]; exists {
		panic(fmt.Sprintf("snap: duplicate driver registration: %q", name))
	}
	r.drivers[name] = d
}

// Get retrieves a driver by name. Returns an error if not found.
func (r *DriverRegistry) Get(name string) (StateDriver, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.drivers[name]
	if !ok {
		return nil, fmt.Errorf("snap: unknown driver: %q", name)
	}
	return d, nil
}

// Resolve takes a list of ConfigEntry pairs from config and returns
// the corresponding StateDriver instances sorted by ascending Priority.
//
// Execution order guarantee:
//
//	PriorityEnvironment (100) → PriorityDatabase (200)
//	Within the same priority tier, order is stable (config order).
func (r *DriverRegistry) Resolve(entries []ConfigEntry) ([]ResolvedDriver, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	resolved := make([]ResolvedDriver, 0, len(entries))
	for i, e := range entries {
		d, ok := r.drivers[e.Driver]
		if !ok {
			return nil, fmt.Errorf("snap: unknown driver: %q", e.Driver)
		}
		resolved = append(resolved, ResolvedDriver{
			Driver:    d,
			Source:    e.Source,
			ConfigIdx: i,
		})
	}

	// Stable sort: preserves config order within the same priority.
	sort.SliceStable(resolved, func(i, j int) bool {
		return resolved[i].Driver.Priority() < resolved[j].Driver.Priority()
	})

	return resolved, nil
}
