// Package depstate provides a way to track the state of dependencies and emit events when all dependencies are in a desired state.
package depstate

// State represents the state of dependencies.
type State string

const (
	// DependenciesMet indicates that all dependencies are in the desired state.
	DependenciesMet State = "dependencies_met"
	// DependenciesNotMet indicates that at least one dependency is not in the desired state.
	DependenciesNotMet State = "dependencies_notmet"
	// DependenciesUnknown indicates that the state of dependencies is unknown.
	DependenciesUnknown State = "dependencies_unknown"
)
