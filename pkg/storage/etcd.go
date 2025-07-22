package storage

import (
	"context"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// EtcdStorage implements Storage interface using etcd

type EtcdStorage struct {
	client *clientv3.Client
}

func NewEtcdStorage(endpoints []string, dialTimeout time.Duration) (*EtcdStorage, error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: dialTimeout,
	})
	if err != nil {
		return nil, err
	}
	return &EtcdStorage{client: cli}, nil
}

func (e *EtcdStorage) Put(ctx context.Context, key, value string, ttl time.Duration) error {
	if ttl > 0 {
		lease, err := e.client.Grant(ctx, int64(ttl.Seconds()))
		if err != nil {
			return err
		}
		_, err = e.client.Put(ctx, key, value, clientv3.WithLease(lease.ID))
		return err
	}
	_, err := e.client.Put(ctx, key, value)
	return err
}

func (e *EtcdStorage) Get(ctx context.Context, key string) (string, error) {
	resp, err := e.client.Get(ctx, key)
	if err != nil {
		return "", err
	}
	if len(resp.Kvs) == 0 {
		return "", nil
	}
	return string(resp.Kvs[0].Value), nil
}

func (e *EtcdStorage) Delete(ctx context.Context, key string) error {
	_, err := e.client.Delete(ctx, key)
	return err
}

func (e *EtcdStorage) List(ctx context.Context, prefix string) (map[string]string, error) {
	resp, err := e.client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for _, kv := range resp.Kvs {
		result[string(kv.Key)] = string(kv.Value)
	}
	return result, nil
}

func (e *EtcdStorage) Watch(ctx context.Context, prefix string) (<-chan WatchEvent, error) {
	ch := make(chan WatchEvent)
	etcdCh := e.client.Watch(ctx, prefix, clientv3.WithPrefix())
	go func() {
		for wresp := range etcdCh {
			for _, ev := range wresp.Events {
				var typ EventType
				switch ev.Type {
				case clientv3.EventTypePut:
					typ = EventTypePut
				case clientv3.EventTypeDelete:
					typ = EventTypeDelete
				}
				ch <- WatchEvent{
					Type:  typ,
					Key:   string(ev.Kv.Key),
					Value: string(ev.Kv.Value),
				}
			}
		}
		close(ch)
	}()
	return ch, nil
}

func (e *EtcdStorage) Close() error {
	return e.client.Close()
}
