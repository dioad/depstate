package depstate

import (
	"context"
	"testing"

	"github.com/dioad/pubsub"
)

type testDep struct {
	id    string
	state string
}

func testIDStateFunc(t testDep) (string, State) {
	return t.id, State(t.state)
}

func TestNewDependencyState(t *testing.T) {
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
	ds.Add(testDepOne, testDepTwo)

	topic.Publish(testDepOne, testDepTwo)
	s := <-depChan
	if s != DependenciesNotMet {
		t.Errorf("Expected state to be %v, got %v", DependenciesNotMet, ds.CurrentState())
	}

	testDepOne.state = "Happy"
	topic.Publish(testDepOne)
	// No state change should be published
	if ds.CurrentState() != DependenciesNotMet {
		t.Errorf("Expected state to be %v, got %v", DependenciesNotMet, ds.CurrentState())
	}

	testDepTwo.state = "Happy"
	topic.Publish(testDepTwo)
	s = <-depChan
	if s != DependenciesMet {
		t.Errorf("Expected state to be %v, got %v", DependenciesMet, s)
	}

	testDepTwo.state = "Sad"
	topic.Publish(testDepTwo)
	s = <-depChan
	if s != DependenciesNotMet {
		t.Errorf("Expected state to be %v, got %v", DependenciesNotMet, ds.CurrentState())
	}

	testDepTwo.state = "Happy"
	topic.Publish(testDepTwo)
	s = <-depChan
	if s != DependenciesMet {
		t.Errorf("Expected state to be %v, got %v", DependenciesMet, s)
	}
}
