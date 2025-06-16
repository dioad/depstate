package depstate

import (
	"context"
	"testing"
	"time"

	"github.com/dioad/pubsub"
)

// TestWaitForDependencies tests the WaitForDependencies method.
func TestWaitForDependencies(t *testing.T) {
	testDepOne := testDep{
		id:    "1",
		state: "Sad",
	}
	testDepTwo := testDep{
		id:    "2",
		state: "Sad",
	}

	topic := pubsub.NewTopic()
	ds, _ := NewDependencyState(context.Background(), testIDStateFunc, State("Happy"), topic.Subscribe())

	// Add initial dependencies
	ds.Add(testDepOne, testDepTwo)

	// Start a goroutine to update the state after a delay
	go func() {
		time.Sleep(5 * time.Millisecond)
		testDepOne.state = "Happy"
		topic.Publish(testDepOne)
		time.Sleep(5 * time.Millisecond)
		testDepTwo.state = "Happy"
		topic.Publish(testDepTwo)
	}()

	// Wait for dependencies to be met with a timeout
	result := ds.WaitForDependencies(context.Background(), 500*time.Millisecond)
	if !result {
		t.Errorf("Expected WaitForDependencies to return true, got false")
	}

	// Test timeout
	testDepOne.state = "Sad"
	topic.Publish(testDepOne)

	// Wait for a bit to ensure the state change is processed
	time.Sleep(10 * time.Millisecond)

	// Wait for dependencies to be met with a short timeout
	result = ds.WaitForDependencies(context.Background(), 10*time.Millisecond)
	if result {
		t.Errorf("Expected WaitForDependencies to return false due to timeout, got true")
	}
}

// TestGetDependencyStates tests the GetDependencyStates method.
func TestGetDependencyStates(t *testing.T) {
	testDepOne := testDep{
		id:    "1",
		state: "Sad",
	}
	testDepTwo := testDep{
		id:    "2",
		state: "Happy",
	}

	topic := pubsub.NewTopic()
	ds, _ := NewDependencyState(context.Background(), testIDStateFunc, State("Happy"), topic.Subscribe())

	// Add initial dependencies
	ds.Add(testDepOne, testDepTwo)

	// Get dependency states
	states := ds.GetDependencyStates()
	if len(states) != 2 {
		t.Errorf("Expected 2 dependencies, got %d", len(states))
	}

	if states["1"] != "Sad" {
		t.Errorf("Expected state of dependency 1 to be Sad, got %v", states["1"])
	}

	if states["2"] != "Happy" {
		t.Errorf("Expected state of dependency 2 to be Happy, got %v", states["2"])
	}

	// Update a dependency
	testDepOne.state = "Happy"
	topic.Publish(testDepOne)

	// Wait for a bit to ensure the state change is processed
	time.Sleep(10 * time.Millisecond)

	// Get dependency states again
	states = ds.GetDependencyStates()
	if states["1"] != "Happy" {
		t.Errorf("Expected state of dependency 1 to be Happy, got %v", states["1"])
	}
}

// TestWaitForDependenciesContextCancellation tests that WaitForDependencies
// returns when the context is canceled.
func TestWaitForDependenciesContextCancellation(t *testing.T) {
	testDepOne := testDep{
		id:    "1",
		state: "Sad",
	}

	topic := pubsub.NewTopic()
	ds, _ := NewDependencyState(context.Background(), testIDStateFunc, State("Happy"), topic.Subscribe())

	// Add initial dependency
	ds.Add(testDepOne)

	// Create a context with cancellation
	ctx, cancel := context.WithCancel(context.Background())

	// Start a goroutine to cancel the context after a delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	// Wait for dependencies to be met with a long timeout
	result := ds.WaitForDependencies(ctx, 1*time.Second)
	if result {
		t.Errorf("Expected WaitForDependencies to return false due to context cancellation, got true")
	}
}
