package main

import (
	"fmt"
	"io"
)

func printHelp(stdout io.Writer, topic string) {
	switch topic {
	case "":
		printGeneralHelp(stdout)
	case "loop":
		printLoopHelp(stdout)
	case "loop render":
		printLoopRenderHelp(stdout)
	case "loop validate":
		printLoopValidateHelp(stdout)
	case "loop pull":
		printLoopPullHelp(stdout)
	case "loop push":
		printLoopPushHelp(stdout)
	case "skill":
		printSkillHelp(stdout)
	case "skill pull":
		printSkillPullHelp(stdout)
	case "skill validate":
		printSkillValidateHelp(stdout)
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
	fmt.Fprintln(stdout, "Agent toolkit for OCI-backed loops and skills.")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Usage:")
	fmt.Fprintln(stdout, "  agentkit [command]")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Loop Commands:")
	fmt.Fprintln(stdout, "  loop push                 Push an Agent Loop YAML artifact")
	fmt.Fprintln(stdout, "  loop pull                 Pull a loop artifact or collection")
	fmt.Fprintln(stdout, "  loop render               Display a loop as a terminal flowchart")
	fmt.Fprintln(stdout, "  loop validate             Validate a local Agent Loop YAML file")
	fmt.Fprintln(stdout, "  loop collection push      Push a loop collection index")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Skill Commands:")
	fmt.Fprintln(stdout, "  skill push                Push an Agent Skill directory")
	fmt.Fprintln(stdout, "  skill pull                Pull a skill artifact or collection")
	fmt.Fprintln(stdout, "  skill validate            Validate a local Agent Skill directory")
	fmt.Fprintln(stdout, "  skill collection push     Push a skill collection index")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Agent Commands:")
	fmt.Fprintln(stdout, "  quickstart                Show a human-oriented getting started guide")
	fmt.Fprintln(stdout, "  prime                     Show AI-optimized workflow context for agents")
	fmt.Fprintln(stdout, "  init                      Add agentkit instructions to AGENTS.md")
	fmt.Fprintln(stdout, "  help                      Help about any command")
}

func printLoopHelp(stdout io.Writer) {
	fmt.Fprintln(stdout, "Manage Agent Loop OCI artifacts.")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Usage:")
	fmt.Fprintln(stdout, "  agentkit loop <push|pull|render|validate|collection>")
}

func printLoopPushHelp(stdout io.Writer) {
	fmt.Fprintln(stdout, "Push an Agent Loop YAML file as an OCI artifact.")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Usage:")
	fmt.Fprintln(stdout, "  agentkit loop push <loop.yml> <registry/namespace/package:tag>")
}

func printLoopPullHelp(stdout io.Writer) {
	fmt.Fprintln(stdout, "Pull an Agent Loop artifact or loop collection into the agentkit root.")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Usage:")
	fmt.Fprintln(stdout, "  agentkit loop pull [flags] <registry/namespace/package:tag>")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Flags:")
	fmt.Fprintln(stdout, "  -agents-dir string   agentkit root (default \".agents\")")
}

func printLoopRenderHelp(stdout io.Writer) {
	fmt.Fprintln(stdout, "Display a loop as a terminal flowchart.")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Usage:")
	fmt.Fprintln(stdout, "  agentkit loop render [flags] <local.yml|registry/namespace/package:tag>")
}

func printLoopValidateHelp(stdout io.Writer) {
	fmt.Fprintln(stdout, "Validate a local Agent Loop YAML file.")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Usage:")
	fmt.Fprintln(stdout, "  agentkit loop validate <loop.yml>")
}

func printSkillHelp(stdout io.Writer) {
	fmt.Fprintln(stdout, "Manage Agent Skill OCI artifacts.")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Usage:")
	fmt.Fprintln(stdout, "  agentkit skill <push|pull|validate|collection>")
}

func printSkillPullHelp(stdout io.Writer) {
	fmt.Fprintln(stdout, "Pull an Agent Skill artifact or skill collection into the agentkit root.")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Usage:")
	fmt.Fprintln(stdout, "  agentkit skill pull [flags] <registry/namespace/package:tag>")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Flags:")
	fmt.Fprintln(stdout, "  -agents-dir string   agentkit root (default \".agents\")")
}

func printSkillValidateHelp(stdout io.Writer) {
	fmt.Fprintln(stdout, "Validate a local Agent Skill directory.")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Usage:")
	fmt.Fprintln(stdout, "  agentkit skill validate <skill-dir>")
}

func printInitHelp(stdout io.Writer) {
	fmt.Fprintln(stdout, "Add agentkit agent instructions to AGENTS.md.")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Usage:")
	fmt.Fprintln(stdout, "  agentkit init [flags]")
}

func printQuickstartHelp(stdout io.Writer) {
	fmt.Fprintln(stdout, "Show a human-oriented getting started guide.")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Usage:")
	fmt.Fprintln(stdout, "  agentkit quickstart")
}

func printPrimeHelp(stdout io.Writer) {
	fmt.Fprintln(stdout, "Show AI-optimized workflow context for agents.")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Usage:")
	fmt.Fprintln(stdout, "  agentkit prime")
}

func printHelpHelp(stdout io.Writer) {
	fmt.Fprintln(stdout, "Help about any command.")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Usage:")
	fmt.Fprintln(stdout, "  agentkit help [topic]")
}
