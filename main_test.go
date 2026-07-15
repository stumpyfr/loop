package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestParseOptionsRequiresPushLocalAndTarget(t *testing.T) {
	var stderr bytes.Buffer
	opts, err := parseOptions([]string{"push", "package.yml", "ghcr.io/Owner/Repo/package-one:v1"}, &stderr)
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if opts.filename != "package.yml" {
		t.Fatalf("filename = %q", opts.filename)
	}
	if opts.targetRef != "ghcr.io/owner/repo/package-one:v1" {
		t.Fatalf("targetRef = %q", opts.targetRef)
	}
	if opts.registry != defaultRegistry {
		t.Fatalf("registry = %q", opts.registry)
	}
}

func TestParseOptionsSupportsPullTarget(t *testing.T) {
	var stderr bytes.Buffer
	opts, err := parseOptions([]string{"pull", "ghcr.io/Owner/Repo/package-one:v1"}, &stderr)
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if opts.command != "pull" {
		t.Fatalf("command = %q", opts.command)
	}
	if opts.targetRef != "ghcr.io/owner/repo/package-one:v1" {
		t.Fatalf("targetRef = %q", opts.targetRef)
	}
	if opts.registry != defaultRegistry {
		t.Fatalf("registry = %q", opts.registry)
	}
}

func TestParseOptionsSupportsPullOutput(t *testing.T) {
	var stderr bytes.Buffer
	opts, err := parseOptions([]string{"pull", "--output", "pulled.yml", "ghcr.io/Owner/Repo/package-one:v1"}, &stderr)
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if opts.output != "pulled.yml" {
		t.Fatalf("output = %q", opts.output)
	}
}

func TestParseOptionsSupportsRunTarget(t *testing.T) {
	var stderr bytes.Buffer
	opts, err := parseOptions([]string{"run", "ghcr.io/Owner/Repo/package-one:v1"}, &stderr)
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if opts.command != "run" {
		t.Fatalf("command = %q", opts.command)
	}
	if opts.targetRef != "ghcr.io/owner/repo/package-one:v1" {
		t.Fatalf("targetRef = %q", opts.targetRef)
	}
	if opts.registry != defaultRegistry {
		t.Fatalf("registry = %q", opts.registry)
	}
}

func TestParseOptionsSupportsRenderLocalFilename(t *testing.T) {
	var stderr bytes.Buffer
	opts, err := parseOptions([]string{"render", "--no-color", "--details", "loop.yml"}, &stderr)
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if opts.command != "render" {
		t.Fatalf("command = %q", opts.command)
	}
	if opts.filename != "loop.yml" {
		t.Fatalf("filename = %q", opts.filename)
	}
	if !opts.renderNoColor {
		t.Fatal("expected no color")
	}
	if !opts.renderDetails {
		t.Fatal("expected details")
	}
}

func TestParseOptionsSupportsRenderOCITarget(t *testing.T) {
	var stderr bytes.Buffer
	opts, err := parseOptions([]string{"render", "ghcr.io/Owner/Repo/package-one:v1"}, &stderr)
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if opts.command != "render" {
		t.Fatalf("command = %q", opts.command)
	}
	if opts.targetRef != "ghcr.io/owner/repo/package-one:v1" {
		t.Fatalf("targetRef = %q", opts.targetRef)
	}
}

func TestParseOptionsSupportsValidateFilename(t *testing.T) {
	var stderr bytes.Buffer
	opts, err := parseOptions([]string{"validate", "loop.yml"}, &stderr)
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if opts.command != "validate" {
		t.Fatalf("command = %q", opts.command)
	}
	if opts.filename != "loop.yml" {
		t.Fatalf("filename = %q", opts.filename)
	}
}

func TestParseOptionsSupportsInit(t *testing.T) {
	var stderr bytes.Buffer
	opts, err := parseOptions([]string{"init"}, &stderr)
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if opts.command != "init" {
		t.Fatalf("command = %q", opts.command)
	}
	if opts.agentsFile != "AGENTS.md" {
		t.Fatalf("agentsFile = %q", opts.agentsFile)
	}
}

func TestParseOptionsSupportsInitAgentsFile(t *testing.T) {
	var stderr bytes.Buffer
	opts, err := parseOptions([]string{"init", "--agents-file", "docs/AGENTS.md"}, &stderr)
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if opts.agentsFile != "docs/AGENTS.md" {
		t.Fatalf("agentsFile = %q", opts.agentsFile)
	}
}

func TestParseOptionsSupportsQuickstart(t *testing.T) {
	var stderr bytes.Buffer
	opts, err := parseOptions([]string{"quickstart"}, &stderr)
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if opts.command != "quickstart" {
		t.Fatalf("command = %q", opts.command)
	}
}

func TestParseOptionsSupportsPrime(t *testing.T) {
	var stderr bytes.Buffer
	opts, err := parseOptions([]string{"prime"}, &stderr)
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if opts.command != "prime" {
		t.Fatalf("command = %q", opts.command)
	}
}

func TestParseOptionsSupportsHelp(t *testing.T) {
	var stderr bytes.Buffer
	opts, err := parseOptions([]string{"help", "run"}, &stderr)
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if opts.command != "help" {
		t.Fatalf("command = %q", opts.command)
	}
	if opts.helpTopic != "run" {
		t.Fatalf("helpTopic = %q", opts.helpTopic)
	}
}

func TestParseOptionsSupportsGlobalHelpFlag(t *testing.T) {
	var stderr bytes.Buffer
	opts, err := parseOptions([]string{"--help"}, &stderr)
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if opts.command != "help" {
		t.Fatalf("command = %q", opts.command)
	}
}

func TestParseOptionsSupportsCommandHelpFlag(t *testing.T) {
	var stderr bytes.Buffer
	opts, err := parseOptions([]string{"run", "--help"}, &stderr)
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if opts.command != "help" {
		t.Fatalf("command = %q", opts.command)
	}
	if opts.helpTopic != "run" {
		t.Fatalf("helpTopic = %q", opts.helpTopic)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestParseOptionsRejectsQuickstartArgs(t *testing.T) {
	var stderr bytes.Buffer
	_, err := parseOptions([]string{"quickstart", "ghcr.io/Owner/Repo/package-one:v1"}, &stderr)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseOptionsRejectsPrimeArgs(t *testing.T) {
	var stderr bytes.Buffer
	_, err := parseOptions([]string{"prime", "ghcr.io/Owner/Repo/package-one:v1"}, &stderr)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseOptionsRejectsHelpTooManyArgs(t *testing.T) {
	var stderr bytes.Buffer
	_, err := parseOptions([]string{"help", "run", "pull"}, &stderr)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseOptionsRejectsValidateWithoutFilename(t *testing.T) {
	var stderr bytes.Buffer
	_, err := parseOptions([]string{"validate"}, &stderr)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseOptionsRejectsRenderWithoutSource(t *testing.T) {
	var stderr bytes.Buffer
	_, err := parseOptions([]string{"render"}, &stderr)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseOptionsRejectsInitArgs(t *testing.T) {
	var stderr bytes.Buffer
	_, err := parseOptions([]string{"init", "AGENTS.md"}, &stderr)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseOptionsRejectsUnknownCommand(t *testing.T) {
	var stderr bytes.Buffer
	_, err := parseOptions([]string{"package.yml", "ghcr.io/owner/repo:v1"}, &stderr)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseOptionsRejectsTargetWithoutTag(t *testing.T) {
	var stderr bytes.Buffer
	_, err := parseOptions([]string{"push", "package.yml", "ghcr.io/owner/repo/package-one"}, &stderr)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseOptionsRequiresNamespaceAndPackage(t *testing.T) {
	var stderr bytes.Buffer
	_, err := parseOptions([]string{"push", "package.yml", "ghcr.io/package-one:v1"}, &stderr)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseOptionsSupportsRegistriesWithPorts(t *testing.T) {
	var stderr bytes.Buffer
	opts, err := parseOptions([]string{"push", "package.yml", "localhost:5000/Owner/Repo/package-one:v1"}, &stderr)
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if opts.registry != "localhost:5000" {
		t.Fatalf("registry = %q", opts.registry)
	}
	if opts.targetRef != "localhost:5000/owner/repo/package-one:v1" {
		t.Fatalf("targetRef = %q", opts.targetRef)
	}
}

func TestNormalizeTargetRefKeepsTagCase(t *testing.T) {
	ref, registry, err := normalizeTargetRef("GHCR.io/Owner/Repo/package-one:Release-1")
	if err != nil {
		t.Fatalf("normalizeTargetRef returned error: %v", err)
	}
	if registry != "ghcr.io" {
		t.Fatalf("registry = %q", registry)
	}
	if ref != "ghcr.io/owner/repo/package-one:Release-1" {
		t.Fatalf("ref = %q", ref)
	}
}

func TestGithubSourceURLInfersRepoFromGHCRRef(t *testing.T) {
	sourceURL, ok := githubSourceURL(options{
		registry:  defaultRegistry,
		targetRef: "ghcr.io/arkham-advisory/test-loophub/package-one:latest",
	})
	if !ok {
		t.Fatal("expected source URL")
	}
	want := "https://github.com/arkham-advisory/test-loophub"
	if sourceURL != want {
		t.Fatalf("sourceURL = %q, want %q", sourceURL, want)
	}
}

func TestGithubSourceURLIgnoresNonGHCRRefs(t *testing.T) {
	_, ok := githubSourceURL(options{
		registry:  "localhost:5000",
		targetRef: "localhost:5000/arkham-advisory/test-loophub/package-one:latest",
	})
	if ok {
		t.Fatal("expected no source URL")
	}
}

func TestManifestAnnotationsUseFixedCreatedTime(t *testing.T) {
	annotations := manifestAnnotations(options{
		registry:  defaultRegistry,
		targetRef: "ghcr.io/arkham-advisory/test-loophub/package-one:latest",
	})
	if annotations[ocispec.AnnotationCreated] != fixedCreatedTime {
		t.Fatalf("created = %q", annotations[ocispec.AnnotationCreated])
	}
	wantSource := "https://github.com/arkham-advisory/test-loophub"
	if annotations[sourceAnnotation] != wantSource {
		t.Fatalf("source = %q, want %q", annotations[sourceAnnotation], wantSource)
	}
}

func TestSameContentComparesDigestAndSize(t *testing.T) {
	first := ocispec.Descriptor{
		Digest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Size:   10,
	}
	second := ocispec.Descriptor{
		Digest: first.Digest,
		Size:   first.Size,
	}
	if !sameContent(first, second) {
		t.Fatal("expected descriptors to match")
	}
	second.Size = 11
	if sameContent(first, second) {
		t.Fatal("expected size mismatch")
	}
}

func TestSelectYAMLLayerPrefersConfiguredMediaType(t *testing.T) {
	want := ocispec.Descriptor{
		MediaType: defaultLayerType,
		Digest:    "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Size:      10,
	}
	got, err := selectYAMLLayer([]ocispec.Descriptor{
		{
			MediaType: "application/vnd.unknown",
			Digest:    "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			Size:      10,
		},
		want,
	}, defaultLayerType)
	if err != nil {
		t.Fatalf("selectYAMLLayer returned error: %v", err)
	}
	if got.Digest != want.Digest {
		t.Fatalf("Digest = %q, want %q", got.Digest, want.Digest)
	}
}

func TestSelectYAMLLayerFallsBackToTitle(t *testing.T) {
	want := ocispec.Descriptor{
		MediaType: "application/octet-stream",
		Digest:    "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Size:      10,
		Annotations: map[string]string{
			ocispec.AnnotationTitle: "package.yaml",
		},
	}
	got, err := selectYAMLLayer([]ocispec.Descriptor{want}, defaultLayerType)
	if err != nil {
		t.Fatalf("selectYAMLLayer returned error: %v", err)
	}
	if got.Digest != want.Digest {
		t.Fatalf("Digest = %q, want %q", got.Digest, want.Digest)
	}
}

func TestWritePulledFile(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "pulled.yml")
	if err := writePulledFile(filename, bytes.NewBufferString("name: demo\n")); err != nil {
		t.Fatalf("writePulledFile returned error: %v", err)
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "name: demo\n" {
		t.Fatalf("data = %q", data)
	}
}

func TestCachedLayerMatches(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "cached.yml")
	data := []byte("name: demo\n")
	if err := os.WriteFile(filename, data, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	desc := ocispec.Descriptor{
		Digest: digest.Digest("sha256:" + hex.EncodeToString(sum[:])),
		Size:   int64(len(data)),
	}
	if !cachedLayerMatches(filename, desc) {
		t.Fatal("expected cached layer to match")
	}
	desc.Size++
	if cachedLayerMatches(filename, desc) {
		t.Fatal("expected size mismatch")
	}
}

func TestPrintPullResult(t *testing.T) {
	var stdout bytes.Buffer
	printPullResult(&stdout, pullResult{
		ref:            "ghcr.io/owner/repo/package-one:latest",
		manifestDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		updated:        true,
	})
	got := stdout.String()
	for _, want := range []string{
		"latest: Pulling from owner/repo/package-one\n",
		"Digest: sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n",
		"Status: Downloaded newer image for ghcr.io/owner/repo/package-one:latest\n",
		"ghcr.io/owner/repo/package-one:latest\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q in %q", want, got)
		}
	}
}

func TestPrintQuickstartProvidesHumanIntro(t *testing.T) {
	var stdout bytes.Buffer
	printQuickstart(&stdout, options{})
	got := stdout.String()
	for _, want := range []string{
		"loop_cli - OCI-backed YAML loop packages",
		"GETTING STARTED",
		"loop_cli init",
		"loop_cli validate ./loop.yml",
		"loop_cli render ./loop.yml",
		"loop_cli push ./loop.yml ghcr.io/owner/repo/package:tag",
		"docker login ghcr.io",
		"Run `loop_cli prime` for agent-oriented workflow instructions.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q in %q", want, got)
		}
	}
}

func TestPrintPrimeProvidesAgentDocs(t *testing.T) {
	var stdout bytes.Buffer
	printPrime(&stdout, options{})
	got := stdout.String()
	for _, want := range []string{
		"# Loop CLI Agent Quickstart",
		"The main agent is an orchestrator only.",
		"Read the loop from the CLI output and execute it as the instruction source for the run.",
		"Do not perform independent exploration, repository changes, file edits, or phase work yourself.",
		"Only perform actions needed to complete the orchestrator role",
		"Coordinate the loop, manage phase state, and ensure required actions happen in the correct order.",
		"Track default phase outputs for every phase",
		"Do not make decisions on behalf of sub-agents",
		"Do not review, implement, or change the codebase yourself",
		"Add a `summary` section to the final report containing `status`, `objective`, `outcome`, and `duration_in_seconds`.",
		"Ensure each phase report contains only outputs defined by that phase",
		"`registry/namespace/package_name:tag`",
		"`loop_cli pull ghcr.io/owner/repo/package:tag`",
		"`loop_cli run ghcr.io/owner/repo/package:tag`",
		"treat that YAML as the execution instructions",
		"Inspect `spec.inputs` before starting the loop.",
		"Ask the user for any missing required input values before running phases.",
		"Do not invent missing input values.",
		"Act as the orchestrator",
		"craft a strict and guided sub-agent prompt from the phase context",
		"act as an expert in <domain>",
		"Do not pass the raw loop YAML as the sub-agent prompt.",
		"Derive the sub-agent role from the phase context",
		"structured report containing all outputs requested by that phase",
		"Close each phase sub-agent immediately after its structured report is collected and stored.",
		"do not keep phase sub-agents open across phases",
		"`.loop/runs/yyyy/mm/dd/hh/mm`",
		"phase_{idx++}_name.md",
		"report.json",
		"only outputs defined by each phase plus orchestrator-tracked `status` and `duration_in_seconds`",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q in %q", want, got)
		}
	}
}

func TestPrintHelpGuidesAgentsToPrime(t *testing.T) {
	var stdout bytes.Buffer
	printHelp(&stdout, "")
	got := stdout.String()
	for _, want := range []string{
		"Usage:",
		"Package Commands:",
		"render      Display a loop as a terminal flowchart",
		"validate    Validate a local loop YAML file",
		"init        Add loop_cli agent instructions to AGENTS.md",
		"Agent Commands:",
		"prime       Show AI-optimized workflow context for agents",
		"Agents should run `loop_cli prime` before running a loop package.",
		"Use \"loop_cli help [command]\" for more information about a command.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q in %q", want, got)
		}
	}
}

func TestPrintRenderHelp(t *testing.T) {
	var stdout bytes.Buffer
	printHelp(&stdout, "render")
	got := stdout.String()
	for _, want := range []string{
		"Display a loop as a terminal flowchart.",
		"loop_cli render [flags] <local.yml|registry/namespace/package_name:tag>",
		"phases, transitions, self-loops, and root escalation inputs",
		"-details",
		"-no-color",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q in %q", want, got)
		}
	}
}

func TestPrintValidateHelp(t *testing.T) {
	var stdout bytes.Buffer
	printHelp(&stdout, "validate")
	got := stdout.String()
	for _, want := range []string{
		"Validate a local loop YAML file.",
		"loop_cli validate <local.yml>",
		"embedded loop JSON Schema",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q in %q", want, got)
		}
	}
}

func TestPrintInitHelp(t *testing.T) {
	var stdout bytes.Buffer
	printHelp(&stdout, "init")
	got := stdout.String()
	for _, want := range []string{
		"Add loop_cli agent instructions to AGENTS.md.",
		"loop_cli init [flags]",
		"points agents to `loop_cli prime`",
		"-agents-file string",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q in %q", want, got)
		}
	}
}

func TestPrintRunHelpGuidesAgentsToPrime(t *testing.T) {
	var stdout bytes.Buffer
	printHelp(&stdout, "run")
	got := stdout.String()
	for _, want := range []string{
		"Usage:",
		"loop_cli run [flags] <registry/namespace/package_name:tag>",
		"Agent Note:",
		"Run `loop_cli prime` first when an agent is asked to execute a loop.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q in %q", want, got)
		}
	}
}

func TestRenderLoopUsesANSIByDefault(t *testing.T) {
	var stdout bytes.Buffer
	if err := renderLoop(&stdout, renderTestLoop(), renderOptions{}); err != nil {
		t.Fatalf("renderLoop returned error: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{
		"\x1b[36m[1] Review Ticket\x1b[0m",
		"if needs clarification -> [1] Review Ticket",
		"if clear -> [2] Edit Ticket",
		"escalation:\x1b[0m clarification_questions",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q in %q", want, got)
		}
	}
}

func TestRenderLoopNoColor(t *testing.T) {
	var stdout bytes.Buffer
	if err := renderLoop(&stdout, renderTestLoop(), renderOptions{noColor: true}); err != nil {
		t.Fatalf("renderLoop returned error: %v", err)
	}
	got := stdout.String()
	if strings.Contains(got, "\x1b[") {
		t.Fatalf("expected no ANSI codes in %q", got)
	}
	if !strings.Contains(got, "[2] Edit Ticket") {
		t.Fatalf("output missing phase in %q", got)
	}
}

func TestRenderLoopDetails(t *testing.T) {
	var stdout bytes.Buffer
	if err := renderLoop(&stdout, renderTestLoop(), renderOptions{noColor: true, details: true}); err != nil {
		t.Fatalf("renderLoop returned error: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{
		"objective: Review the ticket.",
		"actions: Read ticket.",
		"completion: Questions prepared.",
		"outputs: status [completed|escalation_needed]",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q in %q", want, got)
		}
	}
}

func TestRenderLoopImplicitSequentialAndEndEdges(t *testing.T) {
	loop := renderTestLoop()
	loop.Phases[0].Transitions = nil
	var stdout bytes.Buffer
	if err := renderLoop(&stdout, loop, renderOptions{noColor: true}); err != nil {
		t.Fatalf("renderLoop returned error: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{
		"next -> [2] Edit Ticket",
		"├─ end",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q in %q", want, got)
		}
	}
}

func TestRenderLoopRejectsInvalidTransitionTarget(t *testing.T) {
	loop := renderTestLoop()
	loop.Phases[0].Transitions = []loopTransition{{To: "missing", Condition: "broken"}}
	var stdout bytes.Buffer
	if err := renderLoop(&stdout, loop, renderOptions{noColor: true}); err == nil {
		t.Fatal("expected invalid transition error")
	} else if !strings.Contains(err.Error(), "unknown phase") {
		t.Fatalf("error = %v", err)
	}
}

func TestUpdateAgentsContentAppendsManagedBlock(t *testing.T) {
	got, changed, err := updateAgentsContent("# Agent Instructions\n")
	if err != nil {
		t.Fatalf("updateAgentsContent returned error: %v", err)
	}
	if !changed {
		t.Fatal("expected content to change")
	}
	for _, want := range []string{
		"# Agent Instructions",
		loopIntegrationBegin,
		"Run `loop_cli prime` before executing a loop package.",
		"loop_cli render ./loop.yml",
		loopIntegrationEnd,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("content missing %q in %q", want, got)
		}
	}
}

func TestUpdateAgentsContentReplacesManagedBlock(t *testing.T) {
	old := "# Agent Instructions\n\n" + loopIntegrationBegin + "\nold content\n" + loopIntegrationEnd + "\n"
	got, changed, err := updateAgentsContent(old)
	if err != nil {
		t.Fatalf("updateAgentsContent returned error: %v", err)
	}
	if !changed {
		t.Fatal("expected content to change")
	}
	if strings.Contains(got, "old content") {
		t.Fatalf("old block was not replaced: %q", got)
	}
	if strings.Count(got, loopIntegrationBegin) != 1 {
		t.Fatalf("expected one begin marker in %q", got)
	}
}

func TestUpdateAgentsContentIsIdempotent(t *testing.T) {
	current := loopAgentsBlock() + "\n"
	got, changed, err := updateAgentsContent(current)
	if err != nil {
		t.Fatalf("updateAgentsContent returned error: %v", err)
	}
	if changed {
		t.Fatal("expected content to be unchanged")
	}
	if got != current {
		t.Fatalf("got = %q, want %q", got, current)
	}
}

func TestUpdateAgentsContentRejectsPartialManagedBlock(t *testing.T) {
	_, _, err := updateAgentsContent(loopIntegrationBegin + "\n")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdateAgentsFileCreatesFile(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "AGENTS.md")
	action, err := updateAgentsFile(filename)
	if err != nil {
		t.Fatalf("updateAgentsFile returned error: %v", err)
	}
	if action != "created" {
		t.Fatalf("action = %q", action)
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "loop_cli prime") {
		t.Fatalf("file missing loop instructions: %q", string(data))
	}
}

func TestUpdateAgentsFileReportsUpdatedWhenAlreadyInstalled(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "AGENTS.md")
	if err := os.WriteFile(filename, []byte(loopAgentsBlock()+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	action, err := updateAgentsFile(filename)
	if err != nil {
		t.Fatalf("updateAgentsFile returned error: %v", err)
	}
	if action != "updated" {
		t.Fatalf("action = %q", action)
	}
}

func renderTestLoop() loopDefinition {
	return loopDefinition{
		Metadata: loopMetadata{
			Name:    "refine-feature",
			Version: "0.1.0",
			Title:   "Refine Feature",
		},
		Spec: loopSpec{
			Objective: "Clarify a Jira ticket.",
			Inputs: map[string]loopInput{
				"ticket": {
					Type:        "string",
					Required:    true,
					Description: "Jira ticket.",
				},
			},
		},
		Phases: []loopPhase{
			{
				Name:       "review-ticket",
				Title:      "Review Ticket",
				Objective:  "Review the ticket.",
				Actions:    []string{"Read ticket."},
				Completion: []string{"Questions prepared."},
				Outputs: []map[string]any{
					{"status": []any{"completed", "escalation_needed"}},
					{"questions": []any{"string"}},
				},
				Transitions: []loopTransition{
					{To: "review-ticket", Condition: "needs clarification"},
					{To: "edit-ticket", Condition: "clear"},
				},
			},
			{
				Name:       "edit-ticket",
				Title:      "Edit Ticket",
				Objective:  "Edit the ticket.",
				Actions:    []string{"Write summary."},
				Completion: []string{"Ticket edited."},
				Outputs: []map[string]any{
					{"status": []any{"completed"}},
				},
			},
		},
		Escalation: &loopEscalation{
			Principle: "Ask for clarification.",
			EscalationInputs: []loopEscalationInput{
				{Name: "clarification_questions", Type: []any{"string"}},
			},
		},
	}
}

func TestValidateYAMLFileAcceptsValidYAML(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "package.yml")
	if err := os.WriteFile(filename, []byte("name: demo\nversion: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateYAMLFile(filename); err != nil {
		t.Fatalf("validateYAMLFile returned error: %v", err)
	}
}

func TestValidateYAMLFileRejectsDuplicateKeys(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "package.yaml")
	if err := os.WriteFile(filename, []byte("name: one\nname: two\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateYAMLFile(filename); err == nil {
		t.Fatal("expected duplicate key error")
	}
}

func TestValidateLoopFileAcceptsValidLoop(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "loop.yml")
	if err := os.WriteFile(filename, []byte(validLoopYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateLoopFile(filename); err != nil {
		t.Fatalf("validateLoopFile returned error: %v", err)
	}
}

func TestValidateLoopFileRejectsSchemaViolation(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "loop.yml")
	if err := os.WriteFile(filename, []byte(`
apiVersion: loophub.dev/v1alpha1
kind: EngineeringLoop
metadata:
  name: demo
  version: 0.1.0
  title: Demo
  description: Demo loop.
spec:
  objective: Demo objective.
  inputs:
    ticket:
      type: string
      required: true
      description: Ticket id.
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateLoopFile(filename); err == nil {
		t.Fatal("expected schema validation error")
	} else if !strings.Contains(err.Error(), "invalid loop schema") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateLoopFileRejectsMetadataOrchestratorRole(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "loop.yml")
	loopYAML := strings.Replace(validLoopYAML, "  description: Demo loop.\n", "  description: Demo loop.\n  orchestrator_role: Coordinate phases only.\n", 1)
	if err := os.WriteFile(filename, []byte(loopYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateLoopFile(filename); err == nil {
		t.Fatal("expected schema validation error")
	} else if !strings.Contains(err.Error(), "orchestrator_role") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateYAMLFileRequiresYAMLExtension(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "package.txt")
	if err := os.WriteFile(filename, []byte("name: demo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateYAMLFile(filename); err == nil {
		t.Fatal("expected extension error")
	}
}

const validLoopYAML = `
apiVersion: loophub.dev/v1alpha1
kind: EngineeringLoop
metadata:
  name: demo
  version: 0.1.0
  title: Demo Loop
  description: Demo loop.
spec:
  objective: Deliver the requested change.
  inputs:
    ticket:
      type: string
      required: true
      description: Jira issue key or URL.
phases:
  - name: analyze
    title: Analyze Ticket
    objective: Understand the request.
    actions:
      - Read the ticket.
    completion:
      - Ticket is understood.
    outputs:
      - status: ["completed", "blocked", "escalated"]
      - summary: string
    transitions:
      - to: publish
        condition: Continue when complete.
escalation:
  principle: Escalate only when needed.
  escalation_inputs:
    - name: ambiguity
      type: string
      description: Missing decision.
`

func TestGHCREnvCredentialUsesExplicitTokenAndUsername(t *testing.T) {
	t.Setenv("GHCR_TOKEN", "token")
	t.Setenv("GHCR_USERNAME", "owner")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GITHUB_ACTOR", "")

	cred, ok := ghcrEnvCredential()
	if !ok {
		t.Fatal("expected env credential")
	}
	if cred.Username != "owner" {
		t.Fatalf("Username = %q", cred.Username)
	}
	if cred.Password != "token" {
		t.Fatalf("Password = %q", cred.Password)
	}
}

func TestGHCREnvCredentialFallsBackToGitHubActionsEnv(t *testing.T) {
	t.Setenv("GHCR_TOKEN", "")
	t.Setenv("GHCR_USERNAME", "")
	t.Setenv("GITHUB_TOKEN", "token")
	t.Setenv("GITHUB_ACTOR", "actor")

	cred, ok := ghcrEnvCredential()
	if !ok {
		t.Fatal("expected env credential")
	}
	if cred.Username != "actor" {
		t.Fatalf("Username = %q", cred.Username)
	}
	if cred.Password != "token" {
		t.Fatalf("Password = %q", cred.Password)
	}
}

func TestGHCREnvCredentialRequiresUsername(t *testing.T) {
	t.Setenv("GHCR_TOKEN", "token")
	t.Setenv("GHCR_USERNAME", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GITHUB_ACTOR", "")

	if _, ok := ghcrEnvCredential(); ok {
		t.Fatal("expected no env credential without a username")
	}
}
