package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
)

func run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) error {
	opts, err := parseOptions(args, stderr)
	if err != nil {
		return err
	}

	switch {
	case opts.command == "loop push":
		if err := validateLoopFile(opts.filename); err != nil {
			return err
		}
		result, err := pushPackage(ctx, opts)
		return printPushOutcome(stdout, opts, result, err)
	case opts.command == "loop collection push":
		result, err := publishCollection(ctx, opts)
		return printPushOutcome(stdout, opts, result, err)
	case opts.command == "loop pull":
		result, err := installReference(ctx, opts, loopArtifactType)
		if err != nil {
			return err
		}
		printInstallResult(stdout, result)
		return nil
	case opts.command == "loop render":
		return renderSource(ctx, opts, stdout)
	case opts.command == "loop validate":
		if err := validateLoopFile(opts.filename); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "valid %s\n", opts.filename)
		return nil
	case opts.command == "skill push":
		result, err := publishSkill(ctx, opts)
		return printPushOutcome(stdout, opts, result, err)
	case opts.command == "skill collection push":
		result, err := publishCollection(ctx, opts)
		return printPushOutcome(stdout, opts, result, err)
	case opts.command == "skill pull":
		result, err := installReference(ctx, opts, skillArtifactType)
		if err != nil {
			return err
		}
		printInstallResult(stdout, result)
		return nil
	case opts.command == "skill validate":
		if err := validateSkillDir(opts.filename); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "valid %s\n", opts.filename)
		return nil
	case opts.command == "init":
		action, err := updateAgentsFile(opts.agentsFile)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "%s %s\n", action, opts.agentsFile)
		return nil
	case opts.command == "quickstart":
		printQuickstart(stdout, opts)
		return nil
	case opts.command == "prime":
		printPrime(stdout, opts)
		return nil
	case opts.command == "help":
		printHelp(stdout, opts.helpTopic)
		return nil
	default:
		return fmt.Errorf("unknown command %q", opts.command)
	}
}

func printPushOutcome(stdout io.Writer, opts options, result pushResult, err error) error {
	if err != nil {
		return err
	}
	if result.skipped {
		fmt.Fprintf(stdout, "already up to date %s\n", opts.targetRef)
		fmt.Fprintf(stdout, "manifest digest: %s\n", result.digest)
		return nil
	}
	fmt.Fprintf(stdout, "pushed %s\n", opts.targetRef)
	fmt.Fprintf(stdout, "manifest digest: %s\n", result.digest)
	return nil
}

func parseOptions(args []string, stderr io.Writer) (options, error) {
	opts := options{agentsDir: defaultAgentsDir}
	if len(args) == 0 {
		printUsage(stderr)
		return options{}, errors.New("expected command")
	}
	if args[0] == "-h" || args[0] == "--help" {
		opts.command = "help"
		return opts, nil
	}
	switch args[0] {
	case "loop":
		return parseLoopOptions(args[1:], opts, stderr)
	case "skill":
		return parseSkillOptions(args[1:], opts, stderr)
	case "init":
		return parseInitOptions(args[1:], opts, stderr)
	case "quickstart":
		return parseQuickstartOptions(args[1:], opts, stderr)
	case "prime":
		return parsePrimeOptions(args[1:], opts, stderr)
	case "help":
		return parseHelpOptions(args[1:], opts, stderr)
	default:
		printUsage(stderr)
		return options{}, fmt.Errorf("unknown command %q", args[0])
	}
}

func parseLoopOptions(args []string, opts options, stderr io.Writer) (options, error) {
	if len(args) == 0 {
		printHelp(stderr, "loop")
		return options{}, errors.New("expected loop command")
	}
	if wantsHelp(args[:1]) {
		opts.command = "help"
		opts.helpTopic = "loop"
		return opts, nil
	}
	switch args[0] {
	case "push":
		opts.domain = "loop"
		opts.action = "push"
		opts.command = "loop push"
		opts.artifactType = loopArtifactType
		opts.layerType = loopLayerType
		return parsePublishOptions(args[1:], opts, stderr, "agentkit loop push [flags] <loop.yml> <registry/namespace/package:tag>")
	case "pull":
		opts.domain = "loop"
		opts.action = "pull"
		opts.command = "loop pull"
		opts.artifactType = loopArtifactType
		opts.layerType = loopLayerType
		opts.collectionType = loopCollectionType
		return parseInstallOptions(args[1:], opts, stderr, "agentkit loop pull [flags] <registry/namespace/package:tag>")
	case "render":
		opts.domain = "loop"
		opts.action = "render"
		opts.command = "loop render"
		opts.layerType = loopLayerType
		return parseRenderOptions(args[1:], opts, stderr)
	case "validate":
		opts.domain = "loop"
		opts.action = "validate"
		opts.command = "loop validate"
		return parseValidateOptions(args[1:], opts, stderr, "agentkit loop validate <loop.yml>")
	case "collection":
		return parseLoopCollectionOptions(args[1:], opts, stderr)
	default:
		printHelp(stderr, "loop")
		return options{}, fmt.Errorf("unknown loop command %q", args[0])
	}
}

func parseLoopCollectionOptions(args []string, opts options, stderr io.Writer) (options, error) {
	if len(args) == 0 {
		printHelp(stderr, "loop collection")
		return options{}, errors.New("expected loop collection command")
	}
	switch args[0] {
	case "push":
		opts.domain = "loop"
		opts.action = "push"
		opts.resource = "collection"
		opts.command = "loop collection push"
		opts.artifactType = loopCollectionType
		opts.collectionType = loopCollectionType
		return parsePublishOptions(args[1:], opts, stderr, "agentkit loop collection push <collection.json> <registry/namespace/package:tag>")
	default:
		printHelp(stderr, "loop collection")
		return options{}, fmt.Errorf("unknown loop collection command %q", args[0])
	}
}

func parseSkillOptions(args []string, opts options, stderr io.Writer) (options, error) {
	if len(args) == 0 {
		printHelp(stderr, "skill")
		return options{}, errors.New("expected skill command")
	}
	if wantsHelp(args[:1]) {
		opts.command = "help"
		opts.helpTopic = "skill"
		return opts, nil
	}
	switch args[0] {
	case "push":
		opts.domain = "skill"
		opts.action = "push"
		opts.command = "skill push"
		opts.artifactType = skillArtifactType
		opts.layerType = skillLayerType
		return parsePublishOptions(args[1:], opts, stderr, "agentkit skill push <skill-dir> <registry/namespace/package:tag>")
	case "pull":
		opts.domain = "skill"
		opts.action = "pull"
		opts.command = "skill pull"
		opts.artifactType = skillArtifactType
		opts.layerType = skillLayerType
		opts.collectionType = skillCollectionType
		return parseInstallOptions(args[1:], opts, stderr, "agentkit skill pull [flags] <registry/namespace/package:tag>")
	case "validate":
		opts.domain = "skill"
		opts.action = "validate"
		opts.command = "skill validate"
		return parseValidateOptions(args[1:], opts, stderr, "agentkit skill validate <skill-dir>")
	case "collection":
		return parseSkillCollectionOptions(args[1:], opts, stderr)
	default:
		printHelp(stderr, "skill")
		return options{}, fmt.Errorf("unknown skill command %q", args[0])
	}
}

func parseSkillCollectionOptions(args []string, opts options, stderr io.Writer) (options, error) {
	if len(args) == 0 {
		printHelp(stderr, "skill collection")
		return options{}, errors.New("expected skill collection command")
	}
	switch args[0] {
	case "push":
		opts.domain = "skill"
		opts.action = "push"
		opts.resource = "collection"
		opts.command = "skill collection push"
		opts.artifactType = skillCollectionType
		opts.collectionType = skillCollectionType
		return parsePublishOptions(args[1:], opts, stderr, "agentkit skill collection push <collection.json> <registry/namespace/package:tag>")
	default:
		printHelp(stderr, "skill collection")
		return options{}, fmt.Errorf("unknown skill collection command %q", args[0])
	}
}

func parsePublishOptions(args []string, opts options, stderr io.Writer, usage string) (options, error) {
	fs := flag.NewFlagSet(usage, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.artifactType, "artifact-type", opts.artifactType, "OCI artifact type")
	fs.StringVar(&opts.layerType, "layer-media-type", opts.layerType, "OCI layer media type")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: %s\n\n", usage)
		fs.PrintDefaults()
	}
	if wantsHelp(args) {
		topic := opts.command
		opts.command = "help"
		opts.helpTopic = topic
		return opts, nil
	}
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	if fs.NArg() != 2 {
		fs.Usage()
		return options{}, errors.New("expected local path and target reference")
	}
	opts.filename = fs.Arg(0)
	targetRef, registry, err := normalizeTargetRef(fs.Arg(1))
	if err != nil {
		return options{}, err
	}
	if targetTag(targetRef) == "" || targetDigest(targetRef) != "" {
		return options{}, fmt.Errorf("push target reference must include exactly one tag and no digest, got %q", fs.Arg(1))
	}
	opts.targetRef = targetRef
	opts.registry = registry
	return opts, nil
}

func parseInstallOptions(args []string, opts options, stderr io.Writer, usage string) (options, error) {
	fs := flag.NewFlagSet(usage, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.agentsDir, "agents-dir", opts.agentsDir, "agentkit root containing pulled loops, skills, manifest, and lock")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: %s\n\n", usage)
		fs.PrintDefaults()
	}
	if wantsHelp(args) {
		topic := opts.command
		opts.command = "help"
		opts.helpTopic = topic
		return opts, nil
	}
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return options{}, errors.New("expected target reference")
	}
	targetRef, registry, err := normalizeTargetRef(fs.Arg(0))
	if err != nil {
		return options{}, err
	}
	opts.targetRef = targetRef
	opts.registry = registry
	return opts, nil
}

func parseRenderOptions(args []string, opts options, stderr io.Writer) (options, error) {
	fs := flag.NewFlagSet("agentkit loop render", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.BoolVar(&opts.renderNoColor, "no-color", opts.renderNoColor, "disable ANSI color output")
	fs.BoolVar(&opts.renderDetails, "details", opts.renderDetails, "include compact phase actions, completion, and output summaries")
	fs.StringVar(&opts.layerType, "layer-media-type", opts.layerType, "OCI media type for the YAML layer")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: agentkit loop render [flags] <local.yml|registry/namespace/package:tag>\n\n")
		fs.PrintDefaults()
	}
	if wantsHelp(args) {
		opts.command = "help"
		opts.helpTopic = "loop render"
		return opts, nil
	}
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return options{}, errors.New("expected local loop file or target reference")
	}
	opts.source = fs.Arg(0)
	if isYAMLFilename(opts.source) {
		opts.filename = opts.source
		return opts, nil
	}
	targetRef, registry, err := normalizeTargetRef(opts.source)
	if err != nil {
		return options{}, err
	}
	opts.targetRef = targetRef
	opts.registry = registry
	return opts, nil
}

func parseValidateOptions(args []string, opts options, stderr io.Writer, usage string) (options, error) {
	fs := flag.NewFlagSet(usage, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprintf(stderr, "Usage: %s\n\n", usage) }
	if wantsHelp(args) {
		topic := opts.command
		opts.command = "help"
		opts.helpTopic = topic
		return opts, nil
	}
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return options{}, errors.New("expected local path")
	}
	opts.filename = fs.Arg(0)
	return opts, nil
}

func parseInitOptions(args []string, opts options, stderr io.Writer) (options, error) {
	opts.command = "init"
	opts.agentsFile = "AGENTS.md"
	fs := flag.NewFlagSet("agentkit init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.agentsFile, "agents-file", opts.agentsFile, "path to the agent instruction file to update")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: agentkit init [flags]\n\n")
		fs.PrintDefaults()
	}
	if wantsHelp(args) {
		opts.command = "help"
		opts.helpTopic = "init"
		return opts, nil
	}
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return options{}, errors.New("init does not accept arguments")
	}
	return opts, nil
}

func parseQuickstartOptions(args []string, opts options, stderr io.Writer) (options, error) {
	opts.command = "quickstart"
	fs := flag.NewFlagSet("agentkit quickstart", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprintf(stderr, "Usage: agentkit quickstart\n\n") }
	if wantsHelp(args) {
		opts.command = "help"
		opts.helpTopic = "quickstart"
		return opts, nil
	}
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return options{}, errors.New("quickstart does not accept arguments")
	}
	return opts, nil
}

func parsePrimeOptions(args []string, opts options, stderr io.Writer) (options, error) {
	opts.command = "prime"
	fs := flag.NewFlagSet("agentkit prime", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprintf(stderr, "Usage: agentkit prime\n\n") }
	if wantsHelp(args) {
		opts.command = "help"
		opts.helpTopic = "prime"
		return opts, nil
	}
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return options{}, errors.New("prime does not accept arguments")
	}
	return opts, nil
}

func parseHelpOptions(args []string, opts options, stderr io.Writer) (options, error) {
	opts.command = "help"
	fs := flag.NewFlagSet("agentkit help", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprintf(stderr, "Usage: agentkit help [topic]\n\n") }
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	if fs.NArg() > 2 {
		fs.Usage()
		return options{}, errors.New("help accepts at most two topic words")
	}
	if fs.NArg() > 0 {
		opts.helpTopic = fs.Arg(0)
	}
	if fs.NArg() == 2 {
		opts.helpTopic += " " + fs.Arg(1)
	}
	return opts, nil
}

func printUsage(stderr io.Writer) {
	printHelp(stderr, "")
}

func wantsHelp(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return true
		}
	}
	return false
}
