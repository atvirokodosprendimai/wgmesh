package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/atvirokodosprendimai/wgmesh/pkg/pilot"
)

// pilotCmd dispatches pilot subcommands.
func pilotCmd() {
	if len(os.Args) < 3 {
		printPilotUsage()
		os.Exit(1)
	}

	action := os.Args[2]

	switch action {
	case "init":
		pilotInitCmd()
	case "status":
		pilotStatusCmd()
	case "report":
		pilotReportCmd()
	case "milestone":
		pilotMilestoneCmd()
	case "validate":
		pilotValidateCmd()
	case "--help", "-h":
		printPilotUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown pilot action: %s\n", action)
		printPilotUsage()
		os.Exit(1)
	}
}

func printPilotUsage() {
	fmt.Println(`wgmesh pilot - 30-day evaluation framework for network administrators

USAGE:
  wgmesh pilot <command> [options]

COMMANDS:
  init                 Initialize pilot tracking
  status               Show current pilot day, milestones, and health summary
  report               Generate evaluation report (Markdown or JSON)
  milestone <name>     Mark a milestone as completed
  validate             Run automated health checks across the mesh

OPTIONS (init):
  --mode <mode>          Deployment mode: centralized or decentralized (default: decentralized)
  --use-case <case>      Use case: hybrid-site-to-site, multi-cloud, remote-team, managed-fleet, general
  --nodes <count>        Initial node count (default: 1)
  --platforms <list>     Platform breakdown (e.g. "linux:3,darwin:1")
  --state <path>         Pilot state file path (default: ~/.wgmesh/pilot.json)

OPTIONS (status):
  --state <path>         Pilot state file path

OPTIONS (report):
  --format <fmt>         Output format: markdown or json (default: markdown)
  --state <path>         Pilot state file path

OPTIONS (milestone):
  --state <path>         Pilot state file path

OPTIONS (validate):
  --state <path>         Pilot state file path

EXAMPLES:
  # Initialize a new pilot evaluation
  wgmesh pilot init --mode decentralized --use-case remote-team --nodes 3

  # Check pilot status
  wgmesh pilot status

  # Run health checks
  wgmesh pilot validate

  # Mark a milestone complete
  wgmesh pilot milestone mesh-bootstrap

  # Generate a Markdown report
  wgmesh pilot report --format markdown

  # Generate a JSON report
  wgmesh pilot report --format json`)
}

func pilotInitCmd() {
	fs := flag.NewFlagSet("pilot init", flag.ExitOnError)
	mode := fs.String("mode", string(pilot.ModeDecentralized), "Deployment mode: centralized or decentralized")
	useCase := fs.String("use-case", string(pilot.UseCaseGeneral), "Use case category")
	nodes := fs.Int("nodes", 1, "Initial node count")
	platforms := fs.String("platforms", "", "Platform breakdown (e.g. 'linux:3,darwin:1')")
	statePath := fs.String("state", pilot.DefaultPilotPath(), "Pilot state file path")
	fs.Parse(os.Args[3:])

	// Validate deployment mode
	deploymentMode := pilot.DeploymentMode(*mode)
	switch deploymentMode {
	case pilot.ModeCentralized, pilot.ModeDecentralized:
	default:
		fmt.Fprintf(os.Stderr, "Error: invalid deployment mode %q (use 'centralized' or 'decentralized')\n", *mode)
		os.Exit(1)
	}

	// Validate use case
	uc := pilot.UseCase(*useCase)
	switch uc {
	case pilot.UseCaseHybridSiteToSite, pilot.UseCaseMultiCloud,
		pilot.UseCaseRemoteTeam, pilot.UseCaseManagedFleet, pilot.UseCaseGeneral:
	default:
		fmt.Fprintf(os.Stderr, "Error: invalid use case %q\n", *useCase)
		fmt.Fprintln(os.Stderr, "Valid options: hybrid-site-to-site, multi-cloud, remote-team, managed-fleet, general")
		os.Exit(1)
	}

	// Parse platform breakdown
	breakdown := make(map[string]int)
	if *platforms != "" {
		for _, entry := range strings.Split(*platforms, ",") {
			parts := strings.SplitN(strings.TrimSpace(entry), ":", 2)
			if len(parts) != 2 {
				fmt.Fprintf(os.Stderr, "Error: invalid platform entry %q (expected format 'platform:count')\n", entry)
				os.Exit(1)
			}
			var count int
			if _, err := fmt.Sscanf(parts[1], "%d", &count); err != nil || count < 1 {
				fmt.Fprintf(os.Stderr, "Error: invalid count %q in platform entry\n", parts[1])
				os.Exit(1)
			}
			breakdown[parts[0]] = count
		}
	}

	state := pilot.NewPilotState(deploymentMode, uc, *nodes)
	state.PlatformBreakdown = breakdown

	if err := state.Save(*statePath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to save pilot state: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Pilot evaluation initialized")
	fmt.Printf("  Mode:      %s\n", state.Mode)
	fmt.Printf("  Use case:  %s\n", state.UseCase)
	fmt.Printf("  Nodes:     %d\n", state.InitialNodeCount)
	fmt.Printf("  Start:     %s\n", state.StartDate.Format("2006-01-02"))
	fmt.Printf("  State:     %s\n", *statePath)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Deploy wgmesh on all nodes (wgmesh join --secret <SECRET>)")
	fmt.Println("  2. Run 'wgmesh pilot validate' to check mesh health")
	fmt.Println("  3. Mark milestones as you complete them:")
	fmt.Println("     wgmesh pilot milestone mesh-bootstrap")
}

func pilotStatusCmd() {
	fs := flag.NewFlagSet("pilot status", flag.ExitOnError)
	statePath := fs.String("state", pilot.DefaultPilotPath(), "Pilot state file path")
	fs.Parse(os.Args[3:])

	state, err := pilot.LoadState(*statePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintln(os.Stderr, "Run 'wgmesh pilot init' to start a pilot evaluation.")
		os.Exit(1)
	}

	day := state.CurrentDay()
	week := state.CurrentWeek()
	theme := pilot.WeekTheme(week)

	fmt.Printf("Pilot Evaluation Status\n")
	fmt.Printf("=======================\n\n")
	fmt.Printf("Day:          %d / 30 (Week %d)\n", day, week)
	fmt.Printf("Week Theme:   %s\n", theme)
	fmt.Printf("Mode:         %s\n", state.Mode)
	fmt.Printf("Use Case:     %s\n", state.UseCase)
	fmt.Printf("Start Date:   %s\n", state.StartDate.Format("2006-01-02"))
	fmt.Printf("Nodes:        %d\n", state.InitialNodeCount)
	fmt.Println()

	// Current week milestones
	fmt.Printf("Week %d Milestones:\n", week)
	completed, total := pilot.WeekProgress(state, week)
	for _, name := range pilot.WeekMilestones(week) {
		mark := "[ ]"
		if state.MilestoneCompleted(name) {
			mark = "[x]"
		}
		fmt.Printf("  %s %s\n", mark, name)
	}
	fmt.Printf("  Progress: %d / %d\n\n", completed, total)

	// Overall progress across all weeks
	allCompleted := 0
	allTotal := 0
	for w := 1; w <= 4; w++ {
		c, t := pilot.WeekProgress(state, w)
		allCompleted += c
		allTotal += t
	}
	fmt.Printf("Overall Progress: %d / %d milestones\n\n", allCompleted, allTotal)

	// Last health check
	last := state.LastHealthResult()
	if last != nil {
		fmt.Printf("Last Health Check: %s\n", last.Timestamp.Format("2006-01-02 15:04:05"))
		fmt.Printf("  Status: %s  (Pass: %d, Fail: %d, Warn: %d)\n",
			last.Status(), last.PassCount, last.FailCount, last.WarnCount)
		for _, check := range last.Checks {
			icon := "✓"
			if check.Status == pilot.HealthFail {
				icon = "✗"
			} else if check.Status == pilot.HealthWarn {
				icon = "!"
			}
			fmt.Printf("  %s %s: %s\n", icon, check.Name, check.Message)
		}
	} else {
		fmt.Println("No health checks recorded. Run 'wgmesh pilot validate' to start.")
	}

	// Issues count
	if len(state.Issues) > 0 {
		fmt.Printf("\nIssues: %d recorded\n", len(state.Issues))
	}
}

func pilotReportCmd() {
	fs := flag.NewFlagSet("pilot report", flag.ExitOnError)
	format := fs.String("format", string(pilot.ReportMarkdown), "Output format: markdown or json")
	statePath := fs.String("state", pilot.DefaultPilotPath(), "Pilot state file path")
	fs.Parse(os.Args[3:])

	state, err := pilot.LoadState(*statePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintln(os.Stderr, "Run 'wgmesh pilot init' to start a pilot evaluation.")
		os.Exit(1)
	}

	reportFormat := pilot.ReportFormat(*format)
	report, err := pilot.GenerateReport(state, reportFormat)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating report: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(report)
}

func pilotMilestoneCmd() {
	fs := flag.NewFlagSet("pilot milestone", flag.ExitOnError)
	statePath := fs.String("state", pilot.DefaultPilotPath(), "Pilot state file path")
	fs.Parse(os.Args[3:])

	args := fs.Args()
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: wgmesh pilot milestone <name>")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Available milestones:")
		for w := 1; w <= 4; w++ {
			fmt.Fprintf(os.Stderr, "  Week %d (%s):\n", w, pilot.WeekTheme(w))
			for _, name := range pilot.WeekMilestones(w) {
				fmt.Fprintf(os.Stderr, "    %s\n", name)
			}
		}
		os.Exit(1)
	}

	name := args[0]

	state, err := pilot.LoadState(*statePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintln(os.Stderr, "Run 'wgmesh pilot init' to start a pilot evaluation.")
		os.Exit(1)
	}

	if state.MilestoneCompleted(name) {
		fmt.Printf("Milestone '%s' already completed at %s\n", name, state.Milestones[name].Format(time.RFC1123))
		return
	}

	state.CompleteMilestone(name)
	if err := state.Save(*statePath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to save pilot state: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Milestone completed: %s\n", name)
}

func pilotValidateCmd() {
	fs := flag.NewFlagSet("pilot validate", flag.ExitOnError)
	statePath := fs.String("state", pilot.DefaultPilotPath(), "Pilot state file path")
	fs.Parse(os.Args[3:])

	// Load state if it exists (pilot init is optional for running health checks)
	var state *pilot.PilotState
	state, err := pilot.LoadState(*statePath)
	if err != nil {
		// No pilot state — run health checks anyway and report
		state = nil
	}

	fmt.Println("Running health checks...")
	fmt.Println()

	result := pilot.RunHealthChecks()

	// Print results
	fmt.Printf("Health Check Results (%s)\n", result.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Println(strings.Repeat("=", 60))

	for _, check := range result.Checks {
		icon := "✓ PASS"
		if check.Status == pilot.HealthFail {
			icon = "✗ FAIL"
		} else if check.Status == pilot.HealthWarn {
			icon = "! WARN"
		}
		fmt.Printf("  %s  %s\n", icon, check.Name)
		if check.Message != "" {
			fmt.Printf("         %s\n", check.Message)
		}
	}

	fmt.Println()
	fmt.Printf("Summary: %d pass, %d fail, %d warn\n", result.PassCount, result.FailCount, result.WarnCount)

	// Save to pilot state if available
	if state != nil {
		state.AddHealthResult(result)
		if err := state.Save(*statePath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to save health check result: %v\n", err)
		}
	}

	// Exit code: 0 on pass/warn, 1 on fail
	if result.FailCount > 0 {
		os.Exit(1)
	}
}

// suppress unused import warnings for packages used in pilot commands
var _ = json.Marshal
var _ = time.RFC1123
