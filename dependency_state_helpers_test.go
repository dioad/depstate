package depstate

import (
	"context"
	"testing"
	"time"

	"github.com/dioad/pubsub"
)

// TestIsDependencyMet tests the IsDependencyMet method.
func TestIsDependencyMet(t *testing.T) {
	// Arrange: Set up the test
	_, dep1, dep2, topic, ds := setupTest()
	ds.Add(dep1, dep2)

	// Act & Assert: Initially, no dependencies are met
	if ds.IsDependencyMet(dep1.id) {
		t.Errorf("Expected dependency %s to not be met initially", dep1.id)
	}
	if ds.IsDependencyMet(dep2.id) {
		t.Errorf("Expected dependency %s to not be met initially", dep2.id)
	}

	// Act: Update the first dependency to the desired state
	dep1.state = "Happy"
	topic.Publish(dep1)

	// Wait for the state change to be processed
	time.Sleep(50 * time.Millisecond)

	// Assert: First dependency should be met, second should not
	if !ds.IsDependencyMet(dep1.id) {
		t.Errorf("Expected dependency %s to be met after update", dep1.id)
	}
	if ds.IsDependencyMet(dep2.id) {
		t.Errorf("Expected dependency %s to still not be met", dep2.id)
	}

	// Act: Update the second dependency to the desired state
	dep2.state = "Happy"
	topic.Publish(dep2)

	// Wait for the state change to be processed
	time.Sleep(50 * time.Millisecond)

	// Assert: Both dependencies should be met
	if !ds.IsDependencyMet(dep1.id) {
		t.Errorf("Expected dependency %s to still be met", dep1.id)
	}
	if !ds.IsDependencyMet(dep2.id) {
		t.Errorf("Expected dependency %s to be met after update", dep2.id)
	}

	// Act: Check a non-existent dependency
	if ds.IsDependencyMet("non-existent") {
		t.Errorf("Expected non-existent dependency to not be met")
	}
}

// TestWaitForAny tests the WaitForAny method.
func TestWaitForAny(t *testing.T) {
	// Arrange: Set up the test
	ctx, dep1, dep2, topic, ds := setupTest()
	ds.Add(dep1, dep2)

	// Act & Assert: Initially, no dependencies are met, so WaitForAny should timeout
	id, err := ds.WaitForAny(ctx, []string{dep1.id, dep2.id}, 50*time.Millisecond)
	if err == nil {
		t.Errorf("Expected WaitForAny to timeout, but it returned %s", id)
	}

	// Act: Update the first dependency to the desired state in a goroutine
	go func() {
		time.Sleep(50 * time.Millisecond)
		dep1.state = "Happy"
		topic.Publish(dep1)
	}()

	// Act & Assert: WaitForAny should return the first dependency
	id, err = ds.WaitForAny(ctx, []string{dep1.id, dep2.id}, 200*time.Millisecond)
	if err != nil {
		t.Errorf("Expected WaitForAny to succeed, but it returned error: %v", err)
	}
	if id != dep1.id {
		t.Errorf("Expected WaitForAny to return %s, but it returned %s", dep1.id, id)
	}

	// Act: Update the second dependency to the desired state
	dep2.state = "Happy"
	topic.Publish(dep2)

	// Wait for the state change to be processed
	time.Sleep(50 * time.Millisecond)

	// Act & Assert: WaitForAny should return immediately with the first dependency in the list
	id, err = ds.WaitForAny(ctx, []string{dep1.id, dep2.id}, 50*time.Millisecond)
	if err != nil {
		t.Errorf("Expected WaitForAny to succeed, but it returned error: %v", err)
	}
	if id != dep1.id {
		t.Errorf("Expected WaitForAny to return %s, but it returned %s", dep1.id, id)
	}

	// Act & Assert: WaitForAny should return immediately with the second dependency if it's first in the list
	id, err = ds.WaitForAny(ctx, []string{dep2.id, dep1.id}, 50*time.Millisecond)
	if err != nil {
		t.Errorf("Expected WaitForAny to succeed, but it returned error: %v", err)
	}
	if id != dep2.id {
		t.Errorf("Expected WaitForAny to return %s, but it returned %s", dep2.id, id)
	}

	// Act & Assert: WaitForAny should return an error for an empty list
	id, err = ds.WaitForAny(ctx, []string{}, 50*time.Millisecond)
	if err == nil {
		t.Errorf("Expected WaitForAny to return an error for an empty list, but it returned %s", id)
	}

	// Act & Assert: WaitForAny should return an error for a list with non-existent dependencies
	id, err = ds.WaitForAny(ctx, []string{"non-existent"}, 50*time.Millisecond)
	if err == nil {
		t.Errorf("Expected WaitForAny to return an error for non-existent dependencies, but it returned %s", id)
	}
}

// TestNewDependencyStateWithTopic tests the NewDependencyStateWithTopic function.
func TestNewDependencyStateWithTopic(t *testing.T) {
	// Arrange: Create a topic and dependencies
	topic := pubsub.NewTopic()
	dep1 := testDep{id: "1", state: "Sad"}
	dep2 := testDep{id: "2", state: "Sad"}

	// Act: Create a dependency state with the topic
	ctx := context.Background()
	ds, depChan := NewDependencyStateWithTopic(ctx, testIDStateFunc, State("Happy"), topic)

	// Assert: The dependency state should be created successfully
	if ds == nil {
		t.Errorf("Expected NewDependencyStateWithTopic to return a non-nil DependencyState")
	}
	if depChan == nil {
		t.Errorf("Expected NewDependencyStateWithTopic to return a non-nil channel")
	}

	// Act: Add dependencies and publish their initial state
	ds.Add(dep1, dep2)
	topic.Publish(dep1, dep2)

	// Assert: Verify the initial state is DependenciesNotMet
	assertStateEquals(t, ds, DependenciesNotMet, 500*time.Millisecond)
}

// TestNewDependencyStateWithBuffer tests the NewDependencyStateWithBuffer function.
func TestNewDependencyStateWithBuffer(t *testing.T) {
	// Arrange: Create a topic and dependencies
	topic := pubsub.NewTopic()
	dep1 := testDep{id: "1", state: "Sad"}
	dep2 := testDep{id: "2", state: "Sad"}

	// Act: Create a dependency state with the topic and a buffer
	ctx := context.Background()
	ds, depChan := NewDependencyStateWithBuffer(ctx, testIDStateFunc, State("Happy"), topic, 10)

	// Assert: The dependency state should be created successfully
	if ds == nil {
		t.Errorf("Expected NewDependencyStateWithBuffer to return a non-nil DependencyState")
	}
	if depChan == nil {
		t.Errorf("Expected NewDependencyStateWithBuffer to return a non-nil channel")
	}

	// Act: Add dependencies and publish their initial state
	ds.Add(dep1, dep2)
	topic.Publish(dep1, dep2)

	// Assert: Verify the initial state is DependenciesNotMet
	assertStateEquals(t, ds, DependenciesNotMet, 500*time.Millisecond)
}
