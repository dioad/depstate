package depstate

import (
	"context"
	"testing"
	"time"

	"github.com/dioad/pubsub"
)

// benchDep is a simple dependency type for benchmarking
type benchDep struct {
	id    string
	state string
}

// benchIDStateFunc extracts the ID and state from a benchDep
func benchIDStateFunc(d benchDep) (string, State) {
	return d.id, State(d.state)
}

// setupBenchmark creates a common benchmark setup
func setupBenchmark(b *testing.B, numDeps int) (context.Context, []benchDep, pubsub.Topic, DependencyState[benchDep]) {
	b.Helper()
	// Create dependencies
	deps := make([]benchDep, numDeps)
	for i := 0; i < numDeps; i++ {
		deps[i] = benchDep{
			id:    string(rune('A' + i)),
			state: "Sad",
		}
	}

	// Create a topic with buffered channel
	topic := pubsub.NewTopic()
	topicChan := topic.SubscribeWithBuffer(numDeps * 2)

	// Create context
	ctx := context.Background()

	// Create the dependency state tracker
	ds, _ := NewDependencyState(ctx, benchIDStateFunc, State("Happy"), topicChan)

	return ctx, deps, topic, ds
}

// BenchmarkAdd measures the performance of adding dependencies
func BenchmarkAdd(b *testing.B) {
	ctx, deps, _, ds := setupBenchmark(b, 10)
	defer ctx.Done()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ds.Add(deps...)
	}
}

// BenchmarkSet measures the performance of setting dependency states
func BenchmarkSet(b *testing.B) {
	ctx, deps, _, ds := setupBenchmark(b, 10)
	defer ctx.Done()

	// Add dependencies first
	ds.Add(deps...)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, dep := range deps {
			ds.Set(dep.id, State("Happy"))
		}
	}
}

// BenchmarkCurrentState measures the performance of getting the current state
func BenchmarkCurrentState(b *testing.B) {
	ctx, deps, _, ds := setupBenchmark(b, 10)
	defer ctx.Done()

	// Add dependencies first
	ds.Add(deps...)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ds.CurrentState()
	}
}

// BenchmarkGetDependencyStates measures the performance of getting all dependency states
func BenchmarkGetDependencyStates(b *testing.B) {
	ctx, deps, _, ds := setupBenchmark(b, 10)
	defer ctx.Done()

	// Add dependencies first
	ds.Add(deps...)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ds.GetDependencyStates()
	}
}

// BenchmarkIsDependencyMet measures the performance of checking if a dependency is met
func BenchmarkIsDependencyMet(b *testing.B) {
	ctx, deps, _, ds := setupBenchmark(b, 10)
	defer ctx.Done()

	// Add dependencies first
	ds.Add(deps...)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ds.IsDependencyMet(deps[0].id)
	}
}

// BenchmarkStateChange measures the performance of state changes
func BenchmarkStateChange(b *testing.B) {
	ctx, deps, topic, ds := setupBenchmark(b, 10)
	defer ctx.Done()

	// Add dependencies first
	ds.Add(deps...)

	// Create a channel to receive state changes
	stateChan := ds.Chan()
	go func() {
		for range stateChan {
			// Just consume the messages
		}
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Change all dependencies to Happy
		for j := range deps {
			deps[j].state = "Happy"
			topic.Publish(deps[j])
		}

		// Change all dependencies back to Sad
		for j := range deps {
			deps[j].state = "Sad"
			topic.Publish(deps[j])
		}
	}
}

// BenchmarkWaitForDependencies measures the performance of waiting for dependencies
func BenchmarkWaitForDependencies(b *testing.B) {
	ctx, deps, topic, ds := setupBenchmark(b, 3)
	defer ctx.Done()

	// Add dependencies first
	ds.Add(deps...)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Set up a goroutine to make all dependencies happy after a short delay
		go func() {
			time.Sleep(1 * time.Millisecond)
			for j := range deps {
				deps[j].state = "Happy"
				topic.Publish(deps[j])
			}
		}()

		// Wait for dependencies to be met
		ds.WaitForDependencies(ctx, 100*time.Millisecond)

		// Reset dependencies to Sad for the next iteration
		for j := range deps {
			deps[j].state = "Sad"
			topic.Publish(deps[j])
		}
		time.Sleep(1 * time.Millisecond) // Give time for the update to be processed
	}
}

// BenchmarkWaitForAny measures the performance of waiting for any dependency
func BenchmarkWaitForAny(b *testing.B) {
	ctx, deps, topic, ds := setupBenchmark(b, 3)
	defer ctx.Done()

	// Add dependencies first
	ds.Add(deps...)

	// Create a list of dependency IDs
	ids := make([]string, len(deps))
	for i, dep := range deps {
		ids[i] = dep.id
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Set up a goroutine to make one dependency happy after a short delay
		go func() {
			time.Sleep(1 * time.Millisecond)
			deps[0].state = "Happy"
			topic.Publish(deps[0])
		}()

		// Wait for any dependency to be met
		_, _ = ds.WaitForAny(ctx, ids, 100*time.Millisecond)

		// Reset dependencies to Sad for the next iteration
		deps[0].state = "Sad"
		topic.Publish(deps[0])
		time.Sleep(1 * time.Millisecond) // Give time for the update to be processed
	}
}
