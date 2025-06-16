# Dependency State

A Go package for tracking the state of dependencies and emitting events when a predicate is met.

## Overview

The `depstate` package provides a way to track the state of dependencies and emit events when all dependencies are in a
desired state. It's useful for scenarios where you need to wait for multiple conditions to be met before proceeding,
such as waiting for all services to be ready before starting an application.

## Features

- Track the state of multiple dependencies
- Emit events when all dependencies are in a desired state
- Thread-safe and concurrent access
- Support for context cancellation
- Type-safe using Go generics
- Wait for dependencies to be met with a timeout
- Get a snapshot of all dependencies and their states

## Installation

```bash
go get github.com/dioad/depstate
```

## Usage

### Basic Usage

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/dioad/depstate"
	"github.com/dioad/pubsub"
)

// Define a dependency type
type Service struct {
	ID    string
	Ready bool
}

// Define a function to extract ID and state from a dependency
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
```

### Advanced Usage

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/dioad/depstate"
	"github.com/dioad/pubsub"
)

// Define a dependency type
type Service struct {
	ID     string
	Status string
}

// Define a function to extract ID and state from a dependency
func serviceIDStateFunc(s Service) (string, depstate.State) {
	state := depstate.DependenciesNotMet
	if s.Status == "Running" {
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
	service1 := Service{ID: "service1", Status: "Stopped"}
	service2 := Service{ID: "service2", Status: "Stopped"}
	service3 := Service{ID: "service3", Status: "Stopped"}
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
		service1.Status = "Running"
		topic.Publish(service1)

		time.Sleep(1 * time.Second)
		service2.Status = "Running"
		topic.Publish(service2)

		time.Sleep(1 * time.Second)
		service3.Status = "Running"
		topic.Publish(service3)

		time.Sleep(1 * time.Second)
		service2.Status = "Stopped"
		topic.Publish(service2)

		time.Sleep(1 * time.Second)
		service2.Status = "Running"
		topic.Publish(service2)
	}()

	// Wait for a bit to see the output
	time.Sleep(6 * time.Second)
}
```

### Using WaitForDependencies and GetDependencyStates

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/dioad/depstate"
	"github.com/dioad/pubsub"
)

// Define a dependency type
type Service struct {
	ID     string
	Status string
}

// Define a function to extract ID and state from a dependency
func serviceIDStateFunc(s Service) (string, depstate.State) {
	state := depstate.DependenciesNotMet
	if s.Status == "Running" {
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
	service1 := Service{ID: "service1", Status: "Stopped"}
	service2 := Service{ID: "service2", Status: "Stopped"}
	service3 := Service{ID: "service3", Status: "Stopped"}
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
		service1.Status = "Running"
		topic.Publish(service1)

		time.Sleep(1 * time.Second)
		fmt.Println("Starting service2...")
		service2.Status = "Running"
		topic.Publish(service2)

		time.Sleep(1 * time.Second)
		fmt.Println("Starting service3...")
		service3.Status = "Running"
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
```

### Using IsDependencyMet and WaitForAny

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/dioad/depstate"
	"github.com/dioad/pubsub"
)

// Define a dependency type
type Service struct {
	ID     string
	Status string
}

// Define a function to extract ID and state from a dependency
func serviceIDStateFunc(s Service) (string, depstate.State) {
	state := depstate.DependenciesNotMet
	if s.Status == "Running" {
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
		time.Sleep(1 * time.Second)
		fmt.Println("Starting service2...")
		service2.Status = "Running"
		topic.Publish(service2)

		time.Sleep(1 * time.Second)
		fmt.Println("Starting service1...")
		service1.Status = "Running"
		topic.Publish(service1)

		time.Sleep(1 * time.Second)
		fmt.Println("Starting service3...")
		service3.Status = "Running"
		topic.Publish(service3)
	}()

	// Wait for any service to be running
	fmt.Println("Waiting for any service to be running...")
	id, err := ds.WaitForAny(ctx, []string{"service1", "service2", "service3"}, 5*time.Second)
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

	// Wait for a specific service to be running
	fmt.Println("Waiting for service1 to be running...")
	id, err = ds.WaitForAny(ctx, []string{"service1"}, 5*time.Second)
	if err == nil {
		fmt.Printf("Service %s is running!\n", id)
	} else {
		fmt.Println("Timed out waiting for service1 to be running.")
	}
}
```

## API Reference

### Types

#### `State`

```go
type State string
```

A string type representing the state of dependencies.

Constants:

- `DependenciesMet`: All dependencies are in the desired state.
- `DependenciesNotMet`: At least one dependency is not in the desired state.
- `DependenciesUnknown`: The state of dependencies is unknown.

#### `DependencyState[T any]`

```go
type DependencyState[T any] interface {
Set(id string, state State)
Add(t ...T)
Remove(t ...T)
CurrentState() State
Chan() <-chan State
WaitForDependencies(ctx context.Context, timeout time.Duration) bool
WaitUntilState(ctx context.Context, expectedState State, timeout time.Duration) error
GetDependencyStates() map[string]State
IsDependencyMet(id string) bool
WaitForAny(ctx context.Context, ids []string, timeout time.Duration) (string, error)
}
```

An interface for managing dependency states:

- `Set`: Sets the state of a dependency with the given ID.
- `Add`: Adds dependencies to be tracked.
- `Remove`: Removes dependencies from being tracked.
- `CurrentState`: Returns the current state of all dependencies.
- `Chan`: Returns a channel that will receive state changes.
- `WaitForDependencies`: Waits for dependencies to be met with a timeout. Returns true if dependencies are met, false if
  the timeout is reached.
- `WaitUntilState`: Waits until the dependencies reach the expected state or timeout. Returns an error if the timeout is
  reached.
- `GetDependencyStates`: Returns a snapshot of all dependencies and their states.
- `IsDependencyMet`: Returns true if the dependency with the given ID is in the desired state.
- `WaitForAny`: Waits for any of the specified dependencies to be in the desired state. Returns the ID of the first
  dependency that is in the desired state, or an empty string if the timeout is reached.

#### `IDStateFunc[T any]`

```go
type IDStateFunc[T any] func (t T) (string, State)
```

A function type that extracts an ID and state from a dependency.

### Functions

#### `NewDependencyState[T any]`

```go
func NewDependencyState[T any](ctx context.Context, idStateFunc IDStateFunc[T], desiredState State, c <-chan any) (DependencyState[T], <-chan State)
```

Creates a new DependencyState with the given dependencies and desired state. It returns the DependencyState and a
channel that will receive state changes.

#### `NewDependencyStateWithTopic[T any]`

```go
func NewDependencyStateWithTopic[T any](ctx context.Context, idStateFunc IDStateFunc[T], desiredState State, topic pubsub.Topic) (DependencyState[T], <-chan State)
```

Creates a new DependencyState with the given dependencies and desired state, using a pubsub.Topic for event
subscription. This is a more convenient way to create a DependencyState when you already have a pubsub.Topic.

#### `NewDependencyStateWithBuffer[T any]`

```go
func NewDependencyStateWithBuffer[T any](ctx context.Context, idStateFunc IDStateFunc[T], desiredState State, topic pubsub.Topic, bufferSize int) (DependencyState[T], <-chan State)
```

Creates a new DependencyState with the given dependencies and desired state, using a pubsub.Topic with a buffered
subscription for event subscription. This is useful when you want to avoid missing messages due to a full channel.

## License

MIT
