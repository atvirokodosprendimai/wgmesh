package mesh

// Phase names the step of a node's deploy that failed.
type Phase string

const (
	PhaseDetect  Phase = "detect"
	PhaseConnect Phase = "connect"
	PhaseInstall Phase = "install"
	PhaseApply   Phase = "apply"
	PhaseRoutes  Phase = "routes"
	PhasePersist Phase = "persist"
)

// NodeResult is the outcome of one node: Err is nil when the node is
// configured and Phase is the step that failed otherwise.
type NodeResult struct {
	Hostname string
	Phase    Phase
	Err      error
	Warnings []string
}

// PersistFailureFatal makes a config file that cannot be written a failure of
// the node (ADR-0006 decision 6); false at release N, a warning in the result.
var PersistFailureFatal = false

// DeployError is what Deploy returns when at least one node failed.
type DeployError struct {
	Results []NodeResult
}

func (e *DeployError) Error() string { return "stub" }
