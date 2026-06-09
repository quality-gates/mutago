package mutator

import "go/token"

// Mutation defines the behavior of one mutation
type Mutation struct {
	// Position is the source position of the original syntax changed by this
	// mutation. MutateWalkWithPositions falls back to the visited node position
	// when it is unset, preserving compatibility with third-party mutators.
	Position token.Pos
	// Change is called before executing the exec command.
	Change func()
	// Reset is called after executing the exec command.
	Reset func()
}
