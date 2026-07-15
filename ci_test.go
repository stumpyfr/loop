package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
)

func TestMainReturnsForHelpCommand(t *testing.T) {
	oldArgs := os.Args
	defer func() {
		os.Args = oldArgs
	}()
	os.Args = []string{"loop_cli", "help", "validate"}
	main()
}

func TestRunValidatePrintsValidForLoopFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run(context.Background(), []string{"validate", filepath.Join("testdata", "valid_loop.yml")}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run validate returned error: %v\nstderr: %s", err, stderr.String())
	}
	if got, want := stdout.String(), "valid testdata/valid_loop.yml\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestRunInitCreatesAgentsFile(t *testing.T) {
	dir := t.TempDir()
	agentsFile := filepath.Join(dir, "AGENTS.md")

	var stdout, stderr bytes.Buffer
	err := run(context.Background(), []string{"init", "--agents-file", agentsFile}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run init returned error: %v\nstderr: %s", err, stderr.String())
	}
	if got, want := stdout.String(), "created "+agentsFile+"\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	data, err := os.ReadFile(agentsFile)
	if err != nil {
		t.Fatalf("read agents file: %v", err)
	}
	if !strings.Contains(string(data), "loop_cli prime") {
		t.Fatalf("agents file does not mention loop_cli prime:\n%s", string(data))
	}

	stdout.Reset()
	stderr.Reset()
	err = run(context.Background(), []string{"init", "--agents-file", agentsFile}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("second run init returned error: %v\nstderr: %s", err, stderr.String())
	}
	if !strings.HasPrefix(stdout.String(), "updated ") {
		t.Fatalf("second init stdout = %q", stdout.String())
	}
}

func TestRunHelpQuickstartAndPrimePrintGuidance(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "help", args: []string{"help", "validate"}, want: "loop_cli validate <local.yml>"},
		{name: "quickstart", args: []string{"quickstart"}, want: "loop_cli validate ./loop.yml"},
		{name: "prime", args: []string{"prime"}, want: "Agent Role"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := run(context.Background(), tt.args, &stdout, &stderr)
			if err != nil {
				t.Fatalf("run returned error: %v\nstderr: %s", err, stderr.String())
			}
			if !strings.Contains(stdout.String(), tt.want) {
				t.Fatalf("stdout does not contain %q:\n%s", tt.want, stdout.String())
			}
		})
	}
}

func TestRunUnknownCommandReturnsUsageAndError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run(context.Background(), []string{"wat"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), `unknown command "wat"`) {
		t.Fatalf("error = %v", err)
	}
	if !strings.Contains(stderr.String(), "loop_cli [command]") {
		t.Fatalf("stderr does not contain usage:\n%s", stderr.String())
	}
}

func TestRunPushStopsOnLocalValidationError(t *testing.T) {
	dir := t.TempDir()
	badLoop := filepath.Join(dir, "bad.yml")
	if err := os.WriteFile(badLoop, []byte("kind: nope\n"), 0o600); err != nil {
		t.Fatalf("write bad loop: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err := run(context.Background(), []string{"push", badLoop, "ghcr.io/stumpyfr/loop/bad:v1"}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "invalid loop schema") {
		t.Fatalf("expected schema validation error, got %v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunInitReportsFileErrors(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	err := run(context.Background(), []string{"init", "--agents-file", dir}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "is a directory") {
		t.Fatalf("expected read agents file error, got %v", err)
	}
}

func TestRunWithoutCommandPrintsUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run(context.Background(), nil, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "expected command") {
		t.Fatalf("expected command error, got %v", err)
	}
	if !strings.Contains(stderr.String(), "loop_cli [command]") {
		t.Fatalf("stderr missing usage:\n%s", stderr.String())
	}
}

func TestCacheHelpersWriteCopyAndMatchLayer(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.yml")
	dest := filepath.Join(dir, "dest.yml")
	copied := filepath.Join(dir, "copied.yml")
	content := []byte("apiVersion: loophub.dev/v1alpha1\n")

	if err := writePulledFile(source, bytes.NewReader(content)); err != nil {
		t.Fatalf("writePulledFile source: %v", err)
	}
	if err := copyFile(source, dest); err != nil {
		t.Fatalf("copyFile: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("copied content = %q", string(got))
	}

	sum := sha256.Sum256(content)
	layer := ocispec.Descriptor{
		Size:   int64(len(content)),
		Digest: digest.Digest("sha256:" + hex.EncodeToString(sum[:])),
	}
	if !cachedLayerMatches(dest, layer) {
		t.Fatal("expected cached layer to match")
	}
	layer.Size++
	if cachedLayerMatches(dest, layer) {
		t.Fatal("expected size mismatch to fail")
	}
	if err := copyFile(source, copied); err != nil {
		t.Fatalf("copyFile copied: %v", err)
	}
	if err := writePulledFile(" ", bytes.NewReader(content)); err == nil || !strings.Contains(err.Error(), "output path") {
		t.Fatalf("expected empty output path error, got %v", err)
	}
}

func TestCachedFilePathUsesStableHashUnderCacheDir(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("HOME", cacheDir)

	got, err := cachedFilePath("ghcr.io/stumpyfr/loop/package:v1")
	if err != nil {
		t.Fatalf("cachedFilePath returned error: %v", err)
	}
	if !strings.Contains(got, filepath.Join("loop_cli", "refs")+string(os.PathSeparator)) {
		t.Fatalf("cached path %q does not use test cache dir %q", got, cacheDir)
	}
	if filepath.Ext(got) != ".yml" {
		t.Fatalf("cached path ext = %q", filepath.Ext(got))
	}
}

func TestSecurityErrorWrappersAddGHCRHints(t *testing.T) {
	err := errors.New("response status code 403")

	if got := wrapUploadError("ghcr.io", err).Error(); !strings.Contains(got, "GHCR denied the upload") {
		t.Fatalf("upload error missing GHCR hint: %s", got)
	}
	if got := wrapUploadError("example.com", err).Error(); !strings.Contains(got, "upload package:") {
		t.Fatalf("non-GHCR upload error missing wrapper: %s", got)
	}
	if got := wrapPullError("ghcr.io/", err).Error(); !strings.Contains(got, "GHCR denied the pull") {
		t.Fatalf("pull error missing GHCR hint: %s", got)
	}
	if got := wrapPullError("example.com", err); got != err {
		t.Fatalf("non-GHCR pull error = %v, want original", got)
	}
}

func TestYAMLConversionHandlesScalarTypesAndRejectsAliases(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "values.yml")
	data := `
name: loop
enabled: true
count: 3
ratio: 1.5
nothing: null
items:
  - one
`
	if err := os.WriteFile(filename, []byte(data), 0o600); err != nil {
		t.Fatalf("write yaml: %v", err)
	}
	_, root, err := loadYAMLDocument(filename)
	if err != nil {
		t.Fatalf("loadYAMLDocument: %v", err)
	}
	value, err := yamlNodeToJSONValue(root)
	if err != nil {
		t.Fatalf("yamlNodeToJSONValue: %v", err)
	}
	object := value.(map[string]any)
	if object["enabled"] != true || object["count"] != int64(3) || object["ratio"] != 1.5 || object["nothing"] != nil {
		t.Fatalf("unexpected converted values: %#v", object)
	}

	aliasFile := filepath.Join(dir, "alias.yml")
	if err := os.WriteFile(aliasFile, []byte("first: &x value\nsecond: *x\n"), 0o600); err != nil {
		t.Fatalf("write alias yaml: %v", err)
	}
	_, aliasRoot, err := loadYAMLDocument(aliasFile)
	if err != nil {
		t.Fatalf("load alias yaml: %v", err)
	}
	if _, err := yamlNodeToJSONValue(aliasRoot); err == nil || !strings.Contains(err.Error(), "aliases are not supported") {
		t.Fatalf("expected alias error, got %v", err)
	}
}

func TestRunRenderLocalNoColorDetails(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var stdout, stderr bytes.Buffer
	err := run(context.Background(), []string{"render", "--details", filepath.Join("testdata", "valid_loop.yml")}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run render returned error: %v\nstderr: %s", err, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{"Loop:", "Inspect", "actions:", "Escalation:"} {
		if !strings.Contains(output, want) {
			t.Fatalf("render output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "\x1b[") {
		t.Fatalf("render output should not contain ANSI escapes:\n%s", output)
	}
}

func TestRenderSourceFilenameRequiresSource(t *testing.T) {
	filename, err := renderSourceFilename(context.Background(), options{filename: "local.yml"})
	if err != nil {
		t.Fatalf("renderSourceFilename local returned error: %v", err)
	}
	if filename != "local.yml" {
		t.Fatalf("filename = %q", filename)
	}

	_, err = renderSourceFilename(context.Background(), options{})
	if err == nil || !strings.Contains(err.Error(), "render source is empty") {
		t.Fatalf("expected empty source error, got %v", err)
	}
}

func TestRenderLoopRejectsDuplicateAndUnknownPhases(t *testing.T) {
	duplicate := loopDefinition{
		Metadata: loopMetadata{Title: "Duplicate", Version: "0.1.0"},
		Phases: []loopPhase{
			{Name: "same", Title: "One"},
			{Name: "same", Title: "Two"},
		},
	}
	if err := renderLoop(ioDiscard{}, duplicate, renderOptions{noColor: true}); err == nil || !strings.Contains(err.Error(), "duplicate phase name") {
		t.Fatalf("expected duplicate phase error, got %v", err)
	}

	unknown := loopDefinition{
		Metadata: loopMetadata{Title: "Unknown", Version: "0.1.0"},
		Phases: []loopPhase{
			{Name: "start", Title: "Start", Transitions: []loopTransition{{To: "missing", Condition: "ready"}}},
		},
	}
	if err := renderLoop(ioDiscard{}, unknown, renderOptions{noColor: true}); err == nil || !strings.Contains(err.Error(), "unknown phase") {
		t.Fatalf("expected unknown phase error, got %v", err)
	}
}

func TestRenderHelpersCoverEdgeCases(t *testing.T) {
	if got := renderCondition("ready"); got != "if ready" {
		t.Fatalf("renderCondition = %q", got)
	}
	if got := renderCondition(""); got != "if condition" {
		t.Fatalf("empty renderCondition = %q", got)
	}
	if got := renderInputs(nil); got != "none" {
		t.Fatalf("renderInputs = %q", got)
	}
	if got := renderEscalationInputs(nil); got != "none" {
		t.Fatalf("renderEscalationInputs = %q", got)
	}
	if got := renderType([]string{"a", "b"}); got != "[a|b]" {
		t.Fatalf("renderType []string = %q", got)
	}
	values := []string{"c", "a", "b"}
	sortStrings(values)
	if got := strings.Join(values, ""); got != "abc" {
		t.Fatalf("sorted values = %q", got)
	}
}

func TestPrintHelpCoversEveryTopic(t *testing.T) {
	for _, topic := range []string{"", "push", "pull", "run", "render", "validate", "init", "quickstart", "prime", "help", "unknown"} {
		var stdout bytes.Buffer
		printHelp(&stdout, topic)
		if stdout.Len() == 0 {
			t.Fatalf("topic %q printed no help", topic)
		}
	}
}

func TestGHCREnvironmentCredentials(t *testing.T) {
	t.Setenv("GHCR_USERNAME", "octo")
	t.Setenv("GHCR_TOKEN", "secret")
	cred, ok := ghcrEnvCredential()
	if !ok {
		t.Fatal("expected GHCR credentials")
	}
	if cred.Username != "octo" || cred.Password != "secret" {
		t.Fatalf("credential = %#v", cred)
	}

	credentialFunc, err := registryCredential("ghcr.io/")
	if err != nil {
		t.Fatalf("registryCredential returned error: %v", err)
	}
	if credentialFunc == nil {
		t.Fatal("expected credential function")
	}
	repo, err := newRemoteRepository(options{targetRef: "ghcr.io/stumpyfr/loop/review:v1", registry: "ghcr.io"})
	if err != nil {
		t.Fatalf("newRemoteRepository returned error: %v", err)
	}
	if repo.Client == nil {
		t.Fatal("expected authenticated remote client")
	}
}

func TestGHCREnvironmentCredentialsRequireTokenAndUsername(t *testing.T) {
	t.Setenv("GHCR_USERNAME", "")
	t.Setenv("GHCR_TOKEN", "")
	t.Setenv("GITHUB_ACTOR", "")
	t.Setenv("GITHUB_TOKEN", "")
	if _, ok := ghcrEnvCredential(); ok {
		t.Fatal("expected missing GHCR credentials")
	}

	t.Setenv("GHCR_TOKEN", "secret")
	if _, ok := ghcrEnvCredential(); ok {
		t.Fatal("expected missing username to reject credentials")
	}
}

func TestGHCREnvironmentCredentialsFallBackToGitHubActionsEnv(t *testing.T) {
	t.Setenv("GHCR_USERNAME", "")
	t.Setenv("GHCR_TOKEN", "")
	t.Setenv("GITHUB_ACTOR", "actions-user")
	t.Setenv("GITHUB_TOKEN", "actions-token")

	cred, ok := ghcrEnvCredential()
	if !ok {
		t.Fatal("expected GitHub Actions credentials")
	}
	if cred.Username != "actions-user" || cred.Password != "actions-token" {
		t.Fatalf("credential = %#v", cred)
	}
}

func TestValidateYAMLErrorsAreActionable(t *testing.T) {
	dir := t.TempDir()
	if err := validateYAMLFile(filepath.Join(dir, "loop.txt")); err == nil || !strings.Contains(err.Error(), ".yaml or .yml") {
		t.Fatalf("expected extension error, got %v", err)
	}

	empty := filepath.Join(dir, "empty.yml")
	if err := os.WriteFile(empty, []byte(" \n"), 0o600); err != nil {
		t.Fatalf("write empty yaml: %v", err)
	}
	if err := validateYAMLFile(empty); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("expected empty yaml error, got %v", err)
	}

	duplicate := filepath.Join(dir, "duplicate.yml")
	if err := os.WriteFile(duplicate, []byte("name: one\nname: two\n"), 0o600); err != nil {
		t.Fatalf("write duplicate yaml: %v", err)
	}
	if err := validateYAMLFile(duplicate); err == nil || !strings.Contains(err.Error(), "duplicate mapping key") {
		t.Fatalf("expected duplicate key error, got %v", err)
	}

	if _, err := readLoopDefinition(filepath.Join(dir, "missing.yml")); err == nil || !strings.Contains(err.Error(), "read loop file") {
		t.Fatalf("expected read loop error, got %v", err)
	}
}

func TestRemoteOperationsExposeEarlyLocalErrors(t *testing.T) {
	_, err := pullPackage(context.Background(), options{})
	if err == nil || !strings.Contains(err.Error(), "create remote repository") {
		t.Fatalf("expected remote repository error, got %v", err)
	}

	_, err = pushPackage(context.Background(), options{
		filename:  filepath.Join(t.TempDir(), "missing.yml"),
		targetRef: "ghcr.io/stumpyfr/loop/missing:v1",
		registry:  "ghcr.io",
		layerType: defaultLayerType,
	})
	if err == nil || !strings.Contains(err.Error(), "add yaml layer") {
		t.Fatalf("expected local layer error, got %v", err)
	}
}

func TestRunPackageUsesCachedFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ref := "ghcr.io/stumpyfr/loop/package:v1"
	cachePath, err := cachedFilePath(ref)
	if err != nil {
		t.Fatalf("cachedFilePath: %v", err)
	}
	if err := os.WriteFile(cachePath, []byte("kind: EngineeringLoop\n"), 0o600); err != nil {
		t.Fatalf("write cached package: %v", err)
	}

	var stdout bytes.Buffer
	err = runPackage(context.Background(), options{targetRef: ref}, &stdout)
	if err != nil {
		t.Fatalf("runPackage returned error: %v", err)
	}
	if got := stdout.String(); got != "kind: EngineeringLoop\n" {
		t.Fatalf("stdout = %q", got)
	}
}

func TestRunCommandPrintsCachedPackage(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ref := "ghcr.io/stumpyfr/loop/package:v2"
	cachePath, err := cachedFilePath(ref)
	if err != nil {
		t.Fatalf("cachedFilePath: %v", err)
	}
	if err := os.WriteFile(cachePath, []byte("cached: true\n"), 0o600); err != nil {
		t.Fatalf("write cached package: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err = run(context.Background(), []string{"run", ref}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run command returned error: %v\nstderr: %s", err, stderr.String())
	}
	if got := stdout.String(); got != "cached: true\n" {
		t.Fatalf("stdout = %q", got)
	}
}

func TestRunPackageCacheMissReturnsPullError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var stdout bytes.Buffer
	err := runPackage(context.Background(), options{}, &stdout)
	if err == nil || !strings.Contains(err.Error(), "create remote repository") {
		t.Fatalf("expected pull setup error, got %v", err)
	}
}

func TestPushGraphPushesDescriptorOnce(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "loop.yml")
	if err := os.WriteFile(source, []byte("kind: EngineeringLoop\n"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	storage, err := file.New("")
	if err != nil {
		t.Fatalf("create storage: %v", err)
	}
	defer storage.Close()
	desc, err := storage.Add(context.Background(), "loop.yml", defaultLayerType, source)
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}

	pusher := &recordingPusher{}
	pushed := map[string]struct{}{}
	if err := pushGraph(context.Background(), storage, pusher, desc, pushed); err != nil {
		t.Fatalf("pushGraph returned error: %v", err)
	}
	if err := pushGraph(context.Background(), storage, pusher, desc, pushed); err != nil {
		t.Fatalf("pushGraph second call returned error: %v", err)
	}
	if pusher.count != 1 {
		t.Fatalf("push count = %d, want 1", pusher.count)
	}
}

func TestPushGraphPushesPackedManifestAndLayer(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "loop.yml")
	if err := os.WriteFile(source, []byte("kind: EngineeringLoop\n"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	storage, err := file.New("")
	if err != nil {
		t.Fatalf("create storage: %v", err)
	}
	defer storage.Close()
	layer, err := storage.Add(context.Background(), "loop.yml", defaultLayerType, source)
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}
	manifest, err := oras.Pack(context.Background(), storage, defaultArtifactType, []ocispec.Descriptor{layer}, oras.PackOptions{PackImageManifest: true})
	if err != nil {
		t.Fatalf("pack manifest: %v", err)
	}

	pusher := &recordingPusher{}
	if err := pushGraph(context.Background(), storage, pusher, manifest, map[string]struct{}{}); err != nil {
		t.Fatalf("pushGraph returned error: %v", err)
	}
	if pusher.count < 2 {
		t.Fatalf("push count = %d, want at least manifest and layer", pusher.count)
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) {
	return len(p), nil
}

type recordingPusher struct {
	count int
}

func (p *recordingPusher) Push(_ context.Context, _ ocispec.Descriptor, reader io.Reader) error {
	if _, err := io.Copy(io.Discard, reader); err != nil {
		return err
	}
	p.count++
	return nil
}
