package health

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type Checker interface {
	// Check performs a health check on a service
	Check(ctx context.Context, service *ServiceInfo) (bool, error)

	// StartMonitoring begins continuous health monitoring
	StartMonitoring(ctx context.Context, interval time.Duration) error

	// StopMonitoring stops health monitoring
	StopMonitoring() error
}

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


// CheckStrategy defines different health check strategies
type CheckStrategy string

const (
	HttpStrategy CheckStrategy = "http"
	TCPStrategy CheckStrategy = "tcp"
	GRPCStrategy CheckStrategy = "grpc"
)

type HealthChecker struct {
	logger   *zap.Logger
	timeout  time.Duration
	strategy CheckStrategy
	
	// Monitoring
	monitoringCtx    context.Context
	monitoringCancel context.CancelFunc
	monitoringWg     sync.WaitGroup
	monitoringMu     sync.RWMutex
	isMonitoring     bool
	
	// HTTP client for HTTP health checks
	httpClient *http.Client
	
	// Custom health check endpoints
	healthEndpoints map[string]string // serviceID -> custom endpoint
	mu              sync.RWMutex
}

type HealthCheckerOptions struct {
	Logger          *zap.Logger
	Timeout         time.Duration
	Strategy        CheckStrategy
	HTTPClient      *http.Client
	HealthEndpoints map[string]string
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(opts HealthCheckerOptions) *HealthChecker {
	if opts.Logger == nil {
		opts.Logger = zap.NewNop()
	}
	
	if opts.Timeout == 0 {
		opts.Timeout = 5 * time.Second
	}
	
	if opts.Strategy == "" {
		opts.Strategy = HttpStrategy
	}
	
	if opts.HTTPClient == nil {
		opts.HTTPClient = &http.Client{
			Timeout: opts.Timeout,
		}
	}
	
	if opts.HealthEndpoints == nil {
		opts.HealthEndpoints = make(map[string]string)
	}
	
	return &HealthChecker{
		logger:          opts.Logger,
		timeout:         opts.Timeout,
		strategy:        opts.Strategy,
		httpClient:      opts.HTTPClient,
		healthEndpoints: opts.HealthEndpoints,
	}
}

// Check performs a health check on a service
func (hc *HealthChecker) Check(ctx context.Context, service *ServiceInfo) (bool, error) {
	if service == nil {
		return false, fmt.Errorf("service info cannot be nil")
	}
	
	// Create timeout context
	checkCtx, cancel := context.WithTimeout(ctx, hc.timeout)
	defer cancel()
	
	hc.logger.Debug("Performing health check",
		zap.String("service_id", service.ID),
		zap.String("service_name", service.Name),
		zap.String("strategy", string(hc.strategy)),
		zap.String("address", fmt.Sprintf("%s:%d", service.Address, service.Port)),
	)
	
	var isHealthy bool
	var err error
	
	switch hc.strategy {
	case HttpStrategy:
		isHealthy, err = hc.checkHTTP(checkCtx, service)
	case TCPStrategy:
		isHealthy, err = hc.checkTCP(checkCtx, service)
	case GRPCStrategy:
		isHealthy, err = hc.checkGRPC(checkCtx, service)
	default:
		return false, fmt.Errorf("unsupported health check strategy: %s", hc.strategy)
	}
	
	if err != nil {
		hc.logger.Debug("Health check failed",
			zap.String("service_id", service.ID),
			zap.Error(err),
		)
		return false, err
	}
	
	hc.logger.Debug("Health check completed",
		zap.String("service_id", service.ID),
		zap.Bool("healthy", isHealthy),
	)
	
	return isHealthy, nil
}

// checkHTTP performs HTTP health check
func (hc *HealthChecker) checkHTTP(ctx context.Context, service *ServiceInfo) (bool, error) {
	// Get custom endpoint or use default
	hc.mu.RLock()
	endpoint, hasCustom := hc.healthEndpoints[service.ID]
	hc.mu.RUnlock()
	
	var url string
	if hasCustom {
		url = endpoint
	} else {
		// Default health endpoint
		protocol := "http"
		if service.Metadata != nil {
			if proto, ok := service.Metadata["protocol"]; ok {
				protocol = proto
			}
		}
		url = fmt.Sprintf("%s://%s:%d/health", protocol, service.Address, service.Port)
	}
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	
	// Add headers if specified in metadata
	if service.Metadata != nil {
		if _, ok := service.Metadata["health_headers"]; ok {
			// Parse headers from metadata (format: "key1:value1,key2:value2")
			// This is a simplified implementation
			req.Header.Set("User-Agent", "health-checker/1.0")
		}
	}
	
	resp, err := hc.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("HTTP health check failed: %w", err)
	}
	defer resp.Body.Close()
	
	// Consider 2xx status codes as healthy
	return resp.StatusCode >= 200 && resp.StatusCode < 300, nil
}

// checkTCP performs TCP connection health check
func (hc *HealthChecker) checkTCP(ctx context.Context, service *ServiceInfo) (bool, error) {
	address := fmt.Sprintf("%s:%d", service.Address, service.Port)
	
	dialer := &net.Dialer{
		Timeout: hc.timeout,
	}
	
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return false, fmt.Errorf("TCP health check failed: %w", err)
	}
	defer conn.Close()
	
	return true, nil
}

// checkGRPC performs gRPC health check using the standard gRPC health protocol
func (hc *HealthChecker) checkGRPC(ctx context.Context, service *ServiceInfo) (bool, error) {
	address := fmt.Sprintf("%s:%d", service.Address, service.Port)
	
	// Create gRPC connection with timeout
	_, cancel := context.WithTimeout(ctx, hc.timeout)
	defer cancel()
	
	// unc NewClient(target string, opts ...DialOption) (conn *ClientConn, err error)
	conn, err := grpc.NewClient(address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		// grpc.WithBlock(),
	)
	if err != nil {
		return false, fmt.Errorf("failed to connect to gRPC service: %w", err)
	}
	defer conn.Close()
	
	// Create health client
	healthClient := grpc_health_v1.NewHealthClient(conn)
	
	// Perform health check
	serviceName := ""
	if service.Metadata != nil {
		if name, ok := service.Metadata["grpc_service_name"]; ok {
			serviceName = name
		}
	}
	
	req := &grpc_health_v1.HealthCheckRequest{
		Service: serviceName,
	}
	
	resp, err := healthClient.Check(ctx, req)
	if err != nil {
		return false, fmt.Errorf("gRPC health check failed: %w", err)
	}
	
	return resp.Status == grpc_health_v1.HealthCheckResponse_SERVING, nil
}

// StartMonitoring begins continuous health monitoring
func (hc *HealthChecker) StartMonitoring(ctx context.Context, interval time.Duration) error {
	hc.monitoringMu.Lock()
	defer hc.monitoringMu.Unlock()
	
	if hc.isMonitoring {
		return fmt.Errorf("monitoring is already running")
	}
	
	if interval <= 0 {
		interval = 30 * time.Second
	}
	
	hc.monitoringCtx, hc.monitoringCancel = context.WithCancel(ctx)
	hc.isMonitoring = true
	
	hc.logger.Info("Starting health monitoring",
		zap.Duration("interval", interval),
		zap.String("strategy", string(hc.strategy)),
	)
	
	hc.monitoringWg.Add(1)
	go hc.monitoringLoop(interval)
	
	return nil
}

// StopMonitoring stops health monitoring
func (hc *HealthChecker) StopMonitoring() error {
	hc.monitoringMu.Lock()
	defer hc.monitoringMu.Unlock()
	
	if !hc.isMonitoring {
		return fmt.Errorf("monitoring is not running")
	}
	
	hc.logger.Info("Stopping health monitoring")
	
	hc.monitoringCancel()
	hc.monitoringWg.Wait()
	hc.isMonitoring = false
	
	hc.logger.Info("Health monitoring stopped")
	return nil
}

// monitoringLoop runs the continuous monitoring
func (hc *HealthChecker) monitoringLoop(interval time.Duration) {
	defer hc.monitoringWg.Done()
	
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-hc.monitoringCtx.Done():
			hc.logger.Debug("Monitoring loop stopped due to context cancellation")
			return
		case <-ticker.C:
			// FIXME: This is a simplified monitoring loop
			// In a real implementation, you'd need access to the service 
			// to get the list of services to monitor
			hc.logger.Debug("Health monitoring tick")
			// TODO: Implement actual monitoring logic when integrated with service 
		}
	}
}

// SetCustomHealthEndpoint sets a custom health check endpoint for a service
func (hc *HealthChecker) SetCustomHealthEndpoint(serviceID, endpoint string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.healthEndpoints[serviceID] = endpoint
}

// RemoveCustomHealthEndpoint removes a custom health check endpoint
func (hc *HealthChecker) RemoveCustomHealthEndpoint(serviceID string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	delete(hc.healthEndpoints, serviceID)
}

// GetStrategy returns the current health check strategy
func (hc *HealthChecker) GetStrategy() CheckStrategy {
	return hc.strategy
}

// SetStrategy updates the health check strategy
func (hc *HealthChecker) SetStrategy(strategy CheckStrategy) {
	hc.strategy = strategy
	hc.logger.Info("Health check strategy updated",
		zap.String("strategy", string(strategy)),
	)
}

// IsMonitoring returns whether monitoring is currently active
func (hc *HealthChecker) IsMonitoring() bool {
	hc.monitoringMu.RLock()
	defer hc.monitoringMu.RUnlock()
	return hc.isMonitoring
}

// MultiStrategyChecker performs health checks using multiple strategies
type MultiStrategyChecker struct {
	checkers map[CheckStrategy]*HealthChecker
	logger   *zap.Logger
}

// NewMultiStrategyChecker creates a checker that can use different strategies per service
func NewMultiStrategyChecker(logger *zap.Logger) *MultiStrategyChecker {
	if logger == nil {
		logger = zap.NewNop()
	}
	
	return &MultiStrategyChecker{
		checkers: make(map[CheckStrategy]*HealthChecker),
		logger:   logger,
	}
}

// AddStrategy adds a health checker for a specific strategy
func (msc *MultiStrategyChecker) AddStrategy(strategy CheckStrategy, checker *HealthChecker) {
	msc.checkers[strategy] = checker
}

// Check performs health check using the strategy specified in service metadata
func (msc *MultiStrategyChecker) Check(ctx context.Context, service *ServiceInfo) (bool, error) {
	strategy := HttpStrategy // default
	
	if service.Metadata != nil {
		if strategyStr, ok := service.Metadata["health_strategy"]; ok {
			strategy = CheckStrategy(strategyStr)
		}
	}
	
	checker, exists := msc.checkers[strategy]
	if !exists {
		return false, fmt.Errorf("no checker available for strategy: %s", strategy)
	}
	
	return checker.Check(ctx, service)
}

// StartMonitoring starts monitoring for all strategies
func (msc *MultiStrategyChecker) StartMonitoring(ctx context.Context, interval time.Duration) error {
	for strategy, checker := range msc.checkers {
		if err := checker.StartMonitoring(ctx, interval); err != nil {
			msc.logger.Error("Failed to start monitoring for strategy",
				zap.String("strategy", string(strategy)),
				zap.Error(err),
			)
			return err
		}
	}
	return nil
}

// StopMonitoring stops monitoring for all strategies
func (msc *MultiStrategyChecker) StopMonitoring() error {
	var lastErr error
	for strategy, checker := range msc.checkers {
		if err := checker.StopMonitoring(); err != nil {
			msc.logger.Error("Failed to stop monitoring for strategy",
				zap.String("strategy", string(strategy)),
				zap.Error(err),
			)
			lastErr = err
		}
	}
	return lastErr
}
