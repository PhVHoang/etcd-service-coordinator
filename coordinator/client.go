package coordinator

import (
	"log"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func NewEtcdClient(endpoint string) *clientv3.Client {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{endpoint},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatalf("Failed to connect to etcd: %v", err)
	}
	return cli
}
