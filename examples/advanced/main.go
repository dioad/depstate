// Package main demonstrates advanced usage of the depstate package, simulating
// several services transitioning between running and stopped states over time.
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
	ds, depChan := depstate.NewDependencyState(ctx, serviceIDStateFunc, depstate.DependenciesMet, topic.Subscribe())

	// Add services to track
	service1 := Service{ID: "service1", Status: statusStopped}
	service2 := Service{ID: "service2", Status: statusStopped}
	service3 := Service{ID: "service3", Status: statusStopped}
	ds.Add(service1, service2, service3)

	// Start a goroutine to wait for all services to be ready
	go func() {
		for state := range depChan {
			if state == depstate.DependenciesMet {
				fmt.Println("All services are running!")
			} else {
				fmt.Println("Not all services are running yet.")
			}
		}
	}()

	// Simulate services changing state
	go func() {
		time.Sleep(1 * time.Second)
		service1.Status = statusRunning
		topic.Publish(service1)

		time.Sleep(1 * time.Second)
		service2.Status = statusRunning
		topic.Publish(service2)

		time.Sleep(1 * time.Second)
		service3.Status = statusRunning
		topic.Publish(service3)

		time.Sleep(1 * time.Second)
		service2.Status = statusStopped
		topic.Publish(service2)

		time.Sleep(1 * time.Second)
		service2.Status = statusRunning
		topic.Publish(service2)
	}()

	// Wait for a bit to see the output
	time.Sleep(6 * time.Second)
}
