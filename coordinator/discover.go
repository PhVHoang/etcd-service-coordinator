package coordinator

import (
	"context"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func DiscoverServices(client *clientv3.Client, serviceName string) []string {
	prefix := "/services/" + serviceName + "/"
	resp, err := client.Get(context.Background(), prefix, clientv3.WithPrefix())
	if err != nil {
		return []string{}
	}

	var addresses []string
	for _, kv := range resp.Kvs {
		addresses = append(addresses, string(kv.Value))
	}
	return addresses
}
