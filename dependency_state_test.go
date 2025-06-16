package depstate

import (
	"testing"
	"time"
)

// // testDep represents a test dependency with an ID and state
// type testDep struct {
// 	id    string
// 	state string
// }
//
// // testIDStateFunc extracts the ID and state from a testDep
// func testIDStateFunc(t testDep) (string, State) {
// 	return t.id, State(t.state)
// }
//
// // setupTest creates a common test setup with two dependencies
// // Returns the dependencies, topic, dependency state tracker, and context
// func setupTest() (context.Context, testDep, testDep, pubsub.Topic, DependencyState[testDep]) {
// 	// Arrange: Create test dependencies
// 	dep1 := testDep{id: "1", state: "Sad"}
// 	dep2 := testDep{id: "2", state: "Sad"}
//
// 	// Create a topic with buffered channel to avoid missing messages
// 	topic := pubsub.NewTopic()
// 	topicChan := topic.SubscribeWithBuffer(10)
//
// 	// Create context for the test
// 	ctx := context.Background()
//
// 	// Create the dependency state tracker with "Happy" as the desired state
// 	ds, _ := NewDependencyState(ctx, testIDStateFunc, State("Happy"), topicChan)
//
// 	return ctx, dep1, dep2, topic, ds
// }
//
// // assertStateEquals asserts that the current state equals the expected state
// // Waits for the state to change if necessary
// func assertStateEquals(t *testing.T, ds DependencyState[testDep], expectedState State, timeout time.Duration) {
// 	t.Helper()
//
// 	// Wait for the state to change to the expected state
// 	err := ds.WaitUntilState(context.Background(), expectedState, timeout)
// 	if err != nil {
// 		t.Fatalf("Failed to reach state %v: %v", expectedState, err)
// 	}
//
// 	// Verify the current state directly
// 	if ds.CurrentState() != expectedState {
// 		t.Errorf("Expected state to be %v, got %v", expectedState, ds.CurrentState())
// 	}
// }

// TestInitialDependencyState tests that the initial state is correct after adding dependencies
func TestInitialDependencyState(t *testing.T) {
	// Arrange: Set up the test
	_, dep1, dep2, topic, ds := setupTest()

	// Act: Add dependencies and publish their initial state
	ds.Add(dep1, dep2)
	topic.Publish(dep1, dep2)

	// Assert: Verify the initial state is DependenciesNotMet
	assertStateEquals(t, ds, DependenciesNotMet, 500*time.Millisecond)
}

// TestPartialDependencyUpdate tests that updating only one dependency doesn't change the overall state
func TestPartialDependencyUpdate(t *testing.T) {
	// Arrange: Set up the test with dependencies added
	_, dep1, dep2, topic, ds := setupTest()
	ds.Add(dep1, dep2)
	topic.Publish(dep1, dep2)

	// Wait for initial state to be established
	assertStateEquals(t, ds, DependenciesNotMet, 500*time.Millisecond)

	// Act: Update only the first dependency to the desired state
	dep1.state = "Happy"
	topic.Publish(dep1)

	assertStateEquals(t, ds, DependenciesNotMet, 500*time.Millisecond)
}

// TestAllDependenciesMet tests that updating all dependencies to the desired state changes the overall state
func TestAllDependenciesMet(t *testing.T) {
	// Arrange: Set up the test with dependencies added and one already updated
	_, dep1, dep2, topic, ds := setupTest()
	ds.Add(dep1, dep2)
	topic.Publish(dep1, dep2)

	dep1.state = "Happy"
	topic.Publish(dep1)

	// Act: Update the second dependency to the desired state
	dep2.state = "Happy"
	topic.Publish(dep2)

	// Assert: Verify the state changes to DependenciesMet
	assertStateEquals(t, ds, DependenciesMet, 500*time.Millisecond)
}

// TestDependencyStateTransitions tests state transitions when dependencies change
func TestDependencyStateTransitions(t *testing.T) {
	// Arrange: Set up the test with all dependencies in the desired state
	_, dep1, dep2, topic, ds := setupTest()
	ds.Add(dep1, dep2)

	dep1.state = "Happy"
	dep2.state = "Happy"
	topic.Publish(dep1, dep2)

	// Wait for initial state to be established
	assertStateEquals(t, ds, DependenciesMet, 500*time.Millisecond)

	// Act & Assert 1: Change one dependency to undesired state
	dep2.state = "Sad"
	topic.Publish(dep2)
	assertStateEquals(t, ds, DependenciesNotMet, 500*time.Millisecond)

	// Act & Assert 2: Change back to desired state
	dep2.state = "Happy"
	topic.Publish(dep2)
	assertStateEquals(t, ds, DependenciesMet, 500*time.Millisecond)
}
