package coordinator

import (
	"context"
	"log"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func RegisterService(client *clientv3.Client, serviceName, instanceId, address string, ttl int64) error {
	key := "/services/" + serviceName + "/" + instanceId
	lease, err := client.Grant(context.Background(), ttl)
	if err != nil {
		log.Fatalf("Grant failed: %v", err)
		return err
	}

	_, err = client.Put(context.Background(), key, address, clientv3.WithLease(lease.ID))
	if err != nil {
		log.Fatalf("Put failed : %v", err)
		return err
	}

	log.Printf("[Registered] %s -> %s", key, address)

	ch, err := client.KeepAlive(context.Background(), lease.ID)
	if err != nil {
		log.Fatalf("KeepAlive failed: %v", err)
		return err
	}

	// Keep the lease alive
	for ka := range ch {
		log.Printf("[Heartbeat] %s TTL: %d", key, ka.TTL)
	}

	return nil
}