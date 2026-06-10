package pilot

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ReportFormat specifies the output format for the pilot report.
type ReportFormat string

const (
	ReportMarkdown ReportFormat = "markdown"
	ReportJSON     ReportFormat = "json"
)

// GenerateReport creates a pilot evaluation report in the requested format.
func GenerateReport(state *PilotState, format ReportFormat) (string, error) {
	switch format {
	case ReportJSON:
		return generateJSONReport(state)
	case ReportMarkdown:
		return generateMarkdownReport(state), nil
	default:
		return "", fmt.Errorf("unsupported report format: %s", format)
	}
}

// jsonReport is the structured representation for JSON output.
type jsonReport struct {
	GeneratedAt       string             `json:"generated_at"`
	PilotDay          int                `json:"pilot_day"`
	PilotWeek         int                `json:"pilot_week"`
	DeploymentMode    string             `json:"deployment_mode"`
	UseCase           string             `json:"use_case"`
	InitialNodeCount  int                `json:"initial_node_count"`
	PlatformBreakdown map[string]int     `json:"platform_breakdown,omitempty"`
	Milestones        []milestoneJSON    `json:"milestones"`
	WeekProgress      []weekProgressJSON `json:"week_progress"`
	HealthSummary     healthSummaryJSON  `json:"health_summary"`
	Issues            []issueJSON        `json:"issues,omitempty"`
}

type milestoneJSON struct {
	Name        string `json:"name"`
	CompletedAt string `json:"completed_at,omitempty"`
	Status      string `json:"status"`
}

type weekProgressJSON struct {
	Week      int    `json:"week"`
	Theme     string `json:"theme"`
	Completed int    `json:"completed"`
	Total     int    `json:"total"`
}

type healthSummaryJSON struct {
	TotalChecks   int         `json:"total_checks"`
	PassCount     int         `json:"pass_count"`
	FailCount     int         `json:"fail_count"`
	WarnCount     int         `json:"warn_count"`
	LastCheckedAt string      `json:"last_checked_at,omitempty"`
	RecentResults []checkJSON `json:"recent_results,omitempty"`
}

type checkJSON struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type issueJSON struct {
	Description string `json:"description"`
	Resolution  string `json:"resolution,omitempty"`
	Timestamp   string `json:"timestamp"`
}

func generateJSONReport(state *PilotState) (string, error) {
	report := buildStructuredReport(state)
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling JSON report: %w", err)
	}
	return string(data), nil
}

func generateMarkdownReport(state *PilotState) string {
	var b strings.Builder
	day := state.CurrentDay()
	week := state.CurrentWeek()

	fmt.Fprintf(&b, "# wgmesh Pilot Evaluation Report\n\n")
	fmt.Fprintf(&b, "**Generated:** %s  \n", time.Now().Format(time.RFC1123))
	fmt.Fprintf(&b, "**Pilot Day:** %d / 30 (Week %d)  \n", day, week)
	fmt.Fprintf(&b, "**Deployment Mode:** %s  \n", state.Mode)
	fmt.Fprintf(&b, "**Use Case:** %s  \n", state.UseCase)
	fmt.Fprintf(&b, "**Initial Node Count:** %d  \n\n", state.InitialNodeCount)

	// Platform breakdown
	if len(state.PlatformBreakdown) > 0 {
		fmt.Fprintf(&b, "## Platform Breakdown\n\n")
		fmt.Fprintf(&b, "| Platform | Nodes |\n|---|---|\n")
		for platform, count := range state.PlatformBreakdown {
			fmt.Fprintf(&b, "| %s | %d |\n", platform, count)
		}
		fmt.Fprintln(&b)
	}

	// Milestone timeline
	fmt.Fprintf(&b, "## Milestone Timeline\n\n")
	for w := 1; w <= 4; w++ {
		completed, total := WeekProgress(state, w)
		theme := WeekTheme(w)
		fmt.Fprintf(&b, "### Week %d: %s (%d/%d)\n\n", w, theme, completed, total)
		for _, name := range WeekMilestones(w) {
			mark := "[ ]"
			if state.MilestoneCompleted(name) {
				mark = "[x]"
			}
			fmt.Fprintf(&b, "- %s %s\n", mark, name)
		}
		fmt.Fprintln(&b)
	}

	// Health check history
	fmt.Fprintf(&b, "## Health Check Summary\n\n")
	lastResult := state.LastHealthResult()
	if lastResult != nil {
		fmt.Fprintf(&b, "**Last Check:** %s  \n", lastResult.Timestamp.Format(time.RFC1123))
		fmt.Fprintf(&b, "**Overall Status:** %s  \n\n", lastResult.Status())
		fmt.Fprintf(&b, "| Check | Status | Details |\n|---|---|---|\n")
		for _, check := range lastResult.Checks {
			fmt.Fprintf(&b, "| %s | %s | %s |\n", check.Name, check.Status, check.Message)
		}
		fmt.Fprintln(&b)

		if len(state.HealthHistory) > 1 {
			fmt.Fprintf(&b, "### Health History\n\n")
			fmt.Fprintf(&b, "| Timestamp | Pass | Fail | Warn |\n|---|---|---|---|\n")
			// Show last 10 results, most recent first
			for i := len(state.HealthHistory) - 1; i >= 0; i-- {
				r := state.HealthHistory[i]
				fmt.Fprintf(&b, "| %s | %d | %d | %d |\n",
					r.Timestamp.Format(time.DateTime), r.PassCount, r.FailCount, r.WarnCount)
			}
			fmt.Fprintln(&b)
		}
	} else {
		fmt.Fprintf(&b, "No health checks have been run yet. Run `wgmesh pilot validate` to start.\n\n")
	}

	// Issues
	if len(state.Issues) > 0 {
		fmt.Fprintf(&b, "## Issues Encountered\n\n")
		for i, issue := range state.Issues {
			fmt.Fprintf(&b, "### %d. %s\n\n", i+1, issue.Description)
			fmt.Fprintf(&b, "- **Date:** %s\n", issue.Timestamp.Format(time.DateOnly))
			if issue.Resolution != "" {
				fmt.Fprintf(&b, "- **Resolution:** %s\n", issue.Resolution)
			}
			fmt.Fprintln(&b)
		}
	}

	// Pilot status
	if state.IsComplete() {
		fmt.Fprintf(&b, "## Pilot Complete\n\n")
		fmt.Fprintf(&b, "The 30-day pilot evaluation period has concluded.\n")
		fmt.Fprintf(&b, "Review the milestones and health check history above to assess readiness.\n")
	}

	return b.String()
}

func buildStructuredReport(state *PilotState) jsonReport {
	report := jsonReport{
		GeneratedAt:       time.Now().Format(time.RFC3339),
		PilotDay:          int(state.CurrentDay()),
		PilotWeek:         state.CurrentWeek(),
		DeploymentMode:    string(state.Mode),
		UseCase:           string(state.UseCase),
		InitialNodeCount:  state.InitialNodeCount,
		PlatformBreakdown: state.PlatformBreakdown,
	}

	// Milestones
	for w := 1; w <= 4; w++ {
		for _, name := range WeekMilestones(w) {
			msj := milestoneJSON{Name: name, Status: "pending"}
			if t, ok := state.Milestones[name]; ok && !t.IsZero() {
				msj.CompletedAt = t.Format(time.RFC3339)
				msj.Status = "completed"
			}
			report.Milestones = append(report.Milestones, msj)
		}
	}

	// Week progress
	for w := 1; w <= 4; w++ {
		completed, total := WeekProgress(state, w)
		report.WeekProgress = append(report.WeekProgress, weekProgressJSON{
			Week:      w,
			Theme:     WeekTheme(w),
			Completed: completed,
			Total:     total,
		})
	}

	// Health summary
	last := state.LastHealthResult()
	if last != nil {
		report.HealthSummary = healthSummaryJSON{
			TotalChecks:   len(last.Checks),
			PassCount:     last.PassCount,
			FailCount:     last.FailCount,
			WarnCount:     last.WarnCount,
			LastCheckedAt: last.Timestamp.Format(time.RFC3339),
		}
		for _, c := range last.Checks {
			report.HealthSummary.RecentResults = append(report.HealthSummary.RecentResults, checkJSON{
				Name:    c.Name,
				Status:  string(c.Status),
				Message: c.Message,
			})
		}
	}

	// Issues
	for _, issue := range state.Issues {
		report.Issues = append(report.Issues, issueJSON{
			Description: issue.Description,
			Resolution:  issue.Resolution,
			Timestamp:   issue.Timestamp.Format(time.RFC3339),
		})
	}

	return report
}
