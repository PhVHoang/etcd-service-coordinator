package balancer

import (
	"context"
	"math/rand"
	"time"

	"github.com/PhVHoang/cache-coordinator/pkg/errors"
	"github.com/PhVHoang/cache-coordinator/pkg/registry"
)

// RandomBalancer implements random load balancing
type RandomBalancer struct {
	rng *rand.Rand
}

// NewRandomBalancer creates a new random load balancer
func NewRandomBalancer() LoadBalancer {
	return &RandomBalancer{
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Select implements LoadBalancer interface
func (rb *RandomBalancer) Select(ctx context.Context, services []*registry.ServiceInfo) (*registry.ServiceInfo, error) {
	if len(services) == 0 {
		return nil, errors.ErrNoServicesAvailable
	}
	
	index := rb.rng.Intn(len(services))
	return services[index], nil
}

// Name implements LoadBalancer interface
func (rb *RandomBalancer) Name() string {
	return string(StrategyRandom)
}
