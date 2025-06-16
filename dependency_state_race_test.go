package depstate

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/dioad/pubsub"
)

// TestConcurrentAccess tests that concurrent access to the dependency state
// doesn't cause race conditions.
func TestConcurrentAccess(t *testing.T) {
	testDepOne := testDep{
		id:    "1",
		state: "Sad",
	}
	testDepTwo := testDep{
		id:    "2",
		state: "Sad",
	}

	topic := pubsub.NewTopic()
	ds, depChan := NewDependencyState(context.Background(), testIDStateFunc, State("Happy"), topic.Subscribe())

	// Add initial dependencies
	ds.Add(testDepOne, testDepTwo)

	// Start a goroutine to read from the depChan
	go func() {
		for range depChan {
			// Just consume the messages
		}
	}()

	// Use a WaitGroup to wait for all goroutines to finish
	var wg sync.WaitGroup
	wg.Add(3)

	// Start multiple goroutines to update the state concurrently
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			testDepOne.state = "Happy"
			topic.Publish(testDepOne)
			time.Sleep(time.Millisecond)
			testDepOne.state = "Sad"
			topic.Publish(testDepOne)
			time.Sleep(time.Millisecond)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			testDepTwo.state = "Happy"
			topic.Publish(testDepTwo)
			time.Sleep(time.Millisecond)
			testDepTwo.state = "Sad"
			topic.Publish(testDepTwo)
			time.Sleep(time.Millisecond)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			// Read the current state
			_ = ds.CurrentState()
			time.Sleep(time.Millisecond)
		}
	}()

	// Wait for all goroutines to finish
	wg.Wait()
}

// TestContextCancellation tests that canceling the context properly cleans up resources.
func TestContextCancellation(t *testing.T) {
	testDepOne := testDep{
		id:    "1",
		state: "Sad",
	}

	topic := pubsub.NewTopic()
	ctx, cancel := context.WithCancel(context.Background())
	ds, depChan := NewDependencyState(ctx, testIDStateFunc, State("Happy"), topic.Subscribe())

	// Add initial dependency
	ds.Add(testDepOne)

	// Check initial state
	initialState := ds.CurrentState()
	if initialState != DependenciesNotMet {
		t.Errorf("Expected initial state to be %v, got %v", DependenciesNotMet, initialState)
	}

	// Publish a message to trigger a state change
	testDepOne.state = "Happy"
	topic.Publish(testDepOne)

	// Wait for the state change with a timeout
	var state State
	for {
		select {
		case s := <-depChan:
			state = s
			// t.Logf("Received state change: %v", state)
		case <-time.After(50 * time.Millisecond):
			t.Fatalf("Timed out waiting for state change, current state: %v", ds.CurrentState())
		}
		if state == DependenciesMet {
			break
		}
	}

	// Check the current state directly as well
	// currentState := ds.CurrentState()
	// t.Logf("Current state: %v", currentState)
	//
	// // Check the dependency states
	// depStates := ds.GetDependencyStates()
	// for id, s := range depStates {
	// 	t.Logf("Dependency %s state: %v", id, s)
	// }

	if state != DependenciesMet {
		t.Errorf("Expected state to be %v, got %v", DependenciesMet, state)
	}

	// Cancel the context
	cancel()

	// Wait a bit for the goroutines to clean up
	time.Sleep(200 * time.Millisecond)

	// Publish another message, which should not trigger a state change
	testDepOne.state = "Sad"
	topic.Publish(testDepOne)

	// Try to read from the channel, which should be closed or not receive any messages
	select {
	case state, ok := <-depChan:
		if ok {
			t.Errorf("Expected channel to be closed or not receive messages, got %v", state)
		}
	case <-time.After(200 * time.Millisecond):
		// This is expected, the channel should not receive any messages
	}
}

// TestRaceCondition tests that the dependency state correctly handles race conditions
// when multiple goroutines are updating the state concurrently.
func TestRaceCondition(t *testing.T) {
	testDepOne := testDep{
		id:    "1",
		state: "Sad",
	}
	testDepTwo := testDep{
		id:    "2",
		state: "Sad",
	}

	topic := pubsub.NewTopic()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // Ensure context is cancelled when test exits
	ds, depChan := NewDependencyState(ctx, testIDStateFunc, State("Happy"), topic.Subscribe())

	// Add initial dependencies
	ds.Add(testDepOne, testDepTwo)

	// Check initial state
	initialState := ds.CurrentState()
	if initialState != DependenciesNotMet {
		t.Errorf("Expected initial state to be %v, got %v", DependenciesNotMet, initialState)
	}

	// Use a WaitGroup to wait for the update goroutines to finish
	var updateWg sync.WaitGroup
	updateWg.Add(2) // Add 2 for the two update goroutines

	// Start a goroutine to count state changes
	stateChanges := make(chan State, 1000)
	stateDone := make(chan struct{})
	go func() {
		defer close(stateDone)
		for state := range depChan {
			t.Logf("Received state change: %v", state)
			stateChanges <- state
		}
		t.Logf("State change collector finished")
	}()

	// Start multiple goroutines to update the state concurrently
	go func() {
		defer updateWg.Done()
		for i := 0; i < 100; i++ {
			testDepOne.state = "Sad"
			topic.Publish(testDepOne)
			time.Sleep(time.Millisecond)
			testDepOne.state = "Happy"
			topic.Publish(testDepOne)
			time.Sleep(time.Millisecond)
		}
		t.Logf("Update goroutine 1 finished")
	}()

	go func() {
		defer updateWg.Done()
		for i := 0; i < 100; i++ {
			testDepTwo.state = "Happy"
			topic.Publish(testDepTwo)
			time.Sleep(time.Millisecond)
			testDepTwo.state = "Sad"
			topic.Publish(testDepTwo)
			time.Sleep(time.Millisecond)
		}
		t.Logf("Update goroutine 2 finished")
	}()

	// Wait for all update goroutines to finish
	updateWg.Wait()
	t.Logf("All update goroutines finished")

	// Wait a bit for any pending state changes
	time.Sleep(50 * time.Millisecond)

	// Check the current state
	currentState := ds.CurrentState()
	t.Logf("Current state after updates: %v", currentState)

	// Check the dependency states
	depStates := ds.GetDependencyStates()
	for id, s := range depStates {
		t.Logf("Dependency %s state: %v", id, s)
	}

	// Signal the state change collector to stop by closing depChan
	// Cancel the context to close the depChan
	t.Logf("Cancelling context")
	cancel()

	// Wait for the state change collector to finish
	select {
	case <-stateDone:
		t.Logf("State change collector finished")
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("Timed out waiting for state change collector to finish")
	}

	// Now it's safe to close and read from stateChanges
	close(stateChanges)
	changes := 0
	for range stateChanges {
		changes++
	}
	t.Logf("Received %d state changes", changes)

	if changes == 0 {
		t.Errorf("Expected to receive state changes, got none")
	}
}
