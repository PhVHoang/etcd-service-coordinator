package config

import "time"

// Config holds application configuration
type Config struct {
	// Storage configuration
	Storage StorageConfig `yaml:"storage"`
	
	// Health check configuration
	Health HealthConfig `yaml:"health"`
	
	// Load balancer configuration
	LoadBalancer LoadBalancerConfig `yaml:"load_balancer"`
	
	// Server configuration
	Server ServerConfig `yaml:"server"`
	
	// Logging configuration
	Logging LoggingConfig `yaml:"logging"`
}

// StorageConfig configures the storage backend
type StorageConfig struct {
	Type      string        `yaml:"type"`
	Endpoints []string      `yaml:"endpoints"`
	Timeout   time.Duration `yaml:"timeout"`
	Username  string        `yaml:"username"`
	Password  string        `yaml:"password"`
}

// HealthConfig configures health checking
type HealthConfig struct {
	Interval time.Duration `yaml:"interval"`
	Timeout  time.Duration `yaml:"timeout"`
	Strategy string        `yaml:"strategy"`
}

// LoadBalancerConfig configures load balancing
type LoadBalancerConfig struct {
	Strategy string `yaml:"strategy"`
}

// ServerConfig configures the server
type ServerConfig struct {
	HTTPPort int    `yaml:"http_port"`
	GRPCPort int    `yaml:"grpc_port"`
	Address  string `yaml:"address"`
}

// LoggingConfig configures logging
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}