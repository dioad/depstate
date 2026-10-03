// Package main demonstrates basic usage of the depstate package: tracking
// when a set of services become ready.
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/dioad/depstate"
	"github.com/dioad/pubsub"
)

// Service represents a dependency with an ID and a ready flag.
type Service struct {
	ID    string
	Ready bool
}

// serviceIDStateFunc extracts the ID and state from a dependency.
func serviceIDStateFunc(s Service) (string, depstate.State) {
	state := depstate.DependenciesNotMet
	if s.Ready {
		state = depstate.DependenciesMet
	}
	return s.ID, state
}

func main() {
	// Create a topic to publish service events
	topic := pubsub.NewTopic()

	// Create a dependency state tracker
	ctx := context.Background()
	ds, depChan := depstate.NewDependencyState(ctx, serviceIDStateFunc, depstate.DependenciesMet, topic.Subscribe())

	// Add services to track
	service1 := Service{ID: "service1", Ready: false}
	service2 := Service{ID: "service2", Ready: false}
	ds.Add(service1, service2)

	// Start a goroutine to wait for all services to be ready
	go func() {
		for state := range depChan {
			if state == depstate.DependenciesMet {
				fmt.Println("All services are ready!")
			} else {
				fmt.Println("Not all services are ready yet.")
			}
		}
	}()

	// Simulate services becoming ready
	time.Sleep(1 * time.Second)
	service1.Ready = true
	topic.Publish(service1)

	time.Sleep(1 * time.Second)
	service2.Ready = true
	topic.Publish(service2)

	// Wait for a bit to see the output
	time.Sleep(1 * time.Second)
}
