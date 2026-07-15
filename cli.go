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

	switch opts.command {
	case "push":
		if err := validateLoopFile(opts.filename); err != nil {
			return err
		}
		result, err := pushPackage(ctx, opts)
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
	case "pull":
		result, err := pullPackage(ctx, opts)
		if err != nil {
			return err
		}
		printPullResult(stdout, result)
		return nil
	case "run":
		return runPackage(ctx, opts, stdout)
	case "render":
		return renderSource(ctx, opts, stdout)
	case "validate":
		if err := validateLoopFile(opts.filename); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "valid %s\n", opts.filename)
		return nil
	case "init":
		action, err := updateAgentsFile(opts.agentsFile)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "%s %s\n", action, opts.agentsFile)
		return nil
	case "quickstart":
		printQuickstart(stdout, opts)
		return nil
	case "prime":
		printPrime(stdout, opts)
		return nil
	case "help":
		printHelp(stdout, opts.helpTopic)
		return nil
	default:
		return fmt.Errorf("unknown command %q", opts.command)
	}
}

func parseOptions(args []string, stderr io.Writer) (options, error) {
	opts := options{
		artifactType: defaultArtifactType,
		layerType:    defaultLayerType,
	}

	if len(args) == 0 {
		printUsage(stderr)
		return options{}, errors.New("expected command")
	}
	if args[0] == "-h" || args[0] == "--help" {
		opts.command = "help"
		return opts, nil
	}
	switch args[0] {
	case "push":
		return parsePushOptions(args[1:], opts, stderr)
	case "pull":
		return parsePullOptions(args[1:], opts, stderr)
	case "run":
		return parseRunOptions(args[1:], opts, stderr)
	case "render":
		return parseRenderOptions(args[1:], opts, stderr)
	case "validate":
		return parseValidateOptions(args[1:], opts, stderr)
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

func parsePushOptions(args []string, opts options, stderr io.Writer) (options, error) {
	opts.command = "push"
	fs := flag.NewFlagSet("loop_cli push", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.artifactType, "artifact-type", opts.artifactType, "OCI artifact type for the package manifest")
	fs.StringVar(&opts.layerType, "layer-media-type", opts.layerType, "OCI media type for the YAML layer")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: loop_cli push [flags] <local.yml> <registry/namespace/package_name:tag>\n\n")
		fs.PrintDefaults()
	}
	if wantsHelp(args) {
		opts.command = "help"
		opts.helpTopic = "push"
		return opts, nil
	}

	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	if fs.NArg() != 2 {
		fs.Usage()
		return options{}, errors.New("expected <local.yml> and <registry/namespace/package_name:tag>")
	}

	opts.filename = fs.Arg(0)
	targetRef, registry, err := normalizeTargetRef(fs.Arg(1))
	if err != nil {
		return options{}, err
	}
	opts.targetRef = targetRef
	opts.registry = registry
	return opts, nil
}

func parsePullOptions(args []string, opts options, stderr io.Writer) (options, error) {
	opts.command = "pull"
	fs := flag.NewFlagSet("loop_cli pull", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.output, "output", opts.output, "also copy pulled YAML to a file")
	fs.StringVar(&opts.layerType, "layer-media-type", opts.layerType, "OCI media type for the YAML layer")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: loop_cli pull [flags] <registry/namespace/package_name:tag>\n\n")
		fs.PrintDefaults()
	}
	if wantsHelp(args) {
		opts.command = "help"
		opts.helpTopic = "pull"
		return opts, nil
	}

	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return options{}, errors.New("expected <registry/namespace/package_name:tag>")
	}

	targetRef, registry, err := normalizeTargetRef(fs.Arg(0))
	if err != nil {
		return options{}, err
	}
	opts.targetRef = targetRef
	opts.registry = registry
	return opts, nil
}

func parseRunOptions(args []string, opts options, stderr io.Writer) (options, error) {
	opts.command = "run"
	fs := flag.NewFlagSet("loop_cli run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.layerType, "layer-media-type", opts.layerType, "OCI media type for the YAML layer")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: loop_cli run [flags] <registry/namespace/package_name:tag>\n\n")
		fs.PrintDefaults()
	}
	if wantsHelp(args) {
		opts.command = "help"
		opts.helpTopic = "run"
		return opts, nil
	}

	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return options{}, errors.New("expected <registry/namespace/package_name:tag>")
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
	opts.command = "render"
	fs := flag.NewFlagSet("loop_cli render", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.BoolVar(&opts.renderNoColor, "no-color", opts.renderNoColor, "disable ANSI color output")
	fs.BoolVar(&opts.renderDetails, "details", opts.renderDetails, "include compact phase actions, completion, and output summaries")
	fs.StringVar(&opts.layerType, "layer-media-type", opts.layerType, "OCI media type for the YAML layer")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: loop_cli render [flags] <local.yml|registry/namespace/package_name:tag>\n\n")
		fs.PrintDefaults()
	}
	if wantsHelp(args) {
		opts.command = "help"
		opts.helpTopic = "render"
		return opts, nil
	}

	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return options{}, errors.New("expected <local.yml|registry/namespace/package_name:tag>")
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

func parseValidateOptions(args []string, opts options, stderr io.Writer) (options, error) {
	opts.command = "validate"
	fs := flag.NewFlagSet("loop_cli validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: loop_cli validate <local.yml>\n\n")
	}
	if wantsHelp(args) {
		opts.command = "help"
		opts.helpTopic = "validate"
		return opts, nil
	}

	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return options{}, errors.New("expected <local.yml>")
	}

	opts.filename = fs.Arg(0)
	return opts, nil
}

func parseInitOptions(args []string, opts options, stderr io.Writer) (options, error) {
	opts.command = "init"
	opts.agentsFile = "AGENTS.md"
	fs := flag.NewFlagSet("loop_cli init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.agentsFile, "agents-file", opts.agentsFile, "path to the agent instruction file to update")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: loop_cli init [flags]\n\n")
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
	fs := flag.NewFlagSet("loop_cli quickstart", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: loop_cli quickstart\n\n")
		fs.PrintDefaults()
	}
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
	fs := flag.NewFlagSet("loop_cli prime", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: loop_cli prime\n\n")
		fs.PrintDefaults()
	}
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
	fs := flag.NewFlagSet("loop_cli help", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: loop_cli help [command]\n\n")
	}

	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	if fs.NArg() > 1 {
		fs.Usage()
		return options{}, errors.New("help accepts at most one command")
	}
	if fs.NArg() == 1 {
		opts.helpTopic = fs.Arg(0)
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
