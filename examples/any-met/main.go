// Package main demonstrates using WaitForAny and IsDependencyMet to check
// whether any of several tracked services are running.
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/dioad/depstate"
	"github.com/dioad/pubsub"
)

const (
	statusRunning = "Running"
	statusStopped = "Stopped"

	service1ID = "service1"
	service2ID = "service2"
	service3ID = "service3"
)

// Service represents a dependency with an ID and a running status.
type Service struct {
	ID     string
	Status string
}

// serviceIDStateFunc extracts the ID and state from a dependency.
func serviceIDStateFunc(s Service) (string, depstate.State) {
	state := depstate.DependenciesNotMet
	if s.Status == statusRunning {
		state = depstate.DependenciesMet
	}
	return s.ID, state
}

func main() {
	// Create a topic to publish service events
	topic := pubsub.NewTopic()

	// Create a context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a dependency state tracker using the convenience function
	ds, _ := depstate.NewDependencyStateWithTopic(ctx, serviceIDStateFunc, depstate.DependenciesMet, topic)

	// Add services to track
	service1 := Service{ID: service1ID, Status: statusStopped}
	service2 := Service{ID: service2ID, Status: statusStopped}
	service3 := Service{ID: service3ID, Status: statusStopped}
	ds.Add(service1, service2, service3)

	// Check if any service is running
	fmt.Println("Checking if any service is running:")
	fmt.Printf("  service1: %v\n", ds.IsDependencyMet(service1ID))
	fmt.Printf("  service2: %v\n", ds.IsDependencyMet(service2ID))
	fmt.Printf("  service3: %v\n", ds.IsDependencyMet(service3ID))

	// Start a goroutine to update service states
	go func() {
		time.Sleep(1 * time.Second)
		fmt.Println("Starting service2...")
		service2.Status = statusRunning
		topic.Publish(service2)

		time.Sleep(1 * time.Second)
		fmt.Println("Starting service1...")
		service1.Status = statusRunning
		topic.Publish(service1)

		time.Sleep(1 * time.Second)
		fmt.Println("Starting service3...")
		service3.Status = statusRunning
		topic.Publish(service3)
	}()

	// Wait for any service to be running
	fmt.Println("Waiting for any service to be running...")
	id, err := ds.WaitForAny(ctx, []string{service1ID, service2ID, service3ID}, 5*time.Second)
	if err == nil {
		fmt.Printf("Service %s is running!\n", id)
	} else {
		fmt.Println("Timed out waiting for any service to be running.")
	}

	// Check if any service is running again
	fmt.Println("Checking if any service is running:")
	fmt.Printf("  service1: %v\n", ds.IsDependencyMet(service1ID))
	fmt.Printf("  service2: %v\n", ds.IsDependencyMet(service2ID))
	fmt.Printf("  service3: %v\n", ds.IsDependencyMet(service3ID))

	// Wait for a specific service to be running
	fmt.Println("Waiting for service1 to be running...")
	id, err = ds.WaitForAny(ctx, []string{service1ID}, 5*time.Second)
	if err == nil {
		fmt.Printf("Service %s is running!\n", id)
	} else {
		fmt.Println("Timed out waiting for service1 to be running.")
	}
}
