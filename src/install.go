package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gopkg.in/yaml.v3"
	"oras.land/oras-go/v2/content"
)

type resolvedArtifact struct {
	ref      string
	registry string
	desc     ocispec.Descriptor
	manifest *ocispec.Manifest
	index    *ocispec.Index
	fetch    func(context.Context, ocispec.Descriptor) (io.ReadCloser, error)
}

type installResult struct {
	Installed []installRecord `json:"installed"`
	Skipped   []installRecord `json:"skipped"`
	Events    []installEvent  `json:"-"`
}

type installEvent struct {
	Record  installRecord
	Skipped bool
}

type installRecord struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Ref       string `json:"ref"`
	Digest    string `json:"digest"`
	MediaType string `json:"mediaType"`
	Path      string `json:"path"`
	Level     int    `json:"-"`
}

type agentkitManifest struct {
	Loops  []manifestEntry `json:"loops,omitempty"`
	Skills []manifestEntry `json:"skills,omitempty"`
}

type manifestEntry struct {
	Name string `json:"name"`
	Ref  string `json:"ref"`
}

type agentkitLock struct {
	GeneratedAt string          `json:"generatedAt"`
	Artifacts   []installRecord `json:"artifacts"`
}

type loopDependencySet struct {
	Loops  []dependencySpec `yaml:"loops"`
	Skills []dependencySpec `yaml:"skills"`
}

type dependencySpec struct {
	Name        string `yaml:"name"`
	Ref         string `yaml:"ref"`
	Required    *bool  `yaml:"required"`
	Description string `yaml:"description"`
}

func installReference(ctx context.Context, opts options, expectedArtifactType string) (installResult, error) {
	result := installResult{}
	seen := map[string]bool{}
	if err := installRef(ctx, opts, expectedArtifactType, seen, &result, 0); err != nil {
		return installResult{}, err
	}
	if err := writeAgentkitState(opts.agentsDir, result); err != nil {
		return installResult{}, err
	}
	return result, nil
}

func installRef(ctx context.Context, opts options, expectedArtifactType string, seen map[string]bool, result *installResult, level int) error {
	if seen[opts.targetRef] {
		return fmt.Errorf("dependency cycle detected at %s", opts.targetRef)
	}
	seen[opts.targetRef] = true
	defer delete(seen, opts.targetRef)

	resolved, err := resolveArtifact(ctx, opts.targetRef)
	if err != nil {
		return err
	}
	if resolved.index != nil {
		if resolved.index.ArtifactType != collectionTypeForArtifact(expectedArtifactType) {
			return fmt.Errorf("artifact %s is collection type %q, want %q", opts.targetRef, resolved.index.ArtifactType, collectionTypeForArtifact(expectedArtifactType))
		}
		for _, member := range resolved.index.Manifests {
			ref := memberRef(expectedArtifactType, member)
			if ref == "" {
				return fmt.Errorf("collection member %s has no installable ref annotation", member.Digest)
			}
			targetRef, registry, err := normalizeTargetRef(ref)
			if err != nil {
				return err
			}
			child := opts
			child.targetRef = targetRef
			child.registry = registry
			if err := installRef(ctx, child, expectedArtifactType, seen, result, level); err != nil {
				return err
			}
		}
		return nil
	}
	if resolved.manifest == nil {
		return fmt.Errorf("artifact %s is neither manifest nor index", opts.targetRef)
	}
	if !descriptorManifestMatchesArtifactType(resolved.desc, *resolved.manifest, expectedArtifactType) {
		return fmt.Errorf("artifact %s has type %q, want %q", opts.targetRef, descriptorManifestArtifactType(resolved.desc, *resolved.manifest), expectedArtifactType)
	}
	if expectedArtifactType == loopArtifactType {
		return installLoopArtifact(ctx, opts, resolved, seen, result, level)
	}
	return installSkillArtifact(ctx, opts, resolved, result, level)
}

func resolveArtifact(ctx context.Context, ref string) (resolvedArtifact, error) {
	targetRef, registry, err := normalizeTargetRef(ref)
	if err != nil {
		return resolvedArtifact{}, err
	}
	repo, reference, err := openRepository(options{targetRef: targetRef, registry: registry})
	if err != nil {
		return resolvedArtifact{}, err
	}
	desc, err := repo.Resolve(ctx, reference)
	if err != nil {
		return resolvedArtifact{}, wrapPullError(registry, fmt.Errorf("resolve artifact: %w", err))
	}
	if desc.MediaType == ocispec.MediaTypeImageIndex {
		index, err := fetchIndex(ctx, repo, desc)
		if err != nil {
			return resolvedArtifact{}, err
		}
		return resolvedArtifact{ref: targetRef, registry: registry, desc: desc, index: &index, fetch: repo.Fetch}, nil
	}
	manifest, err := fetchManifest(ctx, repo, desc)
	if err != nil {
		return resolvedArtifact{}, err
	}
	return resolvedArtifact{ref: targetRef, registry: registry, desc: desc, manifest: &manifest, fetch: repo.Fetch}, nil
}

func fetchManifest(ctx context.Context, repo artifactRepository, desc ocispec.Descriptor) (ocispec.Manifest, error) {
	reader, err := repo.Fetch(ctx, desc)
	if err != nil {
		return ocispec.Manifest{}, fmt.Errorf("fetch manifest: %w", err)
	}
	defer reader.Close()
	var manifest ocispec.Manifest
	if err := json.NewDecoder(reader).Decode(&manifest); err != nil {
		return ocispec.Manifest{}, fmt.Errorf("decode manifest: %w", err)
	}
	return manifest, nil
}

func fetchIndex(ctx context.Context, repo artifactRepository, desc ocispec.Descriptor) (ocispec.Index, error) {
	reader, err := repo.Fetch(ctx, desc)
	if err != nil {
		return ocispec.Index{}, fmt.Errorf("fetch index: %w", err)
	}
	defer reader.Close()
	var index ocispec.Index
	if err := json.NewDecoder(reader).Decode(&index); err != nil {
		return ocispec.Index{}, fmt.Errorf("decode index: %w", err)
	}
	return index, nil
}

func installLoopArtifact(ctx context.Context, opts options, resolved resolvedArtifact, seen map[string]bool, result *installResult, level int) error {
	layer, err := selectYAMLLayer(resolved.manifest.Layers, loopLayerType)
	if err != nil {
		return err
	}
	reader, err := resolved.fetch(ctx, layer)
	if err != nil {
		return wrapPullError(resolved.registry, fmt.Errorf("fetch loop layer: %w", err))
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("read loop layer: %w", err)
	}
	loop, err := parseLoopBytes(data)
	if err != nil {
		return err
	}
	name := loop.Metadata.Name
	if name == "" {
		name = packageNameFromRef(resolved.ref)
	}
	version := artifactInstallVersion(resolved.ref, resolved.desc.Digest.String())
	path := filepath.Join(opts.agentsDir, "loops", name, version, "loop.yml")
	record := installRecord{Kind: "loop", Name: name, Ref: resolved.ref, Digest: resolved.desc.Digest.String(), MediaType: loopArtifactType, Path: path, Level: level}
	if installedDigestMatches(opts.agentsDir, record) {
		if err := mirrorLoopVersion(path, filepath.Join(opts.agentsDir, "loops", name, "loop.yml")); err != nil {
			return err
		}
		result.Skipped = append(result.Skipped, record)
		result.Events = append(result.Events, installEvent{Record: record, Skipped: true})
	} else {
		if err := writeFileAtomic(path, data); err != nil {
			return err
		}
		if err := mirrorLoopVersion(path, filepath.Join(opts.agentsDir, "loops", name, "loop.yml")); err != nil {
			return err
		}
		result.Installed = append(result.Installed, record)
		result.Events = append(result.Events, installEvent{Record: record})
	}
	for _, dep := range loop.Spec.Dependencies.Loops {
		if dependencyOptional(dep) {
			continue
		}
		child, err := dependencyOptions(opts, dep)
		if err != nil {
			return err
		}
		if err := installRef(ctx, child, loopArtifactType, seen, result, level+1); err != nil {
			return err
		}
	}
	for _, dep := range loop.Spec.Dependencies.Skills {
		if dependencyOptional(dep) {
			continue
		}
		child, err := dependencyOptions(opts, dep)
		if err != nil {
			return err
		}
		if err := installRef(ctx, child, skillArtifactType, seen, result, level+1); err != nil {
			return err
		}
	}
	return nil
}

func installSkillArtifact(ctx context.Context, opts options, resolved resolvedArtifact, result *installResult, level int) error {
	layer, err := selectLayer(resolved.manifest.Layers, skillLayerType)
	if err != nil {
		return err
	}
	reader, err := resolved.fetch(ctx, layer)
	if err != nil {
		return wrapPullError(resolved.registry, fmt.Errorf("fetch skill layer: %w", err))
	}
	defer reader.Close()
	name := resolved.manifest.Annotations[skillNameAnnotation]
	if name == "" {
		name = packageNameFromRef(resolved.ref)
	}
	version := artifactInstallVersion(resolved.ref, resolved.desc.Digest.String())
	path := filepath.Join(opts.agentsDir, "skills", name, version)
	record := installRecord{Kind: "skill", Name: name, Ref: resolved.ref, Digest: resolved.desc.Digest.String(), MediaType: skillArtifactType, Path: path, Level: level}
	if installedDigestMatches(opts.agentsDir, record) {
		if err := mirrorSkillVersion(path, filepath.Join(opts.agentsDir, "skills", name)); err != nil {
			return err
		}
		result.Skipped = append(result.Skipped, record)
		result.Events = append(result.Events, installEvent{Record: record, Skipped: true})
		return nil
	}
	if err := extractSkillArchive(reader, path); err != nil {
		return err
	}
	if err := mirrorSkillVersion(path, filepath.Join(opts.agentsDir, "skills", name)); err != nil {
		return err
	}
	result.Installed = append(result.Installed, record)
	result.Events = append(result.Events, installEvent{Record: record})
	return nil
}

func parseLoopBytes(data []byte) (loopDefinition, error) {
	var loop loopDefinition
	if err := yaml.Unmarshal(data, &loop); err != nil {
		return loopDefinition{}, fmt.Errorf("parse loop yaml: %w", err)
	}
	return loop, nil
}

func dependencyOptions(opts options, dep dependencySpec) (options, error) {
	if dep.Ref == "" {
		return options{}, fmt.Errorf("dependency %q must include ref", dep.Name)
	}
	targetRef, registry, err := normalizeTargetRef(dep.Ref)
	if err != nil {
		return options{}, err
	}
	child := opts
	child.targetRef = targetRef
	child.registry = registry
	return child, nil
}

func dependencyOptional(dep dependencySpec) bool {
	return dep.Required != nil && !*dep.Required
}

func collectionTypeForArtifact(artifactType string) string {
	if artifactType == skillArtifactType {
		return skillCollectionType
	}
	return loopCollectionType
}

func memberRef(expectedArtifactType string, desc ocispec.Descriptor) string {
	if expectedArtifactType == skillArtifactType {
		return desc.Annotations[skillRefAnnotation]
	}
	return desc.Annotations[loopRefAnnotation]
}

func descriptorManifestMatchesArtifactType(desc ocispec.Descriptor, manifest ocispec.Manifest, expectedArtifactType string) bool {
	if descriptorManifestArtifactType(desc, manifest) == expectedArtifactType {
		return true
	}
	switch expectedArtifactType {
	case loopArtifactType:
		_, err := selectYAMLLayer(manifest.Layers, loopLayerType)
		return err == nil
	case skillArtifactType:
		_, err := selectLayer(manifest.Layers, skillLayerType)
		return err == nil
	default:
		return false
	}
}

func descriptorManifestArtifactType(desc ocispec.Descriptor, manifest ocispec.Manifest) string {
	if desc.ArtifactType != "" {
		return desc.ArtifactType
	}
	if manifest.ArtifactType != "" {
		return manifest.ArtifactType
	}
	return manifest.Config.MediaType
}

func selectLayer(descs []ocispec.Descriptor, layerType string) (ocispec.Descriptor, error) {
	for _, desc := range descs {
		if desc.MediaType == layerType {
			return desc, nil
		}
	}
	return ocispec.Descriptor{}, fmt.Errorf("artifact does not contain layer %s", layerType)
}

func writeFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}

func artifactInstallVersion(ref string, resolvedDigest string) string {
	if tag := targetTag(ref); tag != "" {
		return tag
	}
	if digestRef := targetDigest(ref); digestRef != "" {
		return strings.ReplaceAll(digestRef, ":", "-")
	}
	return strings.ReplaceAll(resolvedDigest, ":", "-")
}

func mirrorLoopVersion(versionPath string, compatibilityPath string) error {
	data, err := os.ReadFile(versionPath)
	if err != nil {
		return fmt.Errorf("read loop version: %w", err)
	}
	return writeFileAtomic(compatibilityPath, data)
}

func extractSkillArchive(reader io.Reader, targetRoot string) error {
	gz, err := gzip.NewReader(reader)
	if err != nil {
		return fmt.Errorf("open skill archive: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	targetRoot = filepath.Clean(targetRoot)
	if err := os.RemoveAll(targetRoot); err != nil {
		return fmt.Errorf("remove existing skill: %w", err)
	}
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read skill archive: %w", err)
		}
		parts := strings.Split(filepath.ToSlash(header.Name), "/")
		if len(parts) < 2 {
			continue
		}
		rel := filepath.Join(parts[1:]...)
		dest := filepath.Clean(filepath.Join(targetRoot, rel))
		if !strings.HasPrefix(dest, targetRoot) {
			return fmt.Errorf("skill archive path escapes target: %s", header.Name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(dest, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("create skill dir: %w", err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
				return fmt.Errorf("create skill file dir: %w", err)
			}
			out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("create skill file: %w", err)
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return fmt.Errorf("write skill file: %w", err)
			}
			if err := out.Close(); err != nil {
				return fmt.Errorf("close skill file: %w", err)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(targetRoot, "SKILL.md")); err != nil {
		return fmt.Errorf("installed skill missing SKILL.md: %w", err)
	}
	return nil
}

func mirrorSkillVersion(versionRoot string, skillRoot string) error {
	versionRoot = filepath.Clean(versionRoot)
	skillRoot = filepath.Clean(skillRoot)
	entries, err := os.ReadDir(versionRoot)
	if err != nil {
		return fmt.Errorf("read skill version: %w", err)
	}
	if err := os.MkdirAll(skillRoot, 0o700); err != nil {
		return fmt.Errorf("create skill compatibility root: %w", err)
	}
	for _, entry := range entries {
		source := filepath.Join(versionRoot, entry.Name())
		dest := filepath.Join(skillRoot, entry.Name())
		if filepath.Clean(dest) == versionRoot {
			continue
		}
		if err := copyPath(source, dest); err != nil {
			return err
		}
	}
	return nil
}

func copyPath(source string, dest string) error {
	info, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("stat skill source: %w", err)
	}
	if info.IsDir() {
		if err := os.RemoveAll(dest); err != nil {
			return fmt.Errorf("remove skill compatibility dir: %w", err)
		}
		if err := os.MkdirAll(dest, info.Mode()); err != nil {
			return fmt.Errorf("create skill compatibility dir: %w", err)
		}
		entries, err := os.ReadDir(source)
		if err != nil {
			return fmt.Errorf("read skill compatibility dir: %w", err)
		}
		for _, entry := range entries {
			if err := copyPath(filepath.Join(source, entry.Name()), filepath.Join(dest, entry.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return fmt.Errorf("create skill compatibility file dir: %w", err)
	}
	in, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open skill source: %w", err)
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return fmt.Errorf("create skill compatibility file: %w", err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return fmt.Errorf("copy skill compatibility file: %w", err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("close skill compatibility file: %w", err)
	}
	return nil
}

func writeAgentkitState(agentsDir string, result installResult) error {
	records := append([]installRecord{}, result.Installed...)
	records = append(records, result.Skipped...)
	records = mergeWithExistingLock(agentsDir, records)
	sort.Slice(records, func(i, j int) bool {
		if records[i].Kind == records[j].Kind {
			return records[i].Name < records[j].Name
		}
		return records[i].Kind < records[j].Kind
	})
	manifest := agentkitManifest{}
	for _, record := range records {
		entry := manifestEntry{Name: record.Name, Ref: record.Ref}
		if record.Kind == "loop" {
			manifest.Loops = append(manifest.Loops, entry)
		} else if record.Kind == "skill" {
			manifest.Skills = append(manifest.Skills, entry)
		}
	}
	if err := writeJSON(filepath.Join(agentsDir, agentkitManifestName), manifest); err != nil {
		return err
	}
	lock := agentkitLock{GeneratedAt: time.Now().UTC().Format(time.RFC3339), Artifacts: records}
	return writeJSON(filepath.Join(agentsDir, agentkitLockName), lock)
}

func mergeWithExistingLock(agentsDir string, records []installRecord) []installRecord {
	data, err := os.ReadFile(filepath.Join(agentsDir, agentkitLockName))
	if err != nil {
		return records
	}
	var lock agentkitLock
	if err := json.Unmarshal(data, &lock); err != nil {
		return records
	}
	byKey := map[string]installRecord{}
	for _, record := range lock.Artifacts {
		byKey[installRecordKey(record)] = record
	}
	for _, record := range records {
		byKey[installRecordKey(record)] = record
	}
	merged := make([]installRecord, 0, len(byKey))
	for _, record := range byKey {
		merged = append(merged, record)
	}
	return merged
}

func installRecordKey(record installRecord) string {
	return record.Kind + "\x00" + record.Name + "\x00" + record.Ref
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	data = append(data, '\n')
	return writeFileAtomic(path, data)
}

func installedDigestMatches(agentsDir string, record installRecord) bool {
	lockPath := filepath.Join(agentsDir, agentkitLockName)
	data, err := os.ReadFile(lockPath)
	if err != nil {
		return false
	}
	var lock agentkitLock
	if err := json.Unmarshal(data, &lock); err != nil {
		return false
	}
	for _, existing := range lock.Artifacts {
		if existing.Kind == record.Kind && existing.Name == record.Name && existing.Ref == record.Ref && existing.Digest == record.Digest && existing.Path == record.Path {
			if _, err := os.Stat(record.Path); err != nil {
				return false
			}
			return true
		}
	}
	return false
}

func printInstallResult(stdout io.Writer, result installResult) {
	if len(result.Events) == 0 {
		for _, record := range result.Installed {
			result.Events = append(result.Events, installEvent{Record: record})
		}
		for _, record := range result.Skipped {
			result.Events = append(result.Events, installEvent{Record: record, Skipped: true})
		}
	}
	for _, event := range result.Events {
		status := "installed"
		if event.Skipped {
			status = "already up to date"
		}
		record := event.Record
		fmt.Fprintf(stdout, "%s%s %s %s -> %s\n", installIndent(record), status, record.Kind, record.Name, record.Path)
	}
}

func installIndent(record installRecord) string {
	if record.Level <= 0 {
		return ""
	}
	return strings.Repeat("  ", record.Level)
}

var _ = content.Successors
