package balancer

import (
	"context"
	"sync/atomic"

	customErrors "github.com/PhVHoang/cache-coordinator/pkg/errors"
	"github.com/PhVHoang/cache-coordinator/pkg/health"
)

// RoundRobinBalancer implements a round-robin load balancing strategy
type RoundRobinBalancer struct {
	counter uint64
}

// NewRoundRobinBalancer creates a new RoundRobinBalancer instance
func NewRoundRobinBalancer() *RoundRobinBalancer {
	return &RoundRobinBalancer{
		counter: 0,
	}
}

// Select chooses a service instance using the round-robin strategy
func (b *RoundRobinBalancer) Select(ctx context.Context, services []*health.ServiceInfo) (*health.ServiceInfo, error) {
	if len(services) == 0 {
		return nil, customErrors.ErrNoServicesAvailable
	}

	// Atomic increment to ensure thread safety
	index := atomic.AddUint64(&b.counter, 1)
	selectedIndex := (index - 1) % uint64(len(services))

	return services[selectedIndex], nil
}

// Name implements LoadBalancer interface
func (lb *RoundRobinBalancer) Name() string {
	return string(StrategyRoundRobin)
}
