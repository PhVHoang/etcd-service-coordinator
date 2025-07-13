package errors

import "errors"

var (
	// Registry errors
	ErrServiceNotFound       = errors.New("service not found")
	ErrNoServicesAvailable   = errors.New("no services available")
	ErrServiceAlreadyExists  = errors.New("service already exists")
	
	// Storage errors
	ErrStorageUnavailable    = errors.New("storage unavailable")
	ErrKeyNotFound          = errors.New("key not found")
	
	// Health check errors
	ErrHealthCheckFailed    = errors.New("health check failed")
	ErrHealthCheckTimeout   = errors.New("health check timeout")
	
	// Configuration errors
	ErrInvalidConfiguration = errors.New("invalid configuration")
)