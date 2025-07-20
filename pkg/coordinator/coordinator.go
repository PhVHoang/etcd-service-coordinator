package coordinator

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/PhVHoang/cache-coordinator/pkg/balancer"
	"github.com/PhVHoang/cache-coordinator/pkg/health"
	"github.com/PhVHoang/cache-coordinator/pkg/storage"
	"go.uber.org/zap"
	"honnef.co/go/tools/config"
)

// Coordinator orchestrates service coordination functionality
type Coordinator struct {
	storage      storage.Storage
	balancer     balancer.LoadBalancer
	healthChecker health.Checker
	config       *config.Config
	logger       *zap.Logger
	
	services map[string]*health.ServiceInfo
	mu       sync.RWMutex

	// For graceful shutdown only - NOT for operation contexts
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc

	// Track background goroutines for cleanup
	wg sync.WaitGroup
}

type Options struct {
	Storage storage.Storage
	LoadBalancer balancer.LoadBalancer
	HealthChecker health.Checker
	Config *config.Config
	Logger *zap.Logger
}

func NewCoordinator(ctx context.Context, opts Options) (*Coordinator, error) {
	if opts.Storage == nil {
		return nil, fmt.Errorf("storage backend is required")
	}

	if opts.LoadBalancer == nil {
		opts.LoadBalancer = balancer.NewRoundRobinBalancer()
	}

	if opts.Logger == nil {
		var err error
		opts.Logger, err = zap.NewProduction()
		if err != nil {
			return nil, fmt.Errorf("failed to create zap logger: %w", err)
		}
	}

	// This context is ONLY for coordinator lifecycle/shutdown
	shutdownCtx, cancel := context.WithCancel(context.Background())

	c := &Coordinator{
		storage:       opts.Storage,
		balancer:      opts.LoadBalancer,
		healthChecker: opts.HealthChecker,
		config:        opts.Config,
		logger:        opts.Logger,
		services:      make(map[string]*health.ServiceInfo),
		shutdownCtx:   shutdownCtx,
		shutdownCancel: cancel,
	}

	// Start background health checking if health checker is provided
	if opts.HealthChecker != nil {
		c.healthChecker.StartMonitoring(ctx, time.Duration(time.Duration(30).Seconds()))
	}

	return c, nil
}

// Register implements ServiceRegistry interface
func (c *Coordinator) Register(ctx context.Context, service *health.ServiceInfo) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Store in backend
	key := c.serviceKey(service.Name, service.ID)
	value, err := c.serializeService(service)
	if err != nil {
		return fmt.Errorf("failed to serialize service: %w", err)
	}
	
	if err := c.storage.Put(ctx, key, value, service.TTL); err != nil {
		return fmt.Errorf("failed to store service: %w", err)
	}
	
	// Store locally
	c.services[service.ID] = service
	
	c.logger.Info("Service registered",
		zap.String("service_id", service.ID),
		zap.String("service_name", service.Name),
		zap.String("address", fmt.Sprintf("%s:%d", service.Address, service.Port)),
	)

	return nil
}

// Discover implements ServiceRegistry interface
func (c *Coordinator) Discover(ctx context.Context, serviceName string) ([]*health.ServiceInfo, error) {
	prefix := c.servicePrefix(serviceName)
	data, err := c.storage.List(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to discover services: %w", err)
	}
	
	var services []*health.ServiceInfo
	for _, value := range data {
		service, err := c.deserializeService(value)
		if err != nil {
			c.logger.Error("Failed to deserialize service",
				zap.Error(err),
			)
			continue
		}
		services = append(services, service)
	}
	
	return services, nil
}

// GetHealthy implements ServiceRegistry interface
func (c *Coordinator) GetHealthy(ctx context.Context, serviceName string) ([]*health.ServiceInfo, error) {
	allServices, err := c.Discover(ctx, serviceName)
	if err != nil {
		return nil, err
	}
	
	var healthyServices []*health.ServiceInfo
	for _, service := range allServices {
        // Use the health checker if available
        if c.healthChecker != nil {
            isHealthy, err := c.healthChecker.Check(ctx, service)
            if isHealthy && err == nil {
                service.Health = health.HealthStatusHealthy
                healthyServices = append(healthyServices, service)
            }
        } else if service.Health == health.HealthStatusHealthy {
            healthyServices = append(healthyServices, service)
        }
    }
    
    return healthyServices, nil
}

// Watch implements ServiceRegistry interface
func (c *Coordinator) Watch(ctx context.Context, serviceName string) (<-chan []*health.ServiceInfo, error) {
	prefix := c.servicePrefix(serviceName)
	watchChan, err := c.storage.Watch(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to watch services: %w", err)
	}
	
	resultChan := make(chan []*health.ServiceInfo, 1)
	
	go func() {
		defer close(resultChan)
		
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watchChan:
				if !ok {
					return
				}
				
				c.logger.Debug("Service watch event",
					zap.Int("type", int(event.Type)),
					zap.String("key", event.Key),
				)
				// Get current services and send update
				services, err := c.Discover(ctx, serviceName)
				if err != nil {
					c.logger.Error("Failed to deserialize service",
						zap.Error(err),
					)
					continue
				}
				
				select {
				case resultChan <- services:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	
	return resultChan, nil
}

func (c *Coordinator) SelectService(ctx context.Context, serviceName string) (*health.ServiceInfo, error) {
    healthyServices, err := c.GetHealthy(ctx, serviceName)
    if err != nil {
        return nil, err
    }
    
    if len(healthyServices) == 0 {
        return nil, fmt.Errorf("no healthy services available for %s", serviceName)
    }
    
    return c.balancer.Select(ctx, healthyServices)
}

// Close implements ServiceRegistry interface
func (c *Coordinator) Close() error {
	c.logger.Info("Shutting down coordinator...")
	
	// Signal shutdown to all background goroutines
	c.shutdownCancel()
	
	// Wait for all background goroutines to finish
	c.wg.Wait()
	
	// Close storage
	if err := c.storage.Close(); err != nil {
		c.logger.Error("Failed to close storage", zap.Error(err))
		return err
	}
	
	c.logger.Info("Coordinator shutdown complete")
	return nil
}

// Helper methods
func (c *Coordinator) serviceKey(serviceName, serviceID string) string {
	return fmt.Sprintf("/services/%s/%s", serviceName, serviceID)
}

func (c *Coordinator) servicePrefix(serviceName string) string {
	return fmt.Sprintf("/services/%s/", serviceName)
}

func (c *Coordinator) serializeService(service *health.ServiceInfo) (string, error) {
	// Implementation would serialize to JSON or protobuf
	data, err := json.Marshal(service)
	if err != nil {
		return  "", fmt.Errorf("failed to marshal service: %w", err)
	}
	return string(data), nil
}

func (c *Coordinator) deserializeService(data string) (*health.ServiceInfo, error) {
	// Implementation would deserialize from JSON or protobuf
	var service health.ServiceInfo
	if err := json.Unmarshal([]byte(data), &service); err != nil {
		return nil, fmt.Errorf("failed to unmarshal service: %w", err)
	}
	return &service, nil
}
