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
	opts, err := parseOptions([]string{"loop", "push", "package.yml", "ghcr.io/Owner/Repo/package-one:v1"}, &stderr)
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
	opts, err := parseOptions([]string{"loop", "pull", "ghcr.io/Owner/Repo/package-one:v1"}, &stderr)
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if opts.command != "loop pull" {
		t.Fatalf("command = %q", opts.command)
	}
	if opts.targetRef != "ghcr.io/owner/repo/package-one:v1" {
		t.Fatalf("targetRef = %q", opts.targetRef)
	}
	if opts.registry != defaultRegistry {
		t.Fatalf("registry = %q", opts.registry)
	}
}

func TestParseOptionsSupportsPullAgentsDir(t *testing.T) {
	var stderr bytes.Buffer
	opts, err := parseOptions([]string{"loop", "pull", "--agents-dir", "custom-agents", "ghcr.io/Owner/Repo/package-one:v1"}, &stderr)
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if opts.agentsDir != "custom-agents" {
		t.Fatalf("agentsDir = %q", opts.agentsDir)
	}
}

func TestParseOptionsSupportsRunTarget(t *testing.T) {
	var stderr bytes.Buffer
	opts, err := parseOptions([]string{"loop", "pull", "ghcr.io/Owner/Repo/package-one:v1"}, &stderr)
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if opts.command != "loop pull" {
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
	opts, err := parseOptions([]string{"loop", "render", "--no-color", "--details", "loop.yml"}, &stderr)
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if opts.command != "loop render" {
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
	opts, err := parseOptions([]string{"loop", "render", "ghcr.io/Owner/Repo/package-one:v1"}, &stderr)
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if opts.command != "loop render" {
		t.Fatalf("command = %q", opts.command)
	}
	if opts.targetRef != "ghcr.io/owner/repo/package-one:v1" {
		t.Fatalf("targetRef = %q", opts.targetRef)
	}
}

func TestParseOptionsSupportsValidateFilename(t *testing.T) {
	var stderr bytes.Buffer
	opts, err := parseOptions([]string{"loop", "validate", "loop.yml"}, &stderr)
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if opts.command != "loop validate" {
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
	opts, err := parseOptions([]string{"help", "loop", "render"}, &stderr)
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if opts.command != "help" {
		t.Fatalf("command = %q", opts.command)
	}
	if opts.helpTopic != "loop render" {
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
	opts, err := parseOptions([]string{"loop", "render", "--help"}, &stderr)
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if opts.command != "help" {
		t.Fatalf("command = %q", opts.command)
	}
	if opts.helpTopic != "loop render" {
		t.Fatalf("helpTopic = %q", opts.helpTopic)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestParseOptionsSupportsSkillCommands(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		command string
	}{
		{name: "push", args: []string{"skill", "push", "skill-dir", "ghcr.io/Owner/Repo/skill:v1"}, command: "skill push"},
		{name: "pull", args: []string{"skill", "pull", "--agents-dir", "custom", "ghcr.io/Owner/Repo/skill:v1"}, command: "skill pull"},
		{name: "validate", args: []string{"skill", "validate", "skill-dir"}, command: "skill validate"},
		{name: "collection", args: []string{"skill", "collection", "push", "skills.json", "ghcr.io/Owner/Repo/skills:v1"}, command: "skill collection push"},
		{name: "loop collection", args: []string{"loop", "collection", "push", "loops.json", "ghcr.io/Owner/Repo/loops:v1"}, command: "loop collection push"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stderr bytes.Buffer
			opts, err := parseOptions(tt.args, &stderr)
			if err != nil {
				t.Fatalf("parseOptions returned error: %v", err)
			}
			if opts.command != tt.command {
				t.Fatalf("command = %q, want %q", opts.command, tt.command)
			}
		})
	}
}

func TestParseOptionsRejectsLegacyFlatCommands(t *testing.T) {
	for _, args := range [][]string{
		{"push", "loop.yml", "ghcr.io/owner/repo/package:v1"},
		{"pull", "ghcr.io/owner/repo/package:v1"},
		{"run", "ghcr.io/owner/repo/package:v1"},
		{"render", "loop.yml"},
		{"validate", "loop.yml"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stderr bytes.Buffer
			if _, err := parseOptions(args, &stderr); err == nil {
				t.Fatal("expected legacy command error")
			}
		})
	}
}

func TestParseOptionsRejectsTypedInstallCommands(t *testing.T) {
	for _, args := range [][]string{
		{"loop", "install", "ghcr.io/owner/repo/package:v1"},
		{"skill", "install", "ghcr.io/owner/repo/skill:v1"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stderr bytes.Buffer
			if _, err := parseOptions(args, &stderr); err == nil {
				t.Fatal("expected typed install command error")
			}
		})
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
	_, err := parseOptions([]string{"help", "loop", "render", "pull"}, &stderr)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseOptionsRejectsValidateWithoutFilename(t *testing.T) {
	var stderr bytes.Buffer
	_, err := parseOptions([]string{"loop", "validate"}, &stderr)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseOptionsRejectsRenderWithoutSource(t *testing.T) {
	var stderr bytes.Buffer
	_, err := parseOptions([]string{"loop", "render"}, &stderr)
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
	_, err := parseOptions([]string{"loop", "push", "package.yml", "ghcr.io/owner/repo/package-one"}, &stderr)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseOptionsRequiresNamespaceAndPackage(t *testing.T) {
	var stderr bytes.Buffer
	_, err := parseOptions([]string{"loop", "push", "package.yml", "ghcr.io/package-one:v1"}, &stderr)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseOptionsSupportsRegistriesWithPorts(t *testing.T) {
	var stderr bytes.Buffer
	opts, err := parseOptions([]string{"loop", "push", "package.yml", "localhost:5000/Owner/Repo/package-one:v1"}, &stderr)
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

func TestParseOptionsSupportsPullDigestReference(t *testing.T) {
	digestRef := "ghcr.io/Owner/Repo/package-one@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	var stderr bytes.Buffer
	opts, err := parseOptions([]string{"loop", "pull", digestRef}, &stderr)
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	want := "ghcr.io/owner/repo/package-one@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if opts.targetRef != want {
		t.Fatalf("targetRef = %q, want %q", opts.targetRef, want)
	}
	if opts.registry != defaultRegistry {
		t.Fatalf("registry = %q", opts.registry)
	}
}

func TestParseOptionsSupportsPullTagAndDigestReference(t *testing.T) {
	digestRef := "ghcr.io/Owner/Repo/package-one:Release-1@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	var stderr bytes.Buffer
	opts, err := parseOptions([]string{"skill", "pull", digestRef}, &stderr)
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	want := "ghcr.io/owner/repo/package-one:Release-1@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if opts.targetRef != want {
		t.Fatalf("targetRef = %q, want %q", opts.targetRef, want)
	}
	if targetTag(opts.targetRef) != "Release-1" {
		t.Fatalf("targetTag = %q", targetTag(opts.targetRef))
	}
	if targetDigest(opts.targetRef) != "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("targetDigest = %q", targetDigest(opts.targetRef))
	}
}

func TestParseOptionsRejectsPushDigestReference(t *testing.T) {
	var stderr bytes.Buffer
	_, err := parseOptions([]string{"loop", "push", "package.yml", "ghcr.io/owner/repo/package-one@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}, &stderr)
	if err == nil {
		t.Fatal("expected push digest reference error")
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

func TestDependencyOptionsUsesOCIRef(t *testing.T) {
	opts, err := dependencyOptions(options{agentsDir: ".agents"}, dependencySpec{
		Name: "python-codescene",
		Ref:  "ghcr.io/Owner/Repo/skill/python-codescene:0.1.0@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	})
	if err != nil {
		t.Fatalf("dependencyOptions returned error: %v", err)
	}
	want := "ghcr.io/owner/repo/skill/python-codescene:0.1.0@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if opts.targetRef != want {
		t.Fatalf("targetRef = %q, want %q", opts.targetRef, want)
	}
	if opts.registry != defaultRegistry {
		t.Fatalf("registry = %q", opts.registry)
	}
}

func TestDependencyOptionsRequiresRef(t *testing.T) {
	_, err := dependencyOptions(options{}, dependencySpec{Name: "missing"})
	if err == nil || !strings.Contains(err.Error(), "must include ref") {
		t.Fatalf("expected missing ref error, got %v", err)
	}
}

func TestGithubSourceURLInfersRepoFromGHCRRef(t *testing.T) {
	sourceURL, ok := githubSourceURL(options{
		registry:  defaultRegistry,
		targetRef: "ghcr.io/acme/loop-catalog/package-one:latest",
	})
	if !ok {
		t.Fatal("expected source URL")
	}
	want := "https://github.com/acme/loop-catalog"
	if sourceURL != want {
		t.Fatalf("sourceURL = %q, want %q", sourceURL, want)
	}
}

func TestGithubSourceURLIgnoresNonGHCRRefs(t *testing.T) {
	_, ok := githubSourceURL(options{
		registry:  "localhost:5000",
		targetRef: "localhost:5000/acme/loop-catalog/package-one:latest",
	})
	if ok {
		t.Fatal("expected no source URL")
	}
}

func TestManifestAnnotationsUseFixedCreatedTime(t *testing.T) {
	annotations := manifestAnnotations(options{
		registry:  defaultRegistry,
		targetRef: "ghcr.io/acme/loop-catalog/package-one:latest",
	})
	if annotations[ocispec.AnnotationCreated] != fixedCreatedTime {
		t.Fatalf("created = %q", annotations[ocispec.AnnotationCreated])
	}
	wantSource := "https://github.com/acme/loop-catalog"
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

func TestDescriptorManifestMatchesArtifactTypeFromDescriptor(t *testing.T) {
	desc := ocispec.Descriptor{ArtifactType: loopArtifactType}
	manifest := ocispec.Manifest{}
	if !descriptorManifestMatchesArtifactType(desc, manifest, loopArtifactType) {
		t.Fatal("expected descriptor artifact type to match")
	}
	if got := descriptorManifestArtifactType(desc, manifest); got != loopArtifactType {
		t.Fatalf("artifact type = %q, want %q", got, loopArtifactType)
	}
}

func TestDescriptorManifestMatchesArtifactTypeFromConfig(t *testing.T) {
	desc := ocispec.Descriptor{}
	manifest := ocispec.Manifest{
		Config: ocispec.Descriptor{MediaType: loopArtifactType},
	}
	if !descriptorManifestMatchesArtifactType(desc, manifest, loopArtifactType) {
		t.Fatal("expected config media type to match")
	}
	if got := descriptorManifestArtifactType(desc, manifest); got != loopArtifactType {
		t.Fatalf("artifact type = %q, want %q", got, loopArtifactType)
	}
}

func TestDescriptorManifestMatchesArtifactTypeFromLayerFallback(t *testing.T) {
	desc := ocispec.Descriptor{}
	manifest := ocispec.Manifest{
		Layers: []ocispec.Descriptor{{MediaType: loopLayerType}},
	}
	if !descriptorManifestMatchesArtifactType(desc, manifest, loopArtifactType) {
		t.Fatal("expected loop layer fallback to match")
	}
	if descriptorManifestMatchesArtifactType(desc, manifest, skillArtifactType) {
		t.Fatal("did not expect loop layer fallback to match skill artifact type")
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
		"agentkit",
		"GETTING STARTED",
		"agentkit init",
		"agentkit loop validate ./loop.yml",
		"agentkit loop render ./loop.yml",
		"agentkit loop push ./loop.yml ghcr.io/owner/repo/package:tag",
		"docker login ghcr.io",
		"Run `agentkit prime` for agent-oriented workflow instructions.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q in %q", want, got)
		}
	}
}

func TestPrintInstallResultPreservesTraversalOrder(t *testing.T) {
	var stdout bytes.Buffer
	result := installResult{
		Installed: []installRecord{
			{Kind: "skill", Name: "python-codescene", Path: ".agents/skills/python-codescene/0.1.0", Level: 1},
		},
		Skipped: []installRecord{
			{Kind: "loop", Name: "test", Path: ".agents/loops/test/0.1.0/loop.yml"},
		},
		Events: []installEvent{
			{Record: installRecord{Kind: "loop", Name: "test", Path: ".agents/loops/test/0.1.0/loop.yml"}, Skipped: true},
			{Record: installRecord{Kind: "skill", Name: "python-codescene", Path: ".agents/skills/python-codescene/0.1.0", Level: 1}},
		},
	}
	printInstallResult(&stdout, result)
	got := stdout.String()
	want := "already up to date loop test -> .agents/loops/test/0.1.0/loop.yml\n  installed skill python-codescene -> .agents/skills/python-codescene/0.1.0\n"
	if got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestPrintPrimeProvidesAgentDocs(t *testing.T) {
	var stdout bytes.Buffer
	printPrime(&stdout, options{})
	got := stdout.String()
	for _, want := range []string{
		"# Loop Agent Quickstart",
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
		"Dependency Skills",
		"do not pull dependency skills separately during execution",
		"`.agents/agentkit.lock.json`",
		"`.agents/loops/<loop-name>/<version>/loop.yml`",
		"`.agents/skills/<skill-name>/<version>/`",
		"compatibility copy at `.agents/loops/<loop-name>/loop.yml`",
		"compatibility copy at `.agents/skills/<skill-name>/`",
		"locked skill paths",
		"`registry/namespace/package_name:tag`",
		"`agentkit loop pull ghcr.io/owner/repo/package:tag`",
		"`agentkit loop render ghcr.io/owner/repo/package:tag`",
		"inspect the loop structure before execution",
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
		"Loop Commands:",
		"loop render",
		"loop validate",
		"init",
		"Agent Commands:",
		"prime",
		"agentkit [command]",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q in %q", want, got)
		}
	}
}

func TestPrintRenderHelp(t *testing.T) {
	var stdout bytes.Buffer
	printHelp(&stdout, "loop render")
	got := stdout.String()
	for _, want := range []string{
		"Display a loop as a terminal flowchart.",
		"agentkit loop render [flags] <local.yml|registry/namespace/package:tag>",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q in %q", want, got)
		}
	}
}

func TestPrintValidateHelp(t *testing.T) {
	var stdout bytes.Buffer
	printHelp(&stdout, "loop validate")
	got := stdout.String()
	for _, want := range []string{
		"Validate a local Agent Loop YAML file.",
		"agentkit loop validate <loop.yml>",
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
		"Add agentkit agent instructions to AGENTS.md.",
		"agentkit init [flags]",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q in %q", want, got)
		}
	}
}

func TestPrintLoopPullHelp(t *testing.T) {
	var stdout bytes.Buffer
	printHelp(&stdout, "loop pull")
	got := stdout.String()
	for _, want := range []string{
		"Usage:",
		"agentkit loop pull [flags] <registry/namespace/package:tag>",
		"-agents-dir",
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
		agentkitIntegrationBegin,
		"Run `agentkit prime` before executing an Agent Loop artifact.",
		"agentkit loop render ./loop.yml",
		agentkitIntegrationEnd,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("content missing %q in %q", want, got)
		}
	}
}

func TestUpdateAgentsContentReplacesManagedBlock(t *testing.T) {
	old := "# Agent Instructions\n\n" + agentkitIntegrationBegin + "\nold content\n" + agentkitIntegrationEnd + "\n"
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
	if strings.Count(got, agentkitIntegrationBegin) != 1 {
		t.Fatalf("expected one begin marker in %q", got)
	}
}

func TestUpdateAgentsContentIsIdempotent(t *testing.T) {
	current := agentkitAgentsBlock() + "\n"
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
	_, _, err := updateAgentsContent(agentkitIntegrationBegin + "\n")
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
	if !strings.Contains(string(data), "agentkit prime") {
		t.Fatalf("file missing agentkit instructions: %q", string(data))
	}
}

func TestUpdateAgentsFileReportsUpdatedWhenAlreadyInstalled(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "AGENTS.md")
	if err := os.WriteFile(filename, []byte(agentkitAgentsBlock()+"\n"), 0o600); err != nil {
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
apiVersion: agent-loops.dev/v1alpha1
kind: AgentLoop
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
apiVersion: agent-loops.dev/v1alpha1
kind: AgentLoop
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
      - to: push
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
