package depstate

import (
	"context"
	"sync"

	"github.com/dioad/pubsub"
)

type State string

const (
	DependenciesMet     State = "dependencies_met"
	DependenciesUnmet   State = "dependencies_unmet"
	DependenciesUnknown State = "dependencies_unknown"
)

type DependencyState[T any] interface {
	Set(id string, state State)
	Add(t ...T)
	Remove(t ...T)
	CurrentState() State
	Chan() <-chan State
}

type IDStateFunc[T any] func(t T) (string, State)

func NewDependencyState[T any](ctx context.Context, idStateFunc IDStateFunc[T], desiredState State, c <-chan any) (DependencyState[T], <-chan State) {
	ds := newDependencyState(idStateFunc, desiredState)
	depChan := ds.SubscribeTo(ctx, c)
	return ds, depChan
}

// DependencyState represents a state that is dependent on other states.
type dependencyState[T any] struct {
	dependencies sync.Map
	transitions  pubsub.Topic

	desiredState State
	currentState State

	idStateFunc IDStateFunc[T]
}

// NewDependencyState creates a new DependencyState with the given dependencies and desired state.
func newDependencyState[T any](idStateFunc IDStateFunc[T], desiredState State) *dependencyState[T] {
	s := &dependencyState[T]{
		// dependencies: make(map[string]State),
		transitions:  pubsub.NewTopic(),
		currentState: DependenciesUnknown,
		desiredState: desiredState,
		idStateFunc:  idStateFunc,
	}

	return s
}

// Set sets the state of the given dependency.
func (d *dependencyState[T]) Set(id string, state State) {
	d.set(id, state, true)
}

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

// SubscribeTo subscribes to a channel of dependencies and returns two channels that will receive
// the current state of the dependencies whenever it changes. The first channel will receive the
// state when the dependencies are met, and the second channel will receive the state when the
// dependencies are unmet.
func (d *dependencyState[T]) SubscribeTo(ctx context.Context, events <-chan any) <-chan State {

	// unmetChan := pubsub.SubscribeWithFilter(d.transitions, func(s State) bool { return s == DependenciesUnmet })
	// metChan := pubsub.SubscribeWithFilter(d.transitions, func(s State) bool { return s == DependenciesMet })

	go func() {
		c := pubsub.CastChan[T](events)

		for {
			select {
			case <-ctx.Done():
				return
			case t, ok := <-c:
				if !ok {
					return
				}

				id, state := d.idStateFunc(t)
				d.dependencies.Store(id, state)

				d.assessState()
			}
		}
	}()
	return pubsub.CastChan[State](d.transitions.Subscribe())
}

func (d *dependencyState[T]) publishState(state State) {
	d.transitions.Publish(state)
}

func (d *dependencyState[T]) assessState() {
	newState := DependenciesMet
	d.dependencies.Range(func(_, value interface{}) bool {
		if value.(State) != d.desiredState {
			newState = DependenciesUnmet
			return false
		}
		return true
	})

	if newState != d.currentState {
		d.currentState = newState
		d.publishState(d.currentState)
	}
}

// CurrentState returns the current state of the dependencies.
func (d *dependencyState[T]) CurrentState() State {
	return d.currentState
}

// Chan returns a channel that will receive the current state of the dependencies
// whenever it changes.
func (d *dependencyState[T]) Chan() <-chan State {
	return pubsub.CastChan[State](d.transitions.Subscribe())
}
