package mesh

import (
	"fmt"
	"strings"
)

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

// Error lists every failed node with its phase and error, one line each, and
// then the nodes that were configured.
func (e *DeployError) Error() string {
	var failed []string
	configured := []string{}
	for _, r := range e.Results {
		if r.Err == nil {
			configured = append(configured, r.Hostname)
			continue
		}
		failed = append(failed, fmt.Sprintf("  %s: %s: %s", r.Hostname, r.Phase, oneLine(r.Err.Error())))
	}
	list := "none"
	if len(configured) > 0 {
		list = strings.Join(configured, ", ")
	}
	return fmt.Sprintf("deploy failed on %d of %d nodes\n%s\nconfigured: %s",
		len(failed), len(e.Results), strings.Join(failed, "\n"), list)
}

// oneLine keeps an error text on one line of the report.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// deployResult is nil when every node succeeded, else a *DeployError.
func deployResult(results []NodeResult) error {
	for _, r := range results {
		if r.Err != nil {
			return &DeployError{Results: results}
		}
	}
	return nil
}
