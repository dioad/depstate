package depstate

import (
	"context"
	"testing"
	"time"

	"github.com/dioad/pubsub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testDep represents a test dependency with an ID and state.
type testDep struct {
	id    string
	state string
}

// testIDStateFunc extracts the ID and state from a testDep.
func testIDStateFunc(t testDep) (string, State) {
	return t.id, State(t.state)
}

// setupTest creates a common test setup with two dependencies.
// Returns the context, dependencies, topic, and dependency state tracker.
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

// assertStateEqualsTimeout is the timeout used by assertStateEquals when
// waiting for the dependency state to reach the expected value.
const assertStateEqualsTimeout = 500 * time.Millisecond

// assertStateEquals asserts that the current state equals the expected state.
// Waits for the state to change if necessary.
func assertStateEquals(t *testing.T, ds DependencyState[testDep], expectedState State) {
	t.Helper()

	// Wait for the state to change to the expected state
	err := ds.WaitUntilState(context.Background(), expectedState, assertStateEqualsTimeout)
	require.NoErrorf(t, err, "failed to reach state %v", expectedState)

	// Verify the current state directly
	assert.Equal(t, expectedState, ds.CurrentState())
}

// waitForDependencyMetPollInterval is the polling cadence used by
// waitForDependencyMet.
const waitForDependencyMetPollInterval = time.Millisecond

// waitForDependencyMet polls IsDependencyMet until it returns true or the
// timeout elapses. The transitions topic underlying WaitUntilState and
// WaitForAny only publishes when the *overall* state changes, so there is no
// event to wait on for a single dependency whose update doesn't flip the
// aggregate state.
func waitForDependencyMet(t *testing.T, ds DependencyState[testDep], id string, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for !ds.IsDependencyMet(id) {
		require.Falsef(t, time.Now().After(deadline), "timed out waiting for dependency %s to be met", id)
		time.Sleep(waitForDependencyMetPollInterval)
	}
}
