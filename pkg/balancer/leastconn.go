package balancer

import (
	"context"
	"sync"

	"github.com/PhVHoang/cache-coordinator/pkg/errors"
	"github.com/PhVHoang/cache-coordinator/pkg/registry"
)

// LeastConnectedBalancer implements least connections load balancing
type LeastConnectedBalancer struct {
	connections map[string]int
	mu sync.RWMutex
}

// NewLeastConnectedBalancer creates a new least connections load balancer
func NewLeastConnectedLoadBalancer() LoadBalancer {
	return &LeastConnectedBalancer{
		connections:make(map[string]int),
	}
}

// Select implements LoadBalancer interface
func (lb *LeastConnectedBalancer) Select(ctx context.Context, services []*registry.ServiceInfo) (*registry.ServiceInfo, error) {
	if len(services) == 0 {
		return nil, errors.ErrNoServicesAvailable
	}
	
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	
	var selected *registry.ServiceInfo
	minConnections := int(^uint(0) >> 1) // Max int
	
	for _, service := range services {
		connections := lb.connections[service.ID]
		if connections < minConnections {
			minConnections = connections
			selected = service
		}
	}
	
	// Increment connection count for selected service
	lb.mu.RUnlock()
	lb.mu.Lock()
	lb.connections[selected.ID]++
	lb.mu.Unlock()
	lb.mu.RLock()
	
	return selected, nil
}

// Name implements LoadBalancer interface
func (lb *LeastConnectedBalancer) Name() string {
	return string(StrategyLeastConnected)
}
