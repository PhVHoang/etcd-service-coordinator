package storage_test

import (
	context "context"
	"testing"
	"time"

	"github.com/PhVHoang/cache-coordinator/pkg/storage"
)

// Mock etcd client for unit tests
// For real integration, use embedded etcd or a running etcd instance

type mockEtcdClient struct {
	putCalled    bool
	putKey       string
	putValue     string
	putLease     int64
	getCalled    bool
	getKey       string
	getRespValue string
	getErr       error
	deleteCalled bool
	deleteKey    string
	listCalled   bool
	listPrefix   string
	listResp     map[string]string
	listErr      error
	closeCalled  bool
	closeErr     error
}

// Implement minimal methods needed for EtcdStorage
// Use interface{} for clientv3.Client to allow injection

func TestEtcdStorage_PutGetDelete(t *testing.T) {
	// This test is a stub for real etcd integration
	// For true unit tests, use an embedded etcd or mock client
	endpoints := []string{"localhost:2379"}
	store, err := storage.NewEtcdStorage(endpoints, time.Second)
	if err != nil {
		t.Skip("etcd not available for integration test")
	}
	ctx := context.Background()
	key := "test-key"
	value := "test-value"
	// Put
	err = store.Put(ctx, key, value, 0)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}
	// Get
	got, err := store.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got != value {
		t.Errorf("Get returned %q, want %q", got, value)
	}
	// Delete
	err = store.Delete(ctx, key)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	got, err = store.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get after delete failed: %v", err)
	}
	if got != "" {
		t.Errorf("Get after delete returned %q, want empty string", got)
	}
	_ = store.Close()
}

func TestEtcdStorage_List(t *testing.T) {
	endpoints := []string{"localhost:2379"}
	store, err := storage.NewEtcdStorage(endpoints, time.Second)
	if err != nil {
		t.Skip("etcd not available for integration test")
	}
	ctx := context.Background()
	prefix := "test-list/"
	// Clean up any previous keys
	_ = store.Delete(ctx, prefix+"a")
	_ = store.Delete(ctx, prefix+"b")
	_ = store.Put(ctx, prefix+"a", "va", 0)
	_ = store.Put(ctx, prefix+"b", "vb", 0)
	result, err := store.List(ctx, prefix)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if result[prefix+"a"] != "va" || result[prefix+"b"] != "vb" {
		t.Errorf("List returned wrong values: %v", result)
	}
	_ = store.Delete(ctx, prefix+"a")
	_ = store.Delete(ctx, prefix+"b")
	_ = store.Close()
}

func TestEtcdStorage_Watch(t *testing.T) {
	endpoints := []string{"localhost:2379"}
	store, err := storage.NewEtcdStorage(endpoints, time.Second)
	if err != nil {
		t.Skip("etcd not available for integration test")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	prefix := "test-watch/"
	ch, err := store.Watch(ctx, prefix)
	if err != nil {
		t.Fatalf("Watch failed: %v", err)
	}
	// Put a key to trigger watch
	_ = store.Put(ctx, prefix+"x", "vx", 0)
	select {
	case evt := <-ch:
		if evt.Key != prefix+"x" || evt.Value != "vx" || evt.Type != storage.EventTypePut {
			t.Errorf("Watch event mismatch: %+v", evt)
		}
	case <-time.After(2 * time.Second):
		t.Error("Watch event not received")
	}
	_ = store.Delete(ctx, prefix+"x")
	_ = store.Close()
}
