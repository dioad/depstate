package depstate

import (
	"context"
	"testing"
	"time"

	"github.com/dioad/pubsub"
)

// testDep represents a test dependency with an ID and state
type testDep struct {
	id    string
	state string
}

// testIDStateFunc extracts the ID and state from a testDep
func testIDStateFunc(t testDep) (string, State) {
	return t.id, State(t.state)
}

// setupTest creates a common test setup with two dependencies
// Returns the context, dependencies, topic, and dependency state tracker
func setupTest() (context.Context, testDep, testDep, pubsub.Topic, DependencyState[testDep]) {
	// Arrange: Create test dependencies
	dep1 := testDep{id: "1", state: "Sad"}
	dep2 := testDep{id: "2", state: "Sad"}

	// Create a topic with buffered channel to avoid missing messages
	topic := pubsub.NewTopic()
	topicChan := topic.SubscribeWithBuffer(10)

	// Create context for the test
	ctx := context.Background()

	// Create the dependency state tracker with "Happy" as the desired state
	ds, _ := NewDependencyState(ctx, testIDStateFunc, State("Happy"), topicChan)

	return ctx, dep1, dep2, topic, ds
}

// assertStateEquals asserts that the current state equals the expected state
// Waits for the state to change if necessary
func assertStateEquals(t *testing.T, ds DependencyState[testDep], expectedState State, timeout time.Duration) {
	t.Helper()

	// Wait for the state to change to the expected state
	err := ds.WaitUntilState(context.Background(), expectedState, timeout)
	if err != nil {
		t.Fatalf("Failed to reach state %v: %v", expectedState, err)
	}

	// Verify the current state directly
	if ds.CurrentState() != expectedState {
		t.Errorf("Expected state to be %v, got %v", expectedState, ds.CurrentState())
	}
}

// waitForStateChange waits for a state change with a timeout
// Returns the new state or fails the test if the timeout is reached
// func waitForStateChange(t *testing.T, depChan <-chan State, timeout time.Duration) State {
// 	t.Helper()
//
// 	select {
// 	case state := <-depChan:
// 		return state
// 	case <-time.After(timeout):
// 		t.Fatalf("Timed out waiting for state change")
// 		return ""
// 	}
// }

// publishDependencyState publishes a dependency state change and waits for it to be processed
// func publishDependencyState(t *testing.T, dep *testDep, state string, topic pubsub.Topic, waitTime time.Duration) {
// 	t.Helper()
//
// 	dep.state = state
// 	topic.Publish(*dep)
// 	time.Sleep(waitTime) // Give time for the update to be processed
// }

// setupDependencies adds dependencies to the dependency state tracker and publishes their initial state
// func setupDependencies(t *testing.T, ds DependencyState[testDep], deps []testDep, topic pubsub.Topic) {
// 	t.Helper()
//
// 	// Add dependencies
// 	ds.Add(deps...)
//
// 	// Publish initial state
// 	for _, dep := range deps {
// 		topic.Publish(dep)
// 	}
//
// 	// Wait for the state to be processed
// 	time.Sleep(10 * time.Millisecond)
// }
