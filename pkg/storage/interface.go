package storage

import (
	"context"
	"time"
)

// Storage defines the storage backend interface
type Storage interface {
	// Put stores a key-value pair with optional TTL
	Put(ctx context.Context, key, value string, ttl time.Duration) error
	
	// Get retrieves a value by key
	Get(ctx context.Context, key string) (string, error)
	
	// Delete removes a key
	Delete(ctx context.Context, key string) error
	
	// List returns all keys with a given prefix
	List(ctx context.Context, prefix string) (map[string]string, error)
	
	// Watch watches for changes to keys with a given prefix
	Watch(ctx context.Context, prefix string) (<-chan WatchEvent, error)
	
	// Close closes the storage connection
	Close() error
}

// WatchEvent represents a storage change event
type WatchEvent struct {
	Type  EventType
	Key   string
	Value string
}

// EventType represents the type of storage event
type EventType int

const (
	EventTypePut EventType = iota
	EventTypeDelete
)
