package main

import (
	"fmt"
	"io"
)

func printHelp(stdout io.Writer, topic string) {
	switch topic {
	case "":
		printGeneralHelp(stdout)
	case "push":
		printPushHelp(stdout)
	case "pull":
		printPullHelp(stdout)
	case "run":
		printRunHelp(stdout)
	case "render":
		printRenderHelp(stdout)
	case "validate":
		printValidateHelp(stdout)
	case "init":
		printInitHelp(stdout)
	case "quickstart":
		printQuickstartHelp(stdout)
	case "prime":
		printPrimeHelp(stdout)
	case "help":
		printHelpHelp(stdout)
	default:
		fmt.Fprintf(stdout, "Unknown help topic %q.\n\n", topic)
		printGeneralHelp(stdout)
	}
}

func printGeneralHelp(stdout io.Writer) {
	fmt.Fprintln(stdout, "OCI-backed YAML loop packages.")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Usage:")
	fmt.Fprintln(stdout, "  loop_cli [command]")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Package Commands:")
	fmt.Fprintln(stdout, "  push        Package a YAML file and upload it as an OCI artifact")
	fmt.Fprintln(stdout, "  pull        Pull a loop package into the local cache")
	fmt.Fprintln(stdout, "  run         Print the loop YAML, pulling it first when needed")
	fmt.Fprintln(stdout, "  render      Display a loop as a terminal flowchart")
	fmt.Fprintln(stdout, "  validate    Validate a local loop YAML file")
	fmt.Fprintln(stdout, "  init        Add loop_cli agent instructions to AGENTS.md")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Agent Commands:")
	fmt.Fprintln(stdout, "  quickstart  Show a human-oriented getting started guide")
	fmt.Fprintln(stdout, "  prime       Show AI-optimized workflow context for agents")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Additional Commands:")
	fmt.Fprintln(stdout, "  help        Help about any command")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Agent Workflow:")
	fmt.Fprintln(stdout, "  Agents should run `loop_cli prime` before running a loop package.")
	fmt.Fprintln(stdout, "  It explains registry login, `spec.inputs`, phase sub-agents, and run artifacts.")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Use \"loop_cli help [command]\" for more information about a command.")
}

func printPushHelp(stdout io.Writer) {
	fmt.Fprintln(stdout, "Package a YAML file and upload it as an OCI artifact.")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Usage:")
	fmt.Fprintln(stdout, "  loop_cli push [flags] <local.yml> <registry/namespace/package_name:tag>")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Examples:")
	fmt.Fprintln(stdout, "  loop_cli push ./loop.yml ghcr.io/owner/repo/package:0.1.0")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Flags:")
	fmt.Fprintln(stdout, "  -artifact-type string      OCI artifact type for the package manifest")
	fmt.Fprintln(stdout, "  -layer-media-type string   OCI media type for the YAML layer")
}

func printPullHelp(stdout io.Writer) {
	fmt.Fprintln(stdout, "Pull a loop package into the local cache.")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Usage:")
	fmt.Fprintln(stdout, "  loop_cli pull [flags] <registry/namespace/package_name:tag>")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Flags:")
	fmt.Fprintln(stdout, "  -layer-media-type string   OCI media type for the YAML layer")
	fmt.Fprintln(stdout, "  -output string             also copy pulled YAML to a file")
}

func printRunHelp(stdout io.Writer) {
	fmt.Fprintln(stdout, "Print the loop YAML, pulling it first when needed.")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Usage:")
	fmt.Fprintln(stdout, "  loop_cli run [flags] <registry/namespace/package_name:tag>")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Agent Note:")
	fmt.Fprintln(stdout, "  Run `loop_cli prime` first when an agent is asked to execute a loop.")
	fmt.Fprintln(stdout, "  The prime context explains how to collect `spec.inputs` and orchestrate phases.")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Flags:")
	fmt.Fprintln(stdout, "  -layer-media-type string   OCI media type for the YAML layer")
}

func printRenderHelp(stdout io.Writer) {
	fmt.Fprintln(stdout, "Display a loop as a terminal flowchart.")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Usage:")
	fmt.Fprintln(stdout, "  loop_cli render [flags] <local.yml|registry/namespace/package_name:tag>")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "The command shows phases, transitions, self-loops, and root escalation inputs.")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Flags:")
	fmt.Fprintln(stdout, "  -details                  include compact phase actions, completion, and output summaries")
	fmt.Fprintln(stdout, "  -layer-media-type string   OCI media type for the YAML layer")
	fmt.Fprintln(stdout, "  -no-color                 disable ANSI color output")
}

func printValidateHelp(stdout io.Writer) {
	fmt.Fprintln(stdout, "Validate a local loop YAML file.")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Usage:")
	fmt.Fprintln(stdout, "  loop_cli validate <local.yml>")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "The command checks YAML syntax, duplicate mapping keys, and the embedded loop JSON Schema.")
}

func printInitHelp(stdout io.Writer) {
	fmt.Fprintln(stdout, "Add loop_cli agent instructions to AGENTS.md.")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Usage:")
	fmt.Fprintln(stdout, "  loop_cli init [flags]")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "The command creates or updates a managed AGENTS.md block that points agents to `loop_cli prime`.")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Flags:")
	fmt.Fprintln(stdout, "  -agents-file string   path to the agent instruction file to update (default \"AGENTS.md\")")
}

func printQuickstartHelp(stdout io.Writer) {
	fmt.Fprintln(stdout, "Show a human-oriented getting started guide.")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Usage:")
	fmt.Fprintln(stdout, "  loop_cli quickstart")
}

func printPrimeHelp(stdout io.Writer) {
	fmt.Fprintln(stdout, "Show AI-optimized workflow context for agents.")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Usage:")
	fmt.Fprintln(stdout, "  loop_cli prime")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Use this before an agent runs a loop package. It covers authentication, `spec.inputs`, sub-agent phase execution, and `.loop/runs` artifacts.")
}

func printHelpHelp(stdout io.Writer) {
	fmt.Fprintln(stdout, "Help about any command.")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Usage:")
	fmt.Fprintln(stdout, "  loop_cli help [command]")
}
