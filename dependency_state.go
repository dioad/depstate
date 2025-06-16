// Package depstate provides a way to track the state of dependencies and emit events when all dependencies are in a desired state.
package depstate

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dioad/pubsub"
)

// State represents the state of dependencies.
// type State string
//
// const (
// 	// DependenciesMet indicates that all dependencies are in the desired state.
// 	DependenciesMet State = "dependencies_met"
// 	// DependenciesNotMet indicates that at least one dependency is not in the desired state.
// 	DependenciesNotMet State = "dependencies_notmet"
// 	// DependenciesUnknown indicates that the state of dependencies is unknown.
// 	DependenciesUnknown State = "dependencies_unknown"
// )

// DependencyState is an interface for managing dependency states.
type DependencyState[T any] interface {
	// Set sets the state of a dependency with the given ID.
	Set(id string, state State)
	// Add adds dependencies to be tracked.
	Add(t ...T)
	// Remove removes dependencies from being tracked.
	Remove(t ...T)
	// CurrentState returns the current state of all dependencies.
	CurrentState() State
	// Chan returns a channel that will receive state changes.
	Chan() <-chan State
	// WaitForDependencies waits for dependencies to be met with a timeout.
	// Returns true if dependencies are met, false if the timeout is reached.
	WaitForDependencies(ctx context.Context, timeout time.Duration) bool
	// WaitUntilState waits until the dependencies reach the expected state or timeout.
	WaitUntilState(ctx context.Context, expectedState State, timeout time.Duration) error
	// GetDependencyStates returns a snapshot of all dependencies and their states.
	GetDependencyStates() map[string]State
	// IsDependencyMet returns true if the dependency with the given ID is in the desired state.
	IsDependencyMet(id string) bool
	// WaitForAny waits for any of the specified dependencies to be in the desired state.
	// Returns the ID of the first dependency that is in the desired state, or an empty string if the timeout is reached.
	WaitForAny(ctx context.Context, ids []string, timeout time.Duration) (string, error)
}

// IDStateFunc is a function that extracts an ID and state from a dependency.
type IDStateFunc[T any] func(t T) (string, State)

// NewDependencyState creates a new DependencyState with the given dependencies and desired state.
// It returns the DependencyState and a channel that will receive state changes.
func NewDependencyState[T any](ctx context.Context, idStateFunc IDStateFunc[T], desiredState State, c <-chan any) (DependencyState[T], <-chan State) {
	ds := newDependencyState(idStateFunc, desiredState)
	depChan := ds.SubscribeTo(ctx, c)
	return ds, depChan
}

// NewDependencyStateWithTopic creates a new DependencyState with the given dependencies and desired state,
// using a pubsub.Topic for event subscription. This is a more convenient way to create a DependencyState
// when you already have a pubsub.Topic.
func NewDependencyStateWithTopic[T any](ctx context.Context, idStateFunc IDStateFunc[T], desiredState State, topic pubsub.Topic) (DependencyState[T], <-chan State) {
	return NewDependencyState(ctx, idStateFunc, desiredState, topic.Subscribe())
}

// NewDependencyStateWithBuffer creates a new DependencyState with the given dependencies and desired state,
// using a pubsub.Topic with a buffered subscription for event subscription. This is useful when you want
// to avoid missing messages due to a full channel.
func NewDependencyStateWithBuffer[T any](ctx context.Context, idStateFunc IDStateFunc[T], desiredState State, topic pubsub.Topic, bufferSize int) (DependencyState[T], <-chan State) {
	return NewDependencyState(ctx, idStateFunc, desiredState, topic.SubscribeWithBuffer(bufferSize))
}

// dependencyState represents a state that is dependent on other states.
type dependencyState[T any] struct {
	dependencies sync.Map
	transitions  pubsub.Topic

	desiredState State
	currentState atomic.Value // Stores State

	idStateFunc IDStateFunc[T]
}

// newDependencyState creates a new dependencyState with the given dependencies and desired state.
func newDependencyState[T any](idStateFunc IDStateFunc[T], desiredState State) *dependencyState[T] {
	s := &dependencyState[T]{
		transitions:  pubsub.NewTopic(),
		desiredState: desiredState,
		idStateFunc:  idStateFunc,
	}

	// Initialize the atomic value
	s.currentState.Store(DependenciesUnknown)

	return s
}

// Set sets the state of the given dependency.
func (d *dependencyState[T]) Set(id string, state State) {
	d.set(id, state, true)
}

// set is a helper method that sets the state of a dependency and optionally assesses the overall state.
func (d *dependencyState[T]) set(id string, state State, assess bool) {
	d.dependencies.Store(id, state)
	if assess {
		d.assessState()
	}
}

// Add adds the given dependencies to the state.
func (d *dependencyState[T]) Add(t ...T) {
	for _, dep := range t {
		id, state := d.idStateFunc(dep)
		d.set(id, state, false)
	}
	d.assessState()
}

// Remove removes the given dependencies from the state.
func (d *dependencyState[T]) Remove(t ...T) {
	for _, dep := range t {
		id, _ := d.idStateFunc(dep)
		d.dependencies.Delete(id)
	}

	d.assessState()
}

// SubscribeTo subscribes to a channel of dependencies and returns a channel that will receive
// the current state of the dependencies whenever it changes.
// The returned channel will be closed when the context is canceled or the events channel is closed.
func (d *dependencyState[T]) SubscribeTo(ctx context.Context, events <-chan any) <-chan State {
	// Create a subscription to the transitions topic
	stateChan := d.transitions.Subscribe()

	// Create a channel for the state updates
	resultChan := make(chan State, 1)

	// Create a context that can be cancelled from within the goroutine
	innerCtx, innerCancel := context.WithCancel(ctx)

	// Start a goroutine to handle the subscription
	go func() {
		// Ensure we clean up resources when we're done
		defer d.cleanupSubscription(innerCancel, stateChan, resultChan)

		// Cast the events channel to the correct type
		c := pubsub.CastChan[T](events)

		// Create a channel to signal when the event processor is done
		done := make(chan struct{})
		defer d.waitForEventProcessor(done)

		// Start a goroutine to process events
		go d.processEvents(innerCtx, c, done)

		// Forward state updates from the transitions topic to the result channel
		d.forwardStateUpdates(innerCtx, done, stateChan, resultChan)
	}()

	return resultChan
}

// cleanupSubscription ensures that all resources are cleaned up when a subscription ends.
func (d *dependencyState[T]) cleanupSubscription(cancel context.CancelFunc, stateChan <-chan any, resultChan chan<- State) {
	cancel()
	d.transitions.Unsubscribe(stateChan)
	close(resultChan)
}

// waitForEventProcessor waits for the event processor goroutine to exit.
func (d *dependencyState[T]) waitForEventProcessor(done <-chan struct{}) {
	select {
	case <-done:
		// Event processing goroutine has exited
	case <-time.After(100 * time.Millisecond):
		// Timeout waiting for event processing goroutine to exit
	}
}

// processEvents processes events from the input channel and updates dependency states.
func (d *dependencyState[T]) processEvents(ctx context.Context, events <-chan T, done chan<- struct{}) {
	defer close(done)

	for {
		select {
		case <-ctx.Done():
			return
		case t, ok := <-events:
			if !ok {
				return
			}

			d.processDependencyUpdate(t)
		}
	}
}

// processDependencyUpdate updates the state of a dependency if it exists.
func (d *dependencyState[T]) processDependencyUpdate(t T) {
	id, state := d.idStateFunc(t)
	_, exists := d.dependencies.Load(id)
	if exists {
		d.dependencies.Store(id, state)
		d.assessState()
	}
}

// forwardStateUpdates forwards state updates from the transitions topic to the result channel.
func (d *dependencyState[T]) forwardStateUpdates(ctx context.Context, done <-chan struct{}, stateChan <-chan any, resultChan chan<- State) {
	stateChanTyped := pubsub.CastChan[State](stateChan)

	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case state, ok := <-stateChanTyped:
			if !ok {
				return
			}

			d.sendStateUpdate(ctx, state, resultChan)
		}
	}
}

// sendStateUpdate sends a state update to the result channel, using non-blocking send to prevent deadlocks.
func (d *dependencyState[T]) sendStateUpdate(ctx context.Context, state State, resultChan chan<- State) {
	select {
	case <-ctx.Done():
		// Context was cancelled, don't send any more messages
		return
	case resultChan <- state:
		// State update sent successfully
	default:
		// Channel is full, skip this update
	}
}

// publishState publishes a state change to the transitions topic.
func (d *dependencyState[T]) publishState(state State) {
	d.transitions.Publish(state)
}

// assessState assesses the overall state of the dependencies and publishes a state change if necessary.
func (d *dependencyState[T]) assessState() {
	newState := d.calculateState()
	currentState := d.currentState.Load().(State)

	if newState != currentState {
		d.currentState.Store(newState)
		d.publishState(newState)
	}
}

// calculateState calculates the overall state of the dependencies.
func (d *dependencyState[T]) calculateState() State {
	newState := DependenciesMet
	d.dependencies.Range(func(key, value interface{}) bool {
		if value.(State) != d.desiredState {
			newState = DependenciesNotMet
			return false
		}
		return true
	})
	return newState
}

// CurrentState returns the current state of the dependencies.
func (d *dependencyState[T]) CurrentState() State {
	return d.currentState.Load().(State)
}

// Chan returns a channel that will receive the current state of the dependencies
// whenever it changes.
func (d *dependencyState[T]) Chan() <-chan State {
	return pubsub.CastChan[State](d.transitions.Subscribe())
}

// WaitUntilState waits until the dependencies reach the expected state or timeout.
func (d *dependencyState[T]) WaitUntilState(ctx context.Context, expectedState State, timeout time.Duration) error {
	// If dependencies are already in the expected state, return immediately
	if d.CurrentState() == expectedState {
		return nil
	}

	return d.waitForStateWithTimeout(ctx, expectedState, timeout)
}

// waitForStateWithTimeout waits for the dependencies to reach the expected state with a timeout.
func (d *dependencyState[T]) waitForStateWithTimeout(ctx context.Context, expectedState State, timeout time.Duration) error {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Subscribe to state changes
	rawChan := d.transitions.Subscribe()
	stateChan := pubsub.CastChan[State](rawChan)
	defer d.transitions.Unsubscribe(rawChan)

	return d.waitForStateChange(ctx, expectedState, stateChan)
}

// waitForStateChange waits for a state change to the expected state or for the context to be done.
func (d *dependencyState[T]) waitForStateChange(ctx context.Context, expectedState State, stateChan <-chan State) error {
	for {
		select {
		case <-ctx.Done():
			// Timeout or context canceled
			if d.CurrentState() == expectedState {
				return nil
			}
			return ctx.Err()
		case state, ok := <-stateChan:
			if !ok {
				// Channel closed
				if d.CurrentState() == expectedState {
					return nil
				}
				return ctx.Err()
			}
			if state == expectedState {
				return nil
			}
		}
	}
}

// WaitForDependencies waits for dependencies to be met with a timeout.
// Returns true if dependencies are met, false if the timeout is reached.
func (d *dependencyState[T]) WaitForDependencies(ctx context.Context, timeout time.Duration) bool {
	return d.WaitUntilState(ctx, DependenciesMet, timeout) == nil
}

// GetDependencyStates returns a snapshot of all dependencies and their states.
func (d *dependencyState[T]) GetDependencyStates() map[string]State {
	result := make(map[string]State)
	d.dependencies.Range(func(key, value interface{}) bool {
		result[key.(string)] = value.(State)
		return true
	})
	return result
}

// IsDependencyMet returns true if the dependency with the given ID is in the desired state.
func (d *dependencyState[T]) IsDependencyMet(id string) bool {
	value, ok := d.dependencies.Load(id)
	if !ok {
		return false
	}
	return value.(State) == d.desiredState
}

// WaitForAny waits for any of the specified dependencies to be in the desired state.
// Returns the ID of the first dependency that is in the desired state, or an empty string if the timeout is reached.
func (d *dependencyState[T]) WaitForAny(ctx context.Context, ids []string, timeout time.Duration) (string, error) {
	// Check if any dependency is already in the desired state
	for _, id := range ids {
		if d.IsDependencyMet(id) {
			return id, nil
		}
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Subscribe to state changes
	rawChan := d.transitions.Subscribe()
	stateChan := pubsub.CastChan[State](rawChan)
	defer d.transitions.Unsubscribe(rawChan)

	// Wait for any dependency to be in the desired state or for the context to be done
	for {
		select {
		case <-ctx.Done():
			// Timeout or context canceled
			// Check one more time in case we missed a state change
			for _, id := range ids {
				if d.IsDependencyMet(id) {
					return id, nil
				}
			}
			return "", ctx.Err()
		case _, ok := <-stateChan:
			if !ok {
				// Channel closed
				return "", ctx.Err()
			}
			// Check if any dependency is now in the desired state
			for _, id := range ids {
				if d.IsDependencyMet(id) {
					return id, nil
				}
			}
		}
	}
}
