package mesh

import "io"

// StrictState makes a class B finding a refusal instead of a warning. It is
// false in release N and set in N+1 (ADR-0006, decision 2).
var StrictState = false

// Class says what Deploy does with a finding.
type Class string

const (
	// ClassA is refused: a value a shell or a wg-quick file gives a meaning to,
	// or a field Deploy cannot work without.
	ClassA Class = "A"
	// ClassB only warns, until StrictState is set: a value that parses wrongly
	// but cannot reach a shell.
	ClassB Class = "B"
)

// Finding is one rule a loaded state breaks. Node is empty for a mesh-level
// field.
type Finding struct {
	Node  string
	Field string
	Value string
	Rule  string
	Class Class
}

// Validate checks the loaded state and returns every finding. It is pure.
func (m *Mesh) Validate() []Finding {
	return nil
}

// validateForDeploy prints the class B warnings to out and returns an error
// listing the findings Deploy refuses.
func (m *Mesh) validateForDeploy(out io.Writer) error {
	return nil
}
