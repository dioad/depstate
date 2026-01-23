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

See [examples/basic/main.go](examples/basic/main.go) for a complete example.

```go
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
service1.Ready = true
topic.Publish(service1)
```

### Advanced Usage

See [examples/advanced/main.go](examples/advanced/main.go) for a complete example.

### Using WaitForDependencies and GetDependencyStates

See [examples/wait-for/main.go](examples/wait-for/main.go) for a complete example.

### Using IsDependencyMet and WaitForAny

See [examples/any-met/main.go](examples/any-met/main.go) for a complete example.

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
- `WaitForDependencies`: Waits for the overall state to become DependenciesMet with a timeout. It returns true if the
  dependencies are met within the timeout, false otherwise.
- `WaitUntilState`: Waits until the dependencies reach the expected state or timeout.
- `GetDependencyStates`: Returns a snapshot of all dependencies and their states.
- `IsDependencyMet`: Returns true if the dependency with the given ID is in the desired state.
- `WaitForAny`: Waits for any of the specified dependencies to reach the desired state. It returns the ID of the first
  dependency that reaches the desired state, or an empty string and an error if the timeout is reached or context is
  cancelled.

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

Apache License 2.0
