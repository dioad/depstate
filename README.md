# Dependency State Management

This repository provides a Go package for managing multiple dependency states. It allows you to track the state of a set of dependencies and react to state changes efficiently.

## Features

- **State Management**: Track and manage the state of multiple dependencies.
- **Concurrency**: Safe concurrent access to dependency states.
- **Pub/Sub**: Publish and subscribe to state changes using a pub/sub mechanism.

## Installation

To install the package, use `go get`:

```sh
go get github.com/dioad/depstate
```

## Usage

### Creating a Dependency State

To create a new dependency state, use the `NewDependencyState` function:

```go
package main

import (
    "context"
    "github.com/yourusername/depstate"
)

type MyDependency struct {
    ID    string
    State string
}

func myIDStateFunc(dep MyDependency) (string, depstate.State) {
    return dep.ID, depstate.State(dep.State)
}

func main() {
    ctx := context.Background()
    topic := pubsub.NewTopic()
    deps := []MyDependency{
        {ID: "1", State: "Sad"},
        {ID: "2", State: "Sad"},
    }

    ds, depChan := depstate.NewDependencyState(ctx, myIDStateFunc, depstate.DependenciesMet, topic.Subscribe())
    ds.Add(deps...)

    // Handle state changes
    go func() {
        for state := range depChan {
            // React to state changes
            fmt.Println("New state:", state)
        }
    }()
	
	// Make some state changes
    ds.SetState("1", depstate.State("Happy"))
    ds.SetState("2", depstate.State("Happy"))
}
```

### Adding and Removing Dependencies

You can add and remove dependencies using the `Add` and `Remove` methods:

```go
ds.Add(MyDependency{ID: "3", State: "Happy"})
ds.Remove(MyDependency{ID: "1", State: "Sad"})
```

## Testing

To run the tests, use the `go test` command:

```sh
go test ./...
```

## License

This project is licensed under the Apache License. See the `LICENSE` file for details.