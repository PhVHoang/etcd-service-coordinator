package balancer

import (
	"context"

	"github.com/PhVHoang/cache-coordinator/pkg/health"
)

// LoadBalancer defines the load balancing interface
type LoadBalancer interface {
	// Select chooses a service instance from the available ones
	Select(ctx context.Context, services []*health.ServiceInfo) (*health.ServiceInfo, error)
	
	// Name returns the load balancer strategy name
	Name() string
}

// Strategy represents different load balancing strategies
type Strategy string

const (
	StrategyRoundRobin     Strategy = "round_robin"
	StrategyRandom         Strategy = "random"
	StrategyLeastConnected Strategy = "least_connected"
	StrategyWeighted       Strategy = "weighted"
)
