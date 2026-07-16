package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	"oras.land/oras-go/v2/errdef"
)

func TestMainReturnsForHelpCommand(t *testing.T) {
	oldArgs := os.Args
	defer func() {
		os.Args = oldArgs
	}()
	os.Args = []string{"agentkit", "help", "loop", "validate"}
	main()
}

func TestValidateSkillDirAndArchiveAreDeterministic(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "demo-skill")
	if err := os.MkdirAll(filepath.Join(skillDir, "references"), 0o700); err != nil {
		t.Fatal(err)
	}
	skill := []byte("---\nname: demo-skill\ndescription: Demo skill.\nlicense: Apache-2.0\n---\n# Demo\n")
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), skill, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "references", "note.md"), []byte("hello\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateSkillDir(skillDir); err != nil {
		t.Fatalf("validateSkillDir returned error: %v", err)
	}
	first, err := createSkillArchive(skillDir, "demo-skill")
	if err != nil {
		t.Fatalf("createSkillArchive first: %v", err)
	}
	defer os.Remove(first)
	second, err := createSkillArchive(skillDir, "demo-skill")
	if err != nil {
		t.Fatalf("createSkillArchive second: %v", err)
	}
	defer os.Remove(second)
	firstBytes, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	secondBytes, err := os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatal("skill archive is not deterministic")
	}
	names := skillArchiveNames(t, first)
	want := []string{"demo-skill/SKILL.md", "demo-skill/references", "demo-skill/references/note.md"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("archive names = %#v, want %#v", names, want)
	}
}

func TestExtractSkillArchiveCreatesVersionedAndCompatibilityCopies(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "demo-skill")
	if err := os.MkdirAll(filepath.Join(skillDir, "references"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: demo-skill\ndescription: Demo skill.\n---\n# Demo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "references", "note.md"), []byte("hello\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	archive, err := createSkillArchive(skillDir, "demo-skill")
	if err != nil {
		t.Fatalf("createSkillArchive: %v", err)
	}
	defer os.Remove(archive)
	file, err := os.Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	agentsDir := filepath.Join(dir, ".agents")
	versionRoot := filepath.Join(agentsDir, "skills", "demo-skill", "0.1.0")
	if err := extractSkillArchive(file, versionRoot); err != nil {
		t.Fatalf("extractSkillArchive: %v", err)
	}
	skillRoot := filepath.Join(agentsDir, "skills", "demo-skill")
	if err := mirrorSkillVersion(versionRoot, skillRoot); err != nil {
		t.Fatalf("mirrorSkillVersion: %v", err)
	}
	for _, filename := range []string{
		filepath.Join(versionRoot, "SKILL.md"),
		filepath.Join(versionRoot, "references", "note.md"),
		filepath.Join(skillRoot, "SKILL.md"),
		filepath.Join(skillRoot, "references", "note.md"),
	} {
		if _, err := os.Stat(filename); err != nil {
			t.Fatalf("expected %s: %v", filename, err)
		}
	}
}

func TestMirrorLoopVersionCreatesCompatibilityCopy(t *testing.T) {
	dir := t.TempDir()
	versionPath := filepath.Join(dir, ".agents", "loops", "test", "0.1.0", "loop.yml")
	compatibilityPath := filepath.Join(dir, ".agents", "loops", "test", "loop.yml")
	content := []byte("kind: AgentLoop\n")
	if err := writeFileAtomic(versionPath, content); err != nil {
		t.Fatal(err)
	}
	if err := mirrorLoopVersion(versionPath, compatibilityPath); err != nil {
		t.Fatalf("mirrorLoopVersion: %v", err)
	}
	got, err := os.ReadFile(compatibilityPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Fatalf("compatibility content = %q, want %q", got, content)
	}
}

func TestInstallLoopArtifactWritesVersionAndSkipsMatchingDigest(t *testing.T) {
	agentsDir := filepath.Join(t.TempDir(), ".agents")
	loopBytes := []byte(`apiVersion: agent-loops.dev/v1alpha1
kind: AgentLoop
metadata:
  name: test
  version: 0.1.0
spec:
  objective: Verify install.
  dependencies:
    skills:
      - name: optional-skill
        ref: ghcr.io/acme/skills/optional:1.0.0
        required: false
phases:
  - name: inspect
    title: Inspect
    objective: Inspect.
    actions:
      - Read.
    completion:
      - Done.
    outputs:
      - status: ["completed"]
`)
	layer := ocispec.Descriptor{MediaType: loopLayerType, Digest: digest.FromBytes(loopBytes), Size: int64(len(loopBytes))}
	manifest := ocispec.Manifest{Layers: []ocispec.Descriptor{layer}}
	resolved := resolvedArtifact{
		ref:      "ghcr.io/acme/loops/test:0.1.0",
		registry: "ghcr.io",
		desc:     ocispec.Descriptor{Digest: digest.FromBytes([]byte("loop-manifest"))},
		manifest: &manifest,
		fetch: func(context.Context, ocispec.Descriptor) (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(loopBytes)), nil
		},
	}

	var result installResult
	if err := installLoopArtifact(context.Background(), options{agentsDir: agentsDir}, resolved, map[string]bool{}, &result, 0); err != nil {
		t.Fatalf("installLoopArtifact returned error: %v", err)
	}
	if len(result.Installed) != 1 || result.Installed[0].Kind != "loop" {
		t.Fatalf("installed records = %#v", result.Installed)
	}
	versionPath := filepath.Join(agentsDir, "loops", "test", "0.1.0", "loop.yml")
	compatibilityPath := filepath.Join(agentsDir, "loops", "test", "loop.yml")
	for _, filename := range []string{versionPath, compatibilityPath} {
		if _, err := os.Stat(filename); err != nil {
			t.Fatalf("expected %s: %v", filename, err)
		}
	}
	if err := writeAgentkitState(agentsDir, result); err != nil {
		t.Fatalf("writeAgentkitState returned error: %v", err)
	}

	var skipped installResult
	if err := installLoopArtifact(context.Background(), options{agentsDir: agentsDir}, resolved, map[string]bool{}, &skipped, 0); err != nil {
		t.Fatalf("second installLoopArtifact returned error: %v", err)
	}
	if len(skipped.Skipped) != 1 || skipped.Skipped[0].Path != versionPath {
		t.Fatalf("skipped records = %#v", skipped.Skipped)
	}
}

func TestInstallSkillArtifactWritesVersionAndSkipsMatchingDigest(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "demo-skill")
	if err := os.MkdirAll(skillDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: demo-skill\ndescription: Demo skill.\n---\n# Demo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	archive, err := createSkillArchive(skillDir, "demo-skill")
	if err != nil {
		t.Fatalf("createSkillArchive returned error: %v", err)
	}
	defer os.Remove(archive)
	archiveBytes, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	layer := ocispec.Descriptor{MediaType: skillLayerType, Digest: digest.FromBytes(archiveBytes), Size: int64(len(archiveBytes))}
	manifest := ocispec.Manifest{
		Annotations: map[string]string{skillNameAnnotation: "demo-skill"},
		Layers:      []ocispec.Descriptor{layer},
	}
	resolved := resolvedArtifact{
		ref:      "ghcr.io/acme/skills/demo-skill:2.0.0",
		registry: "ghcr.io",
		desc:     ocispec.Descriptor{Digest: digest.FromBytes([]byte("skill-manifest"))},
		manifest: &manifest,
		fetch: func(context.Context, ocispec.Descriptor) (io.ReadCloser, error) {
			file, err := os.Open(archive)
			if err != nil {
				return nil, err
			}
			return file, nil
		},
	}
	agentsDir := filepath.Join(dir, ".agents")
	var result installResult
	if err := installSkillArtifact(context.Background(), options{agentsDir: agentsDir}, resolved, &result, 1); err != nil {
		t.Fatalf("installSkillArtifact returned error: %v", err)
	}
	versionRoot := filepath.Join(agentsDir, "skills", "demo-skill", "2.0.0")
	compatibilityRoot := filepath.Join(agentsDir, "skills", "demo-skill")
	for _, filename := range []string{filepath.Join(versionRoot, "SKILL.md"), filepath.Join(compatibilityRoot, "SKILL.md")} {
		if _, err := os.Stat(filename); err != nil {
			t.Fatalf("expected %s: %v", filename, err)
		}
	}
	if result.Installed[0].Level != 1 {
		t.Fatalf("level = %d, want 1", result.Installed[0].Level)
	}
	if err := writeAgentkitState(agentsDir, result); err != nil {
		t.Fatalf("writeAgentkitState returned error: %v", err)
	}

	var skipped installResult
	if err := installSkillArtifact(context.Background(), options{agentsDir: agentsDir}, resolved, &skipped, 1); err != nil {
		t.Fatalf("second installSkillArtifact returned error: %v", err)
	}
	if len(skipped.Skipped) != 1 || skipped.Skipped[0].Path != versionRoot {
		t.Fatalf("skipped records = %#v", skipped.Skipped)
	}
}

func TestInstallAndCollectionHelpers(t *testing.T) {
	requiredFalse := false
	if !dependencyOptional(dependencySpec{Required: &requiredFalse}) {
		t.Fatal("expected dependency to be optional")
	}
	if dependencyOptional(dependencySpec{}) {
		t.Fatal("expected dependency without required=false to be required")
	}
	if collectionTypeForArtifact(skillArtifactType) != skillCollectionType {
		t.Fatal("expected skill collection type")
	}
	if collectionTypeForArtifact(loopArtifactType) != loopCollectionType {
		t.Fatal("expected loop collection type")
	}
	desc := ocispec.Descriptor{Annotations: map[string]string{loopRefAnnotation: "loop-ref", skillRefAnnotation: "skill-ref"}}
	if memberRef(loopArtifactType, desc) != "loop-ref" || memberRef(skillArtifactType, desc) != "skill-ref" {
		t.Fatalf("unexpected member refs")
	}
	if got := artifactInstallVersion("ghcr.io/acme/loops/test:1.2.3", "sha256:abc"); got != "1.2.3" {
		t.Fatalf("tag version = %q", got)
	}
	if got := artifactInstallVersion("ghcr.io/acme/loops/test@sha256:aaaaaaaa", "sha256:bbbbbbbb"); got != "sha256-aaaaaaaa" {
		t.Fatalf("digest version = %q", got)
	}
	if got := artifactInstallVersion("ghcr.io/acme/loops/test", "sha256:bbbbbbbb"); got != "sha256-bbbbbbbb" {
		t.Fatalf("resolved digest version = %q", got)
	}

	entries := collectionEntries(collectionFile{
		Items: []collectionEntry{{Name: "explicit", Ref: "ghcr.io/acme/loops/explicit:1.0.0"}},
		Refs:  []string{"ghcr.io/acme/loops/from-ref:2.0.0"},
	})
	if len(entries) != 2 || entries[1].Ref != "ghcr.io/acme/loops/from-ref:2.0.0" {
		t.Fatalf("entries = %#v", entries)
	}
	manifest := ocispec.Manifest{Annotations: map[string]string{skillNameAnnotation: "skill-name", loopNameAnnotation: "loop-name"}}
	skillAnnotations := descriptorAnnotations("skill", collectionEntry{Description: "desc"}, "ghcr.io/acme/skills/fallback:3.0.0", manifest)
	if skillAnnotations[skillNameAnnotation] != "skill-name" || skillAnnotations[skillRefAnnotation] == "" {
		t.Fatalf("skill annotations = %#v", skillAnnotations)
	}
	loopAnnotations := descriptorAnnotations("loop", collectionEntry{}, "ghcr.io/acme/loops/fallback:3.0.0", manifest)
	if loopAnnotations[loopNameAnnotation] != "loop-name" || loopAnnotations[loopRefAnnotation] == "" {
		t.Fatalf("loop annotations = %#v", loopAnnotations)
	}
	collection := collectionAnnotations(options{domain: "skill", registry: defaultRegistry, targetRef: "ghcr.io/acme/toolbox/skills:1.0.0"}, collectionFile{
		Title:       "Skills",
		Description: "Useful skills",
		Version:     "1.0.0",
	})
	if collection[ocispec.AnnotationTitle] != "Skills" || collection["io.agentskills.collection.name"] != "skills" || collection[sourceAnnotation] != "https://github.com/acme/toolbox" {
		t.Fatalf("collection annotations = %#v", collection)
	}
	if specsVersioned().SchemaVersion != 2 {
		t.Fatal("expected OCI schema version 2")
	}
}

func TestSkillMetadataAndAnnotations(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: demo\ndescription: Demo.\nlicense: MIT\ncompatibility: codex\nallowed-tools:\n  - shell\nmetadata:\n  owner: platform\n---\n# Demo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	meta, err := readSkillMetadata(dir)
	if err != nil {
		t.Fatalf("readSkillMetadata returned error: %v", err)
	}
	annotations := skillManifestAnnotations(options{registry: defaultRegistry, targetRef: "ghcr.io/acme/skills/demo:1.0.0"}, meta)
	if annotations[skillNameAnnotation] != "demo" || annotations[ocispec.AnnotationLicenses] != "MIT" || annotations["io.agentskills.skill.compatibility"] != "codex" {
		t.Fatalf("annotations = %#v", annotations)
	}

	noFrontmatter := filepath.Join(t.TempDir(), "plain")
	if err := os.MkdirAll(noFrontmatter, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(noFrontmatter, "SKILL.md"), []byte("# Demo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readSkillMetadata(noFrontmatter); err == nil || !strings.Contains(err.Error(), "frontmatter") {
		t.Fatalf("expected frontmatter error, got %v", err)
	}
	missingName := filepath.Join(t.TempDir(), "missing-name")
	if err := os.MkdirAll(missingName, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(missingName, "SKILL.md"), []byte("---\ndescription: Missing name.\n---\n# Demo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateSkillDir(missingName); err == nil || !strings.Contains(err.Error(), "must include name") {
		t.Fatalf("expected missing name error, got %v", err)
	}
	if err := validateSkillDir(filepath.Join(dir, "SKILL.md")); err == nil || !strings.Contains(err.Error(), "must be a directory") {
		t.Fatalf("expected directory error, got %v", err)
	}
}

func TestPublishArtifactsWithFakeRepository(t *testing.T) {
	repo := newFakeArtifactRepository()
	oldOpen := openRepository
	openRepository = func(opts options) (artifactRepository, string, error) {
		return repo, targetTag(opts.targetRef), nil
	}
	defer func() { openRepository = oldOpen }()

	dir := t.TempDir()
	loopFile := filepath.Join(dir, "loop.yml")
	if err := os.WriteFile(loopFile, []byte("kind: AgentLoop\nmetadata:\n  name: demo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	loopResult, err := pushPackage(context.Background(), options{
		domain:       "loop",
		filename:     loopFile,
		targetRef:    "ghcr.io/acme/toolbox/demo-loop:1.0.0",
		registry:     defaultRegistry,
		artifactType: loopArtifactType,
		layerType:    loopLayerType,
	})
	if err != nil {
		t.Fatalf("pushPackage returned error: %v", err)
	}
	if loopResult.digest == "" || repo.refs["1.0.0"].Digest.String() != loopResult.digest {
		t.Fatalf("loop result = %#v refs = %#v", loopResult, repo.refs)
	}

	var stdout bytes.Buffer
	if err := printPushOutcome(&stdout, options{targetRef: "ghcr.io/acme/toolbox/demo-loop:1.0.0"}, loopResult, nil); err != nil {
		t.Fatalf("printPushOutcome returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "pushed ghcr.io/acme/toolbox/demo-loop:1.0.0") {
		t.Fatalf("push output = %q", stdout.String())
	}

	skillDir := filepath.Join(dir, "demo-skill")
	if err := os.MkdirAll(skillDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: demo-skill\ndescription: Demo skill.\nlicense: MIT\n---\n# Demo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	skillResult, err := publishSkill(context.Background(), options{
		filename:     skillDir,
		targetRef:    "ghcr.io/acme/toolbox/demo-skill:2.0.0",
		registry:     defaultRegistry,
		artifactType: skillArtifactType,
		layerType:    skillLayerType,
	})
	if err != nil {
		t.Fatalf("publishSkill returned error: %v", err)
	}
	if skillResult.digest == "" || repo.refs["2.0.0"].Digest.String() != skillResult.digest {
		t.Fatalf("skill result = %#v refs = %#v", skillResult, repo.refs)
	}

	collectionFile := filepath.Join(dir, "skills.json")
	if err := os.WriteFile(collectionFile, []byte(`{"name":"skills","refs":["ghcr.io/acme/toolbox/demo-skill:2.0.0"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	collectionResult, err := publishCollection(context.Background(), options{
		domain:         "skill",
		filename:       collectionFile,
		targetRef:      "ghcr.io/acme/toolbox/skills:3.0.0",
		registry:       defaultRegistry,
		artifactType:   skillArtifactType,
		collectionType: skillCollectionType,
	})
	if err != nil {
		t.Fatalf("publishCollection returned error: %v", err)
	}
	if collectionResult.digest == "" || repo.refs["3.0.0"].Digest.String() != collectionResult.digest {
		t.Fatalf("collection result = %#v refs = %#v", collectionResult, repo.refs)
	}
}

func TestInstallReferenceResolvesLoopAndSkillDependenciesWithFakeRepository(t *testing.T) {
	repo := newFakeArtifactRepository()
	oldOpen := openRepository
	openRepository = func(opts options) (artifactRepository, string, error) {
		return repo, targetTag(opts.targetRef), nil
	}
	defer func() { openRepository = oldOpen }()

	dir := t.TempDir()
	skillDir := filepath.Join(dir, "demo-skill")
	if err := os.MkdirAll(skillDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: demo-skill\ndescription: Demo skill.\n---\n# Demo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	archive, err := createSkillArchive(skillDir, "demo-skill")
	if err != nil {
		t.Fatalf("createSkillArchive returned error: %v", err)
	}
	defer os.Remove(archive)
	archiveBytes, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	skillLayer := ocispec.Descriptor{MediaType: skillLayerType, Digest: digest.FromBytes(archiveBytes), Size: int64(len(archiveBytes))}
	repo.blobs[skillLayer.Digest] = archiveBytes
	skillManifest := ocispec.Manifest{
		ArtifactType: skillArtifactType,
		Annotations:  map[string]string{skillNameAnnotation: "demo-skill"},
		Layers:       []ocispec.Descriptor{skillLayer},
	}
	repo.addManifest("2.0.0", skillArtifactType, skillManifest)

	loopBytes := []byte(`apiVersion: agent-loops.dev/v1alpha1
kind: AgentLoop
metadata:
  name: demo-loop
spec:
  objective: Verify install.
  dependencies:
    skills:
      - name: demo-skill
        ref: ghcr.io/acme/toolbox/demo-skill:2.0.0
phases:
  - name: inspect
    title: Inspect
    objective: Inspect.
    actions:
      - Read.
    completion:
      - Done.
    outputs:
      - status: ["completed"]
`)
	loopLayer := ocispec.Descriptor{MediaType: loopLayerType, Digest: digest.FromBytes(loopBytes), Size: int64(len(loopBytes))}
	repo.blobs[loopLayer.Digest] = loopBytes
	loopManifest := ocispec.Manifest{
		ArtifactType: loopArtifactType,
		Annotations:  map[string]string{loopNameAnnotation: "demo-loop"},
		Layers:       []ocispec.Descriptor{loopLayer},
	}
	repo.addManifest("1.0.0", loopArtifactType, loopManifest)

	agentsDir := filepath.Join(dir, ".agents")
	result, err := installReference(context.Background(), options{
		agentsDir: agentsDir,
		targetRef: "ghcr.io/acme/toolbox/demo-loop:1.0.0",
		registry:  defaultRegistry,
	}, loopArtifactType)
	if err != nil {
		t.Fatalf("installReference returned error: %v", err)
	}
	if len(result.Installed) != 2 {
		t.Fatalf("installed records = %#v", result.Installed)
	}
	for _, filename := range []string{
		filepath.Join(agentsDir, "loops", "demo-loop", "1.0.0", "loop.yml"),
		filepath.Join(agentsDir, "skills", "demo-skill", "2.0.0", "SKILL.md"),
		filepath.Join(agentsDir, agentkitManifestName),
		filepath.Join(agentsDir, agentkitLockName),
	} {
		if _, err := os.Stat(filename); err != nil {
			t.Fatalf("expected %s: %v", filename, err)
		}
	}
	upToDate, err := installReference(context.Background(), options{
		agentsDir: agentsDir,
		targetRef: "ghcr.io/acme/toolbox/demo-loop:1.0.0",
		registry:  defaultRegistry,
	}, loopArtifactType)
	if err != nil {
		t.Fatalf("second installReference returned error: %v", err)
	}
	if len(upToDate.Skipped) != 2 {
		t.Fatalf("skipped records = %#v", upToDate.Skipped)
	}
}

func TestPullPackageWithFakeRepositoryWritesCacheAndOutput(t *testing.T) {
	repo := newFakeArtifactRepository()
	oldOpen := openRepository
	openRepository = func(opts options) (artifactRepository, string, error) {
		return repo, targetTag(opts.targetRef), nil
	}
	defer func() { openRepository = oldOpen }()
	t.Setenv("HOME", t.TempDir())

	loopBytes := []byte("kind: AgentLoop\nmetadata:\n  name: pulled\n")
	layer := ocispec.Descriptor{MediaType: loopLayerType, Digest: digest.FromBytes(loopBytes), Size: int64(len(loopBytes))}
	repo.blobs[layer.Digest] = loopBytes
	manifest := ocispec.Manifest{ArtifactType: loopArtifactType, Layers: []ocispec.Descriptor{layer}}
	repo.addManifest("1.0.0", loopArtifactType, manifest)

	output := filepath.Join(t.TempDir(), "loop.yml")
	result, err := pullPackage(context.Background(), options{
		targetRef: "ghcr.io/acme/toolbox/pulled:1.0.0",
		registry:  defaultRegistry,
		layerType: loopLayerType,
		output:    output,
	})
	if err != nil {
		t.Fatalf("pullPackage returned error: %v", err)
	}
	if !result.updated || result.cachePath == "" || result.manifestDigest == "" {
		t.Fatalf("pull result = %#v", result)
	}
	got, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(loopBytes) {
		t.Fatalf("output = %q, want %q", got, loopBytes)
	}

	second, err := pullPackage(context.Background(), options{
		targetRef: "ghcr.io/acme/toolbox/pulled:1.0.0",
		registry:  defaultRegistry,
		layerType: loopLayerType,
	})
	if err != nil {
		t.Fatalf("second pullPackage returned error: %v", err)
	}
	if second.updated {
		t.Fatal("expected cached pull to be up to date")
	}
}

func TestResolveArtifactSupportsCollectionsWithFakeRepository(t *testing.T) {
	repo := newFakeArtifactRepository()
	oldOpen := openRepository
	openRepository = func(opts options) (artifactRepository, string, error) {
		return repo, targetTag(opts.targetRef), nil
	}
	defer func() { openRepository = oldOpen }()

	index := ocispec.Index{
		ArtifactType: skillCollectionType,
		Manifests: []ocispec.Descriptor{{
			MediaType:    ocispec.MediaTypeImageManifest,
			ArtifactType: skillArtifactType,
			Digest:       digest.FromBytes([]byte("member")),
			Annotations:  map[string]string{skillRefAnnotation: "ghcr.io/acme/toolbox/demo:1.0.0"},
		}},
	}
	data, err := json.Marshal(index)
	if err != nil {
		t.Fatal(err)
	}
	desc := ocispec.Descriptor{MediaType: ocispec.MediaTypeImageIndex, ArtifactType: skillCollectionType, Digest: digest.FromBytes(data), Size: int64(len(data))}
	repo.refs["latest"] = desc
	repo.blobs[desc.Digest] = data

	resolved, err := resolveArtifact(context.Background(), "ghcr.io/acme/toolbox/skills:latest")
	if err != nil {
		t.Fatalf("resolveArtifact returned error: %v", err)
	}
	if resolved.index == nil || resolved.index.ArtifactType != skillCollectionType {
		t.Fatalf("resolved index = %#v", resolved.index)
	}
}

func TestSmallCoverageHelpers(t *testing.T) {
	if installRecordKey(installRecord{Kind: "loop", Name: "demo", Ref: "ref"}) != "loop\x00demo\x00ref" {
		t.Fatal("unexpected install record key")
	}
	if digestFromBytes([]byte("hello")) != digest.FromBytes([]byte("hello")) {
		t.Fatal("unexpected digest")
	}
	var stdout bytes.Buffer
	printHelp(&stdout, "loop push")
	if !strings.Contains(stdout.String(), "agentkit loop push") {
		t.Fatalf("loop push help = %q", stdout.String())
	}
	stdout.Reset()
	if err := printPushOutcome(&stdout, options{targetRef: "ghcr.io/acme/toolbox/demo:1.0.0"}, pushResult{digest: "sha256:abc", skipped: true}, nil); err != nil {
		t.Fatalf("printPushOutcome skipped returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "already up to date") {
		t.Fatalf("skipped push output = %q", stdout.String())
	}
	if err := printPushOutcome(&stdout, options{}, pushResult{}, errors.New("boom")); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected push error, got %v", err)
	}
	if got := targetDisplayReference("ghcr.io/acme/toolbox/demo@sha256:aaaaaaaa"); got != "sha256:aaaaaaaa" {
		t.Fatalf("digest display = %q", got)
	}
	if got := targetDisplayReference("ghcr.io/acme/toolbox/demo"); got != "latest" {
		t.Fatalf("default display = %q", got)
	}
}

func skillArchiveNames(t *testing.T, filename string) []string {
	t.Helper()
	file, err := os.Open(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var names []string
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, header.Name)
	}
	return names
}

func TestWriteAgentkitStateUsesAgentsDir(t *testing.T) {
	agentsDir := filepath.Join(t.TempDir(), ".custom-agents")
	result := installResult{Installed: []installRecord{
		{Kind: "loop", Name: "review", Ref: "ghcr.io/acme/loops/review:1.0.0", Digest: "sha256:abc", MediaType: loopArtifactType, Path: filepath.Join(agentsDir, "loops", "review", "1.0.0", "loop.yml")},
		{Kind: "skill", Name: "github", Ref: "ghcr.io/acme/skills/github:2.0.0", Digest: "sha256:def", MediaType: skillArtifactType, Path: filepath.Join(agentsDir, "skills", "github", "2.0.0")},
	}}
	if err := writeFileAtomic(result.Installed[0].Path, []byte("kind: AgentLoop\n")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(result.Installed[1].Path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeAgentkitState(agentsDir, result); err != nil {
		t.Fatalf("writeAgentkitState returned error: %v", err)
	}
	for _, name := range []string{agentkitManifestName, agentkitLockName} {
		if _, err := os.Stat(filepath.Join(agentsDir, name)); err != nil {
			t.Fatalf("expected %s under agents dir: %v", name, err)
		}
	}
	data, err := os.ReadFile(filepath.Join(agentsDir, agentkitManifestName))
	if err != nil {
		t.Fatal(err)
	}
	var manifest agentkitManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Loops[0].Ref != result.Installed[0].Ref {
		t.Fatalf("loop manifest ref = %q, want %q", manifest.Loops[0].Ref, result.Installed[0].Ref)
	}
	if manifest.Skills[0].Ref != result.Installed[1].Ref {
		t.Fatalf("skill manifest ref = %q, want %q", manifest.Skills[0].Ref, result.Installed[1].Ref)
	}
	if !installedDigestMatches(agentsDir, result.Installed[0]) {
		t.Fatal("expected installed loop digest to match lock")
	}
}

func TestRunValidatePrintsValidForLoopFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run(context.Background(), []string{"loop", "validate", filepath.Join("testdata", "valid_loop.yml")}, &stdout, &stderr)
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
	if !strings.Contains(string(data), "agentkit prime") {
		t.Fatalf("agents file does not mention agentkit prime:\n%s", string(data))
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
		{name: "help", args: []string{"help", "loop", "validate"}, want: "agentkit loop validate <loop.yml>"},
		{name: "quickstart", args: []string{"quickstart"}, want: "agentkit loop validate ./loop.yml"},
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
	if !strings.Contains(stderr.String(), "agentkit [command]") {
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
	err := run(context.Background(), []string{"loop", "push", badLoop, "ghcr.io/stumpyfr/agentkit/bad:v1"}, &stdout, &stderr)
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
	if !strings.Contains(stderr.String(), "agentkit [command]") {
		t.Fatalf("stderr missing usage:\n%s", stderr.String())
	}
}

func TestCacheHelpersWriteCopyAndMatchLayer(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.yml")
	dest := filepath.Join(dir, "dest.yml")
	copied := filepath.Join(dir, "copied.yml")
	content := []byte("apiVersion: agent-loops.dev/v1alpha1\n")

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

	got, err := cachedFilePath("ghcr.io/stumpyfr/agentkit/package:v1")
	if err != nil {
		t.Fatalf("cachedFilePath returned error: %v", err)
	}
	if !strings.Contains(got, filepath.Join("loop", "refs")+string(os.PathSeparator)) {
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
	err := run(context.Background(), []string{"loop", "render", "--details", filepath.Join("testdata", "valid_loop.yml")}, &stdout, &stderr)
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
	for _, topic := range []string{"", "loop", "loop render", "loop validate", "loop pull", "skill", "skill pull", "skill validate", "init", "quickstart", "prime", "help", "unknown"} {
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
	repo, err := newRemoteRepository(options{targetRef: "ghcr.io/stumpyfr/agentkit/review:v1", registry: "ghcr.io"})
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
		targetRef: "ghcr.io/stumpyfr/agentkit/missing:v1",
		registry:  "ghcr.io",
		layerType: defaultLayerType,
	})
	if err == nil || !strings.Contains(err.Error(), "add yaml layer") {
		t.Fatalf("expected local layer error, got %v", err)
	}
}

func TestRunPackageUsesCachedFile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ref := "ghcr.io/stumpyfr/agentkit/package:v1"
	cachePath, err := cachedFilePath(ref)
	if err != nil {
		t.Fatalf("cachedFilePath: %v", err)
	}
	if err := os.WriteFile(cachePath, []byte("kind: AgentLoop\n"), 0o600); err != nil {
		t.Fatalf("write cached package: %v", err)
	}

	var stdout bytes.Buffer
	err = runPackage(context.Background(), options{targetRef: ref}, &stdout)
	if err != nil {
		t.Fatalf("runPackage returned error: %v", err)
	}
	if got := stdout.String(); got != "kind: AgentLoop\n" {
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
	if err := os.WriteFile(source, []byte("kind: AgentLoop\n"), 0o600); err != nil {
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
	if err := os.WriteFile(source, []byte("kind: AgentLoop\n"), 0o600); err != nil {
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
	if manifest.ArtifactType != loopArtifactType {
		t.Fatalf("packed descriptor artifact type = %q, want %q", manifest.ArtifactType, loopArtifactType)
	}
	reader, err := storage.Fetch(context.Background(), manifest)
	if err != nil {
		t.Fatalf("fetch packed manifest: %v", err)
	}
	defer reader.Close()
	var packedManifest ocispec.Manifest
	if err := json.NewDecoder(reader).Decode(&packedManifest); err != nil {
		t.Fatalf("decode packed manifest: %v", err)
	}
	if packedManifest.Config.MediaType != loopArtifactType {
		t.Fatalf("packed manifest config media type = %q, want %q", packedManifest.Config.MediaType, loopArtifactType)
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

type fakeArtifactRepository struct {
	refs  map[string]ocispec.Descriptor
	blobs map[digest.Digest][]byte
}

func newFakeArtifactRepository() *fakeArtifactRepository {
	return &fakeArtifactRepository{
		refs:  map[string]ocispec.Descriptor{},
		blobs: map[digest.Digest][]byte{},
	}
}

func (r *fakeArtifactRepository) addManifest(reference string, artifactType string, manifest ocispec.Manifest) ocispec.Descriptor {
	data, err := json.Marshal(manifest)
	if err != nil {
		panic(err)
	}
	desc := ocispec.Descriptor{
		MediaType:    ocispec.MediaTypeImageManifest,
		ArtifactType: artifactType,
		Digest:       digest.FromBytes(data),
		Size:         int64(len(data)),
	}
	r.refs[reference] = desc
	r.blobs[desc.Digest] = data
	return desc
}

func (r *fakeArtifactRepository) Resolve(_ context.Context, reference string) (ocispec.Descriptor, error) {
	desc, ok := r.refs[reference]
	if !ok {
		return ocispec.Descriptor{}, errdef.ErrNotFound
	}
	return desc, nil
}

func (r *fakeArtifactRepository) Fetch(_ context.Context, desc ocispec.Descriptor) (io.ReadCloser, error) {
	data, ok := r.blobs[desc.Digest]
	if !ok {
		return nil, errdef.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (r *fakeArtifactRepository) Push(_ context.Context, desc ocispec.Descriptor, reader io.Reader) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	r.blobs[desc.Digest] = data
	return nil
}

func (r *fakeArtifactRepository) Tag(_ context.Context, desc ocispec.Descriptor, reference string) error {
	r.refs[reference] = desc
	return nil
}
