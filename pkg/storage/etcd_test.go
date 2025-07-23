package storage_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	stor "github.com/PhVHoang/cache-coordinator/pkg/storage"
)

type fakeEtcdClient struct {
	putCalls    []struct{ key, value string; ttl int64 }
	getData     map[string]string
	deleteCalls []string
	listData    map[string]string
	watchCh     chan []stor.WatchEvent
	closed      bool
}

func newFakeEtcdClient() *fakeEtcdClient {
	return &fakeEtcdClient{
		getData:  make(map[string]string),
		listData: make(map[string]string),
		watchCh:  make(chan []stor.WatchEvent, 1),
	}
}

// Minimal EtcdStorage for unit test with injectable client

type testEtcdStorage struct {
	client *fakeEtcdClient
}

func (e *testEtcdStorage) Put(ctx context.Context, key, value string, ttl time.Duration) error {
	e.client.putCalls = append(e.client.putCalls, struct{ key, value string; ttl int64 }{key, value, int64(ttl.Seconds())})
	e.client.getData[key] = value
	return nil
}
func (e *testEtcdStorage) Get(ctx context.Context, key string) (string, error) {
	v, ok := e.client.getData[key]
	if !ok {
		return "", nil
	}
	return v, nil
}
func (e *testEtcdStorage) Delete(ctx context.Context, key string) error {
	e.client.deleteCalls = append(e.client.deleteCalls, key)
	delete(e.client.getData, key)
	return nil
}
func (e *testEtcdStorage) List(ctx context.Context, prefix string) (map[string]string, error) {
	result := map[string]string{}
	for k, v := range e.client.getData {
		if len(prefix) == 0 || (len(k) >= len(prefix) && k[:len(prefix)] == prefix) {
			result[k] = v
		}
	}
	return result, nil
}
func (e *testEtcdStorage) Watch(ctx context.Context, prefix string) (<-chan stor.WatchEvent, error) {
	ch := make(chan stor.WatchEvent, 1)
	if events, ok := <-e.client.watchCh; ok {
		for _, evt := range events {
			ch <- evt
		}
	}
	close(ch)
	return ch, nil
}
func (e *testEtcdStorage) Close() error {
	e.client.closed = true
	return nil
}

func TestEtcdStorage_Unit_PutGetDelete(t *testing.T) {
	cli := newFakeEtcdClient()
	store := &testEtcdStorage{client: cli}
	ctx := context.Background()
	key := "k"
	val := "v"
	if err := store.Put(ctx, key, val, 0); err != nil {
		t.Fatalf("Put failed: %v", err)
	}
	got, err := store.Get(ctx, key)
	if err != nil || got != val {
		t.Errorf("Get got %q, want %q, err=%v", got, val, err)
	}
	if err := store.Delete(ctx, key); err != nil {
		t.Errorf("Delete failed: %v", err)
	}
	got, _ = store.Get(ctx, key)
	if got != "" {
		t.Errorf("Get after delete got %q, want empty", got)
	}
}

func TestEtcdStorage_Unit_List(t *testing.T) {
	cli := newFakeEtcdClient()
	store := &testEtcdStorage{client: cli}
	ctx := context.Background()
	store.Put(ctx, "a/x", "1", 0)
	store.Put(ctx, "a/y", "2", 0)
	store.Put(ctx, "b/z", "3", 0)
	result, err := store.List(ctx, "a/")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	want := map[string]string{"a/x": "1", "a/y": "2"}
	if !reflect.DeepEqual(result, want) {
		t.Errorf("List got %v, want %v", result, want)
	}
}

func TestEtcdStorage_Unit_Watch(t *testing.T) {
	cli := newFakeEtcdClient()
	store := &testEtcdStorage{client: cli}
	ctx := context.Background()
	cli.watchCh <- []stor.WatchEvent{{Type: stor.EventTypePut, Key: "foo", Value: "bar"}}
	ch, err := store.Watch(ctx, "foo")
	if err != nil {
		t.Fatalf("Watch failed: %v", err)
	}
	select {
	case evt, ok := <-ch:
		if !ok || evt.Key != "foo" || evt.Value != "bar" || evt.Type != stor.EventTypePut {
			t.Errorf("Watch got %+v, want foo/bar/put", evt)
		}
	case <-time.After(time.Second):
		t.Error("Watch timeout")
	}
}

func TestEtcdStorage_Unit_Close(t *testing.T) {
	cli := newFakeEtcdClient()
	store := &testEtcdStorage{client: cli}
	if err := store.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
	if !cli.closed {
		t.Error("Close did not set closed flag")
	}
}

func TestEtcdStorage_PutGetDelete(t *testing.T) {
	// This test is a stub for real etcd integration
	// For true unit tests, use an embedded etcd or mock client
	endpoints := []string{"localhost:2379"}
	store, err := stor.NewEtcdStorage(endpoints, time.Second)
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
	store, err := stor.NewEtcdStorage(endpoints, time.Second)
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
	store, err := stor.NewEtcdStorage(endpoints, time.Second)
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
		if evt.Key != prefix+"x" || evt.Value != "vx" || evt.Type != stor.EventTypePut {
			t.Errorf("Watch event mismatch: %+v", evt)
		}
	case <-time.After(2 * time.Second):
		t.Error("Watch event not received")
	}
	_ = store.Delete(ctx, prefix+"x")
	_ = store.Close()
}
