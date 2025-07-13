package health

import (
	"context"
	"time"

	"github.com/PhVHoang/cache-coordinator/pkg/registry"
)

type Checker interface {
	// Check performs a health check on a service
	Check(ctx context.Context, service *registry.ServiceInfo) error

	// StartMonitoring begins continuous health monitoring
	StartMonitoring(ctx context.Context, interval time.Duration) error

	// StopMonitoring stops health monitoring
	StopMonitoring() error
}

// CheckStrategy defines different health check strategies
type CheckStrategy string

const (
	HttpStrategy CheckStrategy = "http"
	TCPStrategy CheckStrategy = "tcp"
	GRPCStrategy CheckStrategy = "grpc"
)
