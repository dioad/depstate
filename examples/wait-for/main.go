// Package main demonstrates using WaitForDependencies and GetDependencyStates
// to wait for a set of tracked services to be running.
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

	// Create a dependency state tracker
	ds, _ := depstate.NewDependencyState(ctx, serviceIDStateFunc, depstate.DependenciesMet, topic.Subscribe())

	// Add services to track
	service1 := Service{ID: "service1", Status: statusStopped}
	service2 := Service{ID: "service2", Status: statusStopped}
	service3 := Service{ID: "service3", Status: statusStopped}
	ds.Add(service1, service2, service3)

	// Get the current state of all dependencies
	states := ds.GetDependencyStates()
	fmt.Println("Initial dependency states:")
	for id, state := range states {
		fmt.Printf("  %s: %s\n", id, state)
	}

	// Start a goroutine to update service states
	go func() {
		time.Sleep(1 * time.Second)
		fmt.Println("Starting service1...")
		service1.Status = statusRunning
		topic.Publish(service1)

		time.Sleep(1 * time.Second)
		fmt.Println("Starting service2...")
		service2.Status = statusRunning
		topic.Publish(service2)

		time.Sleep(1 * time.Second)
		fmt.Println("Starting service3...")
		service3.Status = statusRunning
		topic.Publish(service3)
	}()

	// Wait for all services to be running with a timeout
	fmt.Println("Waiting for all services to be running...")
	if ds.WaitForDependencies(ctx, 5*time.Second) {
		fmt.Println("All services are running!")
	} else {
		fmt.Println("Timed out waiting for services to be running.")
	}

	// Get the current state of all dependencies again
	states = ds.GetDependencyStates()
	fmt.Println("Final dependency states:")
	for id, state := range states {
		fmt.Printf("  %s: %s\n", id, state)
	}
}
