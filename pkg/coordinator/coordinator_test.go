package coordinator_test

import (
	context "context"
	"fmt"
	"testing"
	"time"

	"github.com/PhVHoang/cache-coordinator/pkg/coordinator"
	"github.com/PhVHoang/cache-coordinator/pkg/health"
	"github.com/PhVHoang/cache-coordinator/pkg/storage"
)

// Mock implementations
type mockStorage struct {
	putCalled   bool
	putErr      error
	listCalled  bool
	listData    []string
	listErr     error
	closeCalled bool
	closeErr    error
}

func (m *mockStorage) Put(ctx context.Context, key, value string, ttl time.Duration) error {
	m.putCalled = true
	return m.putErr
}
func (m *mockStorage) Get(ctx context.Context, key string) (string, error) {
	return "", nil
}
func (m *mockStorage) Delete(ctx context.Context, key string) error {
	return nil
}
func (m *mockStorage) List(ctx context.Context, prefix string) (map[string]string, error) {
	m.listCalled = true
	result := map[string]string{}
	for i, v := range m.listData {
		result[fmt.Sprintf("key%d", i)] = v
	}
	return result, m.listErr
}
func (m *mockStorage) Watch(ctx context.Context, prefix string) (<-chan storage.WatchEvent, error) {
	ch := make(chan storage.WatchEvent)
	close(ch)
	return ch, nil
}
func (m *mockStorage) Close() error {
	m.closeCalled = true
	return m.closeErr
}

// Health checker mock
type mockHealthChecker struct {
	checkHealthy bool
	checkErr     error
	startCalled  bool
}
func (m *mockHealthChecker) Check(ctx context.Context, service *health.ServiceInfo) (bool, error) {
	return m.checkHealthy, m.checkErr
}
func (m *mockHealthChecker) StartMonitoring(ctx context.Context, interval time.Duration) error {
	m.startCalled = true
	return nil
}
func (m *mockHealthChecker) StopMonitoring() error {
	return nil
}

func TestNewCoordinator_Defaults(t *testing.T) {
	storage := &mockStorage{}
	coord, err := coordinator.NewCoordinator(context.Background(), coordinator.Options{
		Storage: storage,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// We can't access coord.balancer directly since it's unexported, but we can check that no error was returned and coord is not nil
	if coord == nil {
		t.Fatal("expected coordinator instance")
	}
}

func TestRegister_Success(t *testing.T) {
	storage := &mockStorage{}
	coord, _ := coordinator.NewCoordinator(context.Background(), coordinator.Options{
		Storage: storage,
	})
	service := &health.ServiceInfo{
		ID:      "id1",
		Name:    "svc",
		Address: "127.0.0.1",
		Port:    8080,
		TTL:     time.Second,
	}
	err := coord.Register(context.Background(), service)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !storage.putCalled {
		t.Error("expected Put to be called")
	}
}

func TestRegister_SerializeError(t *testing.T) {
	t.Skip("Cannot simulate serialization error without changing ServiceInfo definition")
}

func TestDiscover_Success(t *testing.T) {
	storage := &mockStorage{listData: []string{`{"ID":"id1","Name":"svc","Address":"127.0.0.1","Port":8080}`}}
	coord, _ := coordinator.NewCoordinator(context.Background(), coordinator.Options{
		Storage: storage,
	})
	services, err := coord.Discover(context.Background(), "svc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(services) != 1 {
		t.Errorf("expected 1 service, got %d", len(services))
	}
}

func TestGetHealthy_WithHealthChecker(t *testing.T) {
	storage := &mockStorage{listData: []string{`{"ID":"id1","Name":"svc","Address":"127.0.0.1","Port":8080}`}}
	hc := &mockHealthChecker{checkHealthy: true}
	coord, _ := coordinator.NewCoordinator(context.Background(), coordinator.Options{
		Storage: storage,
		HealthChecker: hc,
	})
	services, err := coord.GetHealthy(context.Background(), "svc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(services) != 1 {
		t.Errorf("expected 1 healthy service, got %d", len(services))
	}
}

func TestSelectService_NoHealthy(t *testing.T) {
	storage := &mockStorage{listData: []string{}}
	coord, _ := coordinator.NewCoordinator(context.Background(), coordinator.Options{
		Storage: storage,
	})
	_, err := coord.SelectService(context.Background(), "svc")
	if err == nil {
		t.Error("expected error for no healthy services")
	}
}

func TestClose_CallsStorageClose(t *testing.T) {
	storage := &mockStorage{}
	coord, _ := coordinator.NewCoordinator(context.Background(), coordinator.Options{
		Storage: storage,
	})
	_ = coord.Close()
	if !storage.closeCalled {
		t.Error("expected storage.Close to be called")
	}
}
