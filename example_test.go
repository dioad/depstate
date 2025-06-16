package depstate_test

import (
	"context"
	"fmt"
	"time"

	"github.com/dioad/depstate"
	"github.com/dioad/pubsub"
)

// This example demonstrates the basic usage of the depstate package.
// It shows how to create a dependency state tracker, add dependencies,
// and wait for all dependencies to be in the desired state.
func Example_basic() {
	// Define a dependency type
	type Service struct {
		ID    string
		Ready bool
	}

	// Define a function to extract ID and state from a dependency
	serviceIDStateFunc := func(s Service) (string, depstate.State) {
		state := depstate.DependenciesNotMet
		if s.Ready {
			state = depstate.DependenciesMet
		}
		return s.ID, state
	}

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
	fmt.Println("Initial state:", ds.CurrentState())

	fmt.Println("Making service1 ready...")
	service1.Ready = true
	topic.Publish(service1)
	time.Sleep(10 * time.Millisecond) // Give time for the update to be processed

	fmt.Println("Making service2 ready...")
	service2.Ready = true
	topic.Publish(service2)
	time.Sleep(10 * time.Millisecond) // Give time for the update to be processed

	fmt.Println("Final state:", ds.CurrentState())

	// Output:
	// Initial state: dependencies_notmet
	// Making service1 ready...
	// Not all services are ready yet.
	// Making service2 ready...
	// All services are ready!
	// Final state: dependencies_met
}

// This example demonstrates how to use the WaitForDependencies method
// to wait for all dependencies to be in the desired state with a timeout.
func Example_waitForDependencies() {
	// Define a dependency type
	type Service struct {
		ID     string
		Status string
	}

	// Define a function to extract ID and state from a dependency
	serviceIDStateFunc := func(s Service) (string, depstate.State) {
		state := depstate.DependenciesNotMet
		if s.Status == "Running" {
			state = depstate.DependenciesMet
		}
		return s.ID, state
	}

	// Create a topic to publish service events
	topic := pubsub.NewTopic()

	// Create a context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a dependency state tracker
	ds, _ := depstate.NewDependencyState(ctx, serviceIDStateFunc, depstate.DependenciesMet, topic.Subscribe())

	// Add services to track
	service1 := Service{ID: "service1", Status: "Stopped"}
	service2 := Service{ID: "service2", Status: "Stopped"}
	ds.Add(service1, service2)

	// Start a goroutine to update service states
	go func() {
		time.Sleep(10 * time.Millisecond)
		fmt.Println("Starting service1...")
		service1.Status = "Running"
		topic.Publish(service1)

		time.Sleep(10 * time.Millisecond)
		fmt.Println("Starting service2...")
		service2.Status = "Running"
		topic.Publish(service2)
	}()

	// Wait for all services to be running with a timeout
	fmt.Println("Waiting for all services to be running...")
	if ds.WaitForDependencies(ctx, 100*time.Millisecond) {
		fmt.Println("All services are running!")
	} else {
		fmt.Println("Timed out waiting for services to be running.")
	}

	// Output:
	// Waiting for all services to be running...
	// Starting service1...
	// Starting service2...
	// All services are running!
}

// This example demonstrates how to use the IsDependencyMet and WaitForAny methods
// to check if specific dependencies are in the desired state and wait for any of
// the specified dependencies to be in the desired state.
func Example_isDependencyMetAndWaitForAny() {
	// Define a dependency type
	type Service struct {
		ID     string
		Status string
	}

	// Define a function to extract ID and state from a dependency
	serviceIDStateFunc := func(s Service) (string, depstate.State) {
		state := depstate.DependenciesNotMet
		if s.Status == "Running" {
			state = depstate.DependenciesMet
		}
		return s.ID, state
	}

	// Create a topic to publish service events
	topic := pubsub.NewTopic()

	// Create a context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a dependency state tracker using the convenience function
	ds, _ := depstate.NewDependencyStateWithTopic(ctx, serviceIDStateFunc, depstate.DependenciesMet, topic)

	// Add services to track
	service1 := Service{ID: "service1", Status: "Stopped"}
	service2 := Service{ID: "service2", Status: "Stopped"}
	service3 := Service{ID: "service3", Status: "Stopped"}
	ds.Add(service1, service2, service3)

	// Check if any service is running
	fmt.Println("Checking if any service is running:")
	fmt.Printf("  service1: %v\n", ds.IsDependencyMet("service1"))
	fmt.Printf("  service2: %v\n", ds.IsDependencyMet("service2"))
	fmt.Printf("  service3: %v\n", ds.IsDependencyMet("service3"))

	// Start a goroutine to update service states
	go func() {
		time.Sleep(10 * time.Millisecond)
		fmt.Println("Starting service2...")
		service2.Status = "Running"
		topic.Publish(service2)
	}()

	// Wait for any service to be running
	fmt.Println("Waiting for any service to be running...")
	id, err := ds.WaitForAny(ctx, []string{"service1", "service2", "service3"}, 100*time.Millisecond)
	if err == nil {
		fmt.Printf("Service %s is running!\n", id)
	} else {
		fmt.Println("Timed out waiting for any service to be running.")
	}

	// Check if any service is running again
	fmt.Println("Checking if any service is running:")
	fmt.Printf("  service1: %v\n", ds.IsDependencyMet("service1"))
	fmt.Printf("  service2: %v\n", ds.IsDependencyMet("service2"))
	fmt.Printf("  service3: %v\n", ds.IsDependencyMet("service3"))

	// Output:
	// Checking if any service is running:
	//   service1: false
	//   service2: false
	//   service3: false
	// Waiting for any service to be running...
	// Starting service2...
	// Service service2 is running!
	// Checking if any service is running:
	//   service1: false
	//   service2: true
	//   service3: false
}
