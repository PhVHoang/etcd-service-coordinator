package registry

import (
	"context"
	"time"
)

type ServiceInfo struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Address  string            `json:"address"`
	Port     int               `json:"port"`
	Health   HealthStatus      `json:"health"`
	Metadata map[string]string `json:"metadata"`
	TTL      time.Duration     `json:"ttl"`
}

type HealthStatus string

const (
	HealthStatusUnknown  HealthStatus = "unknown"
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// ServiceRegistry defines the main registry interface
type ServiceRegistry interface {
	// Register registers a service instance
	Register(ctx context.Context, service *ServiceInfo) error
	
	// Deregister removes a service instance
	Deregister(ctx context.Context, serviceID string) error
	
	// Discover finds all instances of a service
	Discover(ctx context.Context, serviceName string) ([]*ServiceInfo, error)
	
	// GetHealthy returns only healthy service instances
	GetHealthy(ctx context.Context, serviceName string) ([]*ServiceInfo, error)
	
	// Watch watches for service changes
	Watch(ctx context.Context, serviceName string) (<-chan []*ServiceInfo, error)
	
	// Close gracefully shuts down the registry
	Close() error
}
