// Package depstate provides a way to track the state of dependencies and emit events when all dependencies are in a desired state.
package depstate

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dioad/pubsub"
)

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
	// The returned channel will receive the current state of the dependencies whenever it changes.
	// The channel is closed when ctx is cancelled.
	Chan(ctx context.Context) <-chan State
	// WaitForDependencies waits for the overall state to become DependenciesMet with a timeout.
	// It returns true if the dependencies are met within the timeout, false otherwise.
	WaitForDependencies(ctx context.Context, timeout time.Duration) bool
	// WaitUntilState waits until the dependencies reach the expected state or timeout.
	WaitUntilState(ctx context.Context, expectedState State, timeout time.Duration) error
	// GetDependencyStates returns a snapshot of all dependencies and their states.
	GetDependencyStates() map[string]State
	// IsDependencyMet returns true if the dependency with the given ID is in the desired state.
	IsDependencyMet(id string) bool
	// WaitForAny waits for any of the specified dependencies to reach the desired state.
	// It returns the ID of the first dependency that reaches the desired state, or an empty string and an error if the timeout is reached or context is cancelled.
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

// Set sets the state of a dependency with the given ID.
// This will trigger an assessment of the overall state.
func (d *dependencyState[T]) Set(id string, state State) {
	d.dependencies.Store(id, state)
	d.assessState()
}

// Add adds the given dependencies to be tracked.
// The initial state of each dependency is determined by the idStateFunc.
// This will trigger an assessment of the overall state.
func (d *dependencyState[T]) Add(t ...T) {
	for _, dep := range t {
		id, state := d.idStateFunc(dep)
		d.dependencies.Store(id, state)
	}
	d.assessState()
}

// Remove removes the given dependencies from being tracked.
// This will trigger an assessment of the overall state.
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

// eventProcessorShutdownTimeout is the maximum time to wait for the processEvents
// goroutine to acknowledge context cancellation. The goroutine checks ctx.Done()
// on every iteration, so in practice it exits almost immediately; this value is
// a safety net to prevent the outer goroutine from blocking indefinitely if the
// events channel stalls during shutdown.
const eventProcessorShutdownTimeout = 100 * time.Millisecond

// waitForEventProcessor waits for the event processor goroutine to exit.
func (d *dependencyState[T]) waitForEventProcessor(done <-chan struct{}) {
	select {
	case <-done:
	case <-time.After(eventProcessorShutdownTimeout):
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
func (d *dependencyState[T]) forwardStateUpdates(ctx context.Context, done <-chan struct{}, stateChan <-chan any, resultChan chan State) {
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

// sendStateUpdate sends a state update to the result channel. resultChan carries
// current-state semantics: consumers only care about the latest value, so when
// the buffer is already full of a stale, superseded state, that stale value is
// replaced rather than the fresh one being dropped. Dropping the fresh value
// would otherwise let a later, real transition (e.g. DependenciesMet ->
// DependenciesNotMet) go unseen whenever the consumer hasn't yet drained an
// earlier update, since assessState only republishes on an actual state change.
func (d *dependencyState[T]) sendStateUpdate(ctx context.Context, state State, resultChan chan State) {
	select {
	case <-ctx.Done():
		// Context was cancelled, don't send any more messages
		return
	case resultChan <- state:
		// State update sent successfully
		return
	default:
	}

	// Buffer full: discard the stale value, then land the fresh one.
	select {
	case <-resultChan:
	default:
	}

	select {
	case resultChan <- state:
	case <-ctx.Done():
	}
}

// publishState publishes a state change to the transitions topic. It uses
// PublishReliable rather than Publish: Topic.Subscribe hands out a buffer-1
// channel with non-blocking, drop-on-full delivery, and assessState only
// republishes on an actual state change (no retry), so a dropped publish is
// permanently lost. Since every subscriber here (forwardStateUpdates, and
// any Chan/WaitUntilState/WaitForAny caller) does trivial O(1) work per
// message, the bounded block this introduces is negligible in practice and
// far preferable to silently losing a transition.
func (d *dependencyState[T]) publishState(state State) {
	d.transitions.PublishReliable(state)
}

// assessState assesses the overall state of the dependencies and publishes a state change if necessary.
func (d *dependencyState[T]) assessState() {
	newState := d.calculateState()
	for {
		current := d.loadCurrentState()
		if current == newState {
			return
		}
		if d.currentState.CompareAndSwap(current, newState) {
			d.publishState(newState)
			return
		}
	}
}

// loadCurrentState returns the current overall state. currentState is only
// ever stored as a State, so the assertion failing would indicate a bug
// elsewhere; it is still checked to satisfy forcetypeassert and to avoid a
// panic if that invariant is ever violated.
func (d *dependencyState[T]) loadCurrentState() State {
	state, ok := d.currentState.Load().(State)
	if !ok {
		return DependenciesUnknown
	}
	return state
}

// calculateState calculates the overall state of the dependencies.
func (d *dependencyState[T]) calculateState() State {
	empty := true
	newState := DependenciesMet
	d.dependencies.Range(func(_, value any) bool {
		empty = false
		state, ok := value.(State)
		if !ok || state != d.desiredState {
			newState = DependenciesNotMet
			return false
		}
		return true
	})
	if empty {
		return DependenciesUnknown
	}
	return newState
}

// CurrentState returns the current overall state of all tracked dependencies.
func (d *dependencyState[T]) CurrentState() State {
	return d.loadCurrentState()
}

// Chan returns a channel that will receive the current state of the dependencies
// whenever it changes. The channel is closed when ctx is cancelled.
func (d *dependencyState[T]) Chan(ctx context.Context) <-chan State {
	rawChan := d.transitions.Subscribe()
	typedChan := pubsub.CastChan[State](rawChan)
	go func() {
		<-ctx.Done()
		d.transitions.Unsubscribe(rawChan)
	}()
	return typedChan
}

// WaitUntilState waits until the dependencies reach the expected state or timeout.
func (d *dependencyState[T]) WaitUntilState(ctx context.Context, expectedState State, timeout time.Duration) error {
	return d.waitForStateWithTimeout(ctx, expectedState, timeout)
}

// waitForStateWithTimeout waits for the dependencies to reach the expected state with a timeout.
func (d *dependencyState[T]) waitForStateWithTimeout(ctx context.Context, expectedState State, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Subscribe before checking current state to avoid missing a transition in between.
	rawChan := d.transitions.Subscribe()
	stateChan := pubsub.CastChan[State](rawChan)
	defer d.transitions.Unsubscribe(rawChan)

	if d.CurrentState() == expectedState {
		return nil
	}

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

// WaitForDependencies waits for the overall state to become DependenciesMet with a timeout.
// It returns true if the dependencies are met within the timeout, false otherwise.
func (d *dependencyState[T]) WaitForDependencies(ctx context.Context, timeout time.Duration) bool {
	return d.WaitUntilState(ctx, DependenciesMet, timeout) == nil
}

// GetDependencyStates returns a snapshot of all dependencies and their states.
func (d *dependencyState[T]) GetDependencyStates() map[string]State {
	result := make(map[string]State)
	d.dependencies.Range(func(key, value any) bool {
		id, ok := key.(string)
		if !ok {
			return true
		}
		state, ok := value.(State)
		if !ok {
			return true
		}
		result[id] = state
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
	state, ok := value.(State)
	if !ok {
		return false
	}
	return state == d.desiredState
}

// WaitForAny waits for any of the specified dependencies to reach the desired state.
// It returns the ID of the first dependency that reaches the desired state, or an empty string and an error if the timeout is reached or context is cancelled.
func (d *dependencyState[T]) WaitForAny(ctx context.Context, ids []string, timeout time.Duration) (string, error) {
	if len(ids) == 0 {
		return "", errors.New("WaitForAny requires at least one dependency ID")
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Subscribe before checking current state to avoid missing a transition in between.
	rawChan := d.transitions.Subscribe()
	stateChan := pubsub.CastChan[State](rawChan)
	defer d.transitions.Unsubscribe(rawChan)

	if id, ok := d.firstMetDependency(ids); ok {
		return id, nil
	}

	for {
		select {
		case <-ctx.Done():
			if id, ok := d.firstMetDependency(ids); ok {
				return id, nil
			}
			return "", ctx.Err()
		case _, ok := <-stateChan:
			if !ok {
				return "", ctx.Err()
			}
			if id, ok := d.firstMetDependency(ids); ok {
				return id, nil
			}
		}
	}
}

// firstMetDependency returns the first ID in ids whose dependency is currently
// in the desired state.
func (d *dependencyState[T]) firstMetDependency(ids []string) (string, bool) {
	for _, id := range ids {
		if d.IsDependencyMet(id) {
			return id, true
		}
	}
	return "", false
}
