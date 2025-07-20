package registry

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/PhVHoang/cache-coordinator/pkg/health"
)

// ServiceRegistry defines the main registry interface
type ServiceRegistry interface {
	// Register registers a service instance
	Register(ctx context.Context, service *health.ServiceInfo) error
	
	// Deregister removes a service instance
	Deregister(ctx context.Context, serviceID string) error
	
	// Discover finds all instances of a service
	Discover(ctx context.Context, serviceName string) ([]*health.ServiceInfo, error)
	
	// GetHealthy returns only healthy service instances
	GetHealthy(ctx context.Context, serviceName string) ([]*health.ServiceInfo, error)
	
	// Watch watches for service changes
	Watch(ctx context.Context, serviceName string) (<-chan []*health.ServiceInfo, error)
	
	// Close gracefully shuts down the registry
	Close() error
}

// InMemoryServiceRegistry implements ServiceRegistry using in-memory storage
type InMemoryRegistry struct {
	mu sync.RWMutex
	services map[string]*health.ServiceInfo
	watchers map[string][]chan []*health.ServiceInfo // ServiceName -> Channels
	watcherMu sync.RWMutex

	// Background goroutine management
	stopCh chan struct {}
	wg sync.WaitGroup

	// Configuration
	cleanupInterval time.Duration
	// healthCheckFunc health.HealthChecker
}


type RegistryOption func(*InMemoryRegistry)

// WithCleanupInterval sets how often expired services are cleaned up
func WithCleanupInterval(interval time.Duration) RegistryOption {
	return func(r *InMemoryRegistry) {
		r.cleanupInterval = interval
	}
}

func NewInMemoryRegistry(opts ...RegistryOption) *InMemoryRegistry {
	registry := &InMemoryRegistry{
		services:        make(map[string]*health.ServiceInfo),
		watchers:        make(map[string][]chan []*health.ServiceInfo),
		stopCh:          make(chan struct{}),
		cleanupInterval: 30 * time.Second,
	}
	
	for _, opt := range opts {
		opt(registry)
	}

	// Start background cleanup goroutines
	registry.wg.Add(1)
	go registry.cleanupExpiredServices()

	return registry

}

// removeWatcher removes a watcher channel from the watchers map
func (r *InMemoryRegistry) removeWatcher(serviceName string, ch chan []*health.ServiceInfo) {
	r.watcherMu.Lock()
	defer r.watcherMu.Unlock()
	
	watchers, exists := r.watchers[serviceName]
	if !exists {
		return
	}
	
	// Find and remove the channel
	for i, watcher := range watchers {
		if watcher == ch {
			// Remove the watcher from the slice
			r.watchers[serviceName] = append(watchers[:i], watchers[i+1:]...)
			break
		}
	}
	
	// Clean up empty watcher lists
	if len(r.watchers[serviceName]) == 0 {
		delete(r.watchers, serviceName)
	}
}

// cleanupExpiredServices runs in the background to remove expired services
func (r *InMemoryRegistry) cleanupExpiredServices() {
	defer r.wg.Done()
	
	ticker := time.NewTicker(r.cleanupInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.performCleanup()
		}
	}
}

// performCleanup removes expired services based on TTL
func (r *InMemoryRegistry) performCleanup() {
	now := time.Now()
	var expiredServices []string
	var affectedServiceNames []string

	r.mu.Lock()
	for serviceID, service := range r.services {
		lastHeartbeatStr, exists := service.Metadata["last_heartbeat"]
		if !exists {
			lastHeartbeatStr = service.Metadata["registered_at"]
		}

		if lastHeartbeatStr != "" {
			lastHeartbeat, err := time.Parse(time.RFC3339, lastHeartbeatStr)
			if err == nil {
				// Check if service has expired
				if now.Sub(lastHeartbeat) > service.TTL {
					expiredServices = append(expiredServices, serviceID)
					affectedServiceNames = append(affectedServiceNames, service.Name)
					// Mark as unhealthy before removal
					service.Health = health.HealthStatusUnhealthy
				}
			}
		}
	}

	// Remove expired services
	for _, serviceID := range expiredServices {
		delete(r.services, serviceID)
	}
	r.mu.Unlock()
	
	// Notify watchers for affected services
	uniqueServiceNames := make(map[string]bool)
	for _, serviceName := range affectedServiceNames {
		if !uniqueServiceNames[serviceName] {
			uniqueServiceNames[serviceName] = true
			r.notifyWatchers(serviceName)
		}
	}
}

func (r *InMemoryRegistry) Discover(ctx context.Context, serviceName string) ([]*health.ServiceInfo, error) {
	if serviceName == "" {
		return nil, fmt.Errorf("service name cannot be empty")
	}
	
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	var services []*health.ServiceInfo
	for _, service := range r.services {
		if service.Name == serviceName {
			// Create a copy to avoid concurrent modification issues
			serviceCopy := *service
			if service.Metadata != nil {
				serviceCopy.Metadata = make(map[string]string)
				for k, v := range service.Metadata {
					serviceCopy.Metadata[k] = v
				}
			}
			services = append(services, &serviceCopy)
		}
	}
	
	return services, nil
}

// Register registers a service instance
func (r *InMemoryRegistry) Register(ctx context.Context, service *health.ServiceInfo) error {
	if service == nil {
		return fmt.Errorf("service cannot be nil")
	}
	
	if service.ID == "" {
		return fmt.Errorf("service ID cannot be empty")
	}
	
	if service.Name == "" {
		return fmt.Errorf("service name cannot be empty")
	}
	
	if service.Address == "" {
		return fmt.Errorf("service address cannot be empty")
	}
	
	if service.Port <= 0 || service.Port > 65535 {
		return fmt.Errorf("service port must be between 1 and 65535")
	}
	
	// Set default TTL if not provided
	if service.TTL == 0 {
		service.TTL = 60 * time.Second
	}
	
	// Initialize metadata if nil
	if service.Metadata == nil {
		service.Metadata = make(map[string]string)
	}
	
	// Add registration timestamp
	service.Metadata["registered_at"] = time.Now().Format(time.RFC3339)
	service.Metadata["last_heartbeat"] = time.Now().Format(time.RFC3339)
	
	r.mu.Lock()
	r.services[service.ID] = service
	r.mu.Unlock()
	
	// Notify watchers
	r.notifyWatchers(service.Name)
	
	return nil
}

// Deregister removes a service instance
func (r *InMemoryRegistry) Deregister(ctx context.Context, serviceID string) error {
	if serviceID == "" {
		return fmt.Errorf("service ID cannot be empty")
	}
	
	r.mu.Lock()
	service, exists := r.services[serviceID]
	if !exists {
		r.mu.Unlock()
		return fmt.Errorf("service with ID %s not found", serviceID)
	}
	
	serviceName := service.Name
	delete(r.services, serviceID)
	r.mu.Unlock()
	
	// Notify watchers
	r.notifyWatchers(serviceName)
	
	return nil
}

// notifyWatchers sends updates to all watchers of a service
func (r* InMemoryRegistry) notifyWatchers(serviceName string) {
	r.watcherMu.RLock()
	watchers, exists := r.watchers[serviceName]
	if !exists {
		r.watcherMu.RUnlock()
		return
	}

	// Create a copy of watchers to avoid holding the lock during notifications
	watchersCopy := make([]chan []*health.ServiceInfo, len(watchers))
	copy(watchersCopy, watchers)
	r.watcherMu.RUnlock()
	
	// Get current services
	services, err := r.Discover(context.Background(), serviceName)
	if err != nil {
		return
	}
	
	// Notify all watchers
	for _, ch := range watchersCopy {
		select {
		case ch <- services:
		default:
			// Channel is full or closed, skip
		}
	}
}

// Watch watches for service changes
func (r *InMemoryRegistry) Watch(ctx context.Context, serviceName string) (<-chan []*health.ServiceInfo, error) {
	if serviceName == "" {
		return nil, fmt.Errorf("service name cannot be empty")
	}
	
	ch := make(chan []*health.ServiceInfo, 1)
	
	r.watcherMu.Lock()
	r.watchers[serviceName] = append(r.watchers[serviceName], ch)
	r.watcherMu.Unlock()
	
	// Send initial state
	go func() {
		services, err := r.Discover(ctx, serviceName)
		if err == nil {
			select {
			case ch <- services:
			case <-ctx.Done():
				return
			}
		}
	}()
	
	// Clean up watcher when context is done
	go func() {
		<-ctx.Done()
		r.removeWatcher(serviceName, ch)
		close(ch)
	}()
	
	return ch, nil
}

// Close gracefully shuts down the registry
func (r *InMemoryRegistry) Close() error {
	close(r.stopCh)
	r.wg.Wait()
	
	// Close all watcher channels
	r.watcherMu.Lock()
	for _, watchers := range r.watchers {
		for _, ch := range watchers {
			close(ch)
		}
	}
	r.watchers = make(map[string][]chan []*health.ServiceInfo)
	r.watcherMu.Unlock()
	
	return nil
}

// Heartbeat updates the last heartbeat time for a service
func (r *InMemoryRegistry) Heartbeat(ctx context.Context, serviceID string) error {
	if serviceID == "" {
		return fmt.Errorf("service ID cannot be empty")
	}
	
	r.mu.Lock()
	defer r.mu.Unlock()
	
	service, exists := r.services[serviceID]
	if !exists {
		return fmt.Errorf("service with ID %s not found", serviceID)
	}
	
	if service.Metadata == nil {
		service.Metadata = make(map[string]string)
	}
	
	service.Metadata["last_heartbeat"] = time.Now().Format(time.RFC3339)
	service.Health = health.HealthStatusHealthy
	
	return nil
}

// GetService returns a specific service by ID
func (r *InMemoryRegistry) GetService(ctx context.Context, serviceID string) (*health.ServiceInfo, error) {
	if serviceID == "" {
		return nil, fmt.Errorf("service ID cannot be empty")
	}
	
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	service, exists := r.services[serviceID]
	if !exists {
		return nil, fmt.Errorf("service with ID %s not found", serviceID)
	}
	
	// Create a copy
	serviceCopy := *service
	if service.Metadata != nil {
		serviceCopy.Metadata = make(map[string]string)
		for k, v := range service.Metadata {
			serviceCopy.Metadata[k] = v
		}
	}
	
	return &serviceCopy, nil
}

// ListAllServices returns all registered services
func (r *InMemoryRegistry) ListAllServices(ctx context.Context) ([]*health.ServiceInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	var services []*health.ServiceInfo
	for _, service := range r.services {
		// Create a copy
		serviceCopy := *service
		if service.Metadata != nil {
			serviceCopy.Metadata = make(map[string]string)
			for k, v := range service.Metadata {
				serviceCopy.Metadata[k] = v
			}
		}
		services = append(services, &serviceCopy)
	}
	
	return services, nil
}
