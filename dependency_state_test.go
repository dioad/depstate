package depstate

import (
	"testing"
	"time"
)

// TestInitialDependencyState tests that the initial state is correct after adding dependencies.
func TestInitialDependencyState(t *testing.T) {
	// Arrange: Set up the test
	_, dep1, dep2, topic, ds := setupTest()

	// Act: Add dependencies and publish their initial state
	ds.Add(dep1, dep2)
	topic.Publish(dep1, dep2)

	// Assert: Verify the initial state is DependenciesNotMet
	assertStateEquals(t, ds, DependenciesNotMet)
}

// TestPartialDependencyUpdate tests that updating only one dependency doesn't change the overall state.
func TestPartialDependencyUpdate(t *testing.T) {
	// Arrange: Set up the test with dependencies added
	_, dep1, dep2, topic, ds := setupTest()
	ds.Add(dep1, dep2)
	topic.Publish(dep1, dep2)

	// Wait for initial state to be established
	assertStateEquals(t, ds, DependenciesNotMet)

	// Act: Update only the first dependency to the desired state
	dep1.state = "Happy"
	topic.Publish(dep1)

	assertStateEquals(t, ds, DependenciesNotMet)
}

// TestAllDependenciesMet tests that updating all dependencies to the desired state changes the overall state.
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
	assertStateEquals(t, ds, DependenciesMet)
}

func TestRemove(t *testing.T) {
	_, dep1, dep2, topic, ds := setupTest()
	ds.Add(dep1, dep2)
	topic.Publish(dep1, dep2)

	assertStateEquals(t, ds, DependenciesNotMet)

	// Update dep1 to Happy
	dep1.state = "Happy"
	topic.Publish(dep1)
	assertStateEquals(t, ds, DependenciesNotMet)

	// Remove dep2, overall state should become Happy
	ds.Remove(dep2)
	assertStateEquals(t, ds, DependenciesMet)
}

func TestSet(t *testing.T) {
	_, dep1, dep2, _, ds := setupTest()
	ds.Add(dep1, dep2)

	// Manually set state for dep1
	ds.Set("1", "Happy")
	if !ds.IsDependencyMet("1") {
		t.Errorf("Expected dep1 to be met")
	}
	assertStateEquals(t, ds, DependenciesNotMet)

	// Manually set state for dep2
	ds.Set("2", "Happy")
	if !ds.IsDependencyMet("2") {
		t.Errorf("Expected dep2 to be met")
	}
	assertStateEquals(t, ds, DependenciesMet)
}

func TestChan(t *testing.T) {
	ctx, dep1, _, topic, ds := setupTest()
	ds.Add(dep1)

	stateChan := ds.Chan(ctx)

	// Update dep1 to Happy
	dep1.state = "Happy"
	topic.Publish(dep1)

	select {
	case state := <-stateChan:
		if state != DependenciesMet {
			t.Errorf("Expected state DependenciesMet from Chan, got %v", state)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timed out waiting for state change from Chan")
	}
}

// TestDependencyStateTransitions tests state transitions when dependencies change.
func TestDependencyStateTransitions(t *testing.T) {
	// Arrange: Set up the test with all dependencies in the desired state
	_, dep1, dep2, topic, ds := setupTest()
	ds.Add(dep1, dep2)

	dep1.state = "Happy"
	dep2.state = "Happy"
	topic.Publish(dep1, dep2)

	// Wait for initial state to be established
	assertStateEquals(t, ds, DependenciesMet)

	// Act & Assert 1: Change one dependency to undesired state
	dep2.state = "Sad"
	topic.Publish(dep2)
	assertStateEquals(t, ds, DependenciesNotMet)

	// Act & Assert 2: Change back to desired state
	dep2.state = "Happy"
	topic.Publish(dep2)
	assertStateEquals(t, ds, DependenciesMet)
}
