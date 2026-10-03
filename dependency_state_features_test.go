package depstate

import (
	"context"
	"testing"
	"time"

	"github.com/dioad/pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWaitForDependencies tests the WaitForDependencies method.
func TestWaitForDependencies(t *testing.T) {
	t.Parallel()

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
	assert.True(t, result, "expected WaitForDependencies to return true")

	// Test timeout
	testDepOne.state = "Sad"
	topic.Publish(testDepOne)

	// Wait for the state change to be processed before testing the short timeout below.
	assertStateEquals(t, ds, DependenciesNotMet)

	// Wait for dependencies to be met with a short timeout
	result = ds.WaitForDependencies(context.Background(), 10*time.Millisecond)
	assert.False(t, result, "expected WaitForDependencies to return false due to timeout")
}

// TestGetDependencyStates tests the GetDependencyStates method.
func TestGetDependencyStates(t *testing.T) {
	t.Parallel()

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
	assert.Len(t, states, 2)
	assert.Equal(t, State("Sad"), states["1"])
	assert.Equal(t, State("Happy"), states["2"])

	// Update a dependency
	testDepOne.state = "Happy"
	topic.Publish(testDepOne)

	// Wait for the state change to be processed before re-reading states.
	_, err := ds.WaitForAny(context.Background(), []string{testDepOne.id}, time.Second)
	require.NoError(t, err)

	// Get dependency states again
	states = ds.GetDependencyStates()
	assert.Equal(t, State("Happy"), states["1"])
}

// TestWaitForDependenciesContextCancellation tests that WaitForDependencies
// returns when the context is canceled.
func TestWaitForDependenciesContextCancellation(t *testing.T) {
	t.Parallel()

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
	assert.False(t, result, "expected WaitForDependencies to return false due to context cancellation")
}
