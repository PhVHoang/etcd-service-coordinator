package main

import (
	"log"
	"sync"
	"time"

	"github.com/PhVHoang/cache-coordinator/coordinator"
)

func main() {
	cli := coordinator.NewEtcdClient("localhost:2379")
	defer cli.Close()

	serviceName := "my-service"
	instanceID := "instance-1"

	var wg sync.WaitGroup
	wg.Add(2)
 
	// Register service
	go func() {
		if err := coordinator.RegisterService(cli, serviceName, instanceID, "127.0.0.1:8080", 5); err != nil {
			log.Fatalf("Service registration failed: %v", err)
		}
	}()

	// Discover services
	go func() {
		for {
			services := coordinator.DiscoverServices(cli, serviceName)
			log.Println("[Discovered services]:", services)
			time.Sleep(5 * time.Second)
		}
	}()
	
	wg.Wait()
	// Leader election
	// coordinator.RunElection(cli, serviceName, instanceID)
}
