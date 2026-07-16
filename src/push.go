package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/errdef"
)

func pushPackage(ctx context.Context, opts options) (pushResult, error) {
	storage, err := file.New("")
	if err != nil {
		return pushResult{}, fmt.Errorf("create local OCI storage: %w", err)
	}
	defer storage.Close()

	layer, err := storage.Add(ctx, filepath.Base(opts.filename), opts.layerType, opts.filename)
	if err != nil {
		return pushResult{}, fmt.Errorf("add yaml layer: %w", err)
	}
	layer.Annotations = map[string]string{
		ocispec.AnnotationTitle: filepath.Base(opts.filename),
	}

	repo, reference, err := openRepository(opts)
	if err != nil {
		return pushResult{}, err
	}
	existingManifest, matches, err := remoteYAMLLayerMatches(ctx, repo, reference, opts, layer)
	if err != nil {
		return pushResult{}, wrapUploadError(opts.registry, err)
	}
	if matches {
		return pushResult{digest: existingManifest.Digest.String(), skipped: true}, nil
	}

	packOptions := oras.PackOptions{
		PackImageManifest:   true,
		ManifestAnnotations: manifestAnnotations(opts),
	}

	manifest, err := oras.Pack(ctx, storage, opts.artifactType, []ocispec.Descriptor{layer}, packOptions)
	if err != nil {
		return pushResult{}, fmt.Errorf("pack oci artifact: %w", err)
	}

	if err := pushGraph(ctx, storage, repo, manifest, map[string]struct{}{}); err != nil {
		return pushResult{}, wrapUploadError(opts.registry, err)
	}
	if err := repo.Tag(ctx, manifest, reference); err != nil {
		return pushResult{}, fmt.Errorf("tag package: %w", err)
	}
	return pushResult{digest: manifest.Digest.String()}, nil
}

type collectionFile struct {
	Name        string            `json:"name"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Version     string            `json:"version"`
	Refs        []string          `json:"refs"`
	Items       []collectionEntry `json:"items"`
}

type collectionEntry struct {
	Name        string `json:"name"`
	Ref         string `json:"ref"`
	Description string `json:"description"`
}

func publishCollection(ctx context.Context, opts options) (pushResult, error) {
	data, err := os.ReadFile(opts.filename)
	if err != nil {
		return pushResult{}, fmt.Errorf("read collection file: %w", err)
	}
	var collection collectionFile
	if err := json.Unmarshal(data, &collection); err != nil {
		return pushResult{}, fmt.Errorf("parse collection file: %w", err)
	}
	entries := collectionEntries(collection)
	if len(entries) == 0 {
		return pushResult{}, errors.New("collection must contain refs or items")
	}

	descriptors := make([]ocispec.Descriptor, 0, len(entries))
	for _, entry := range entries {
		ref := entry.Ref
		targetRef, registry, err := normalizeTargetRef(ref)
		if err != nil {
			return pushResult{}, fmt.Errorf("invalid collection ref %q: %w", ref, err)
		}
		repo, reference, err := openRepository(options{targetRef: targetRef, registry: registry})
		if err != nil {
			return pushResult{}, err
		}
		desc, err := repo.Resolve(ctx, reference)
		if err != nil {
			return pushResult{}, wrapPullError(registry, fmt.Errorf("resolve collection member %s: %w", targetRef, err))
		}
		manifest, err := fetchManifest(ctx, repo, desc)
		if err != nil {
			return pushResult{}, err
		}
		expected := opts.artifactType
		if opts.domain == "loop" {
			expected = loopArtifactType
		} else if opts.domain == "skill" {
			expected = skillArtifactType
		}
		if !descriptorManifestMatchesArtifactType(desc, manifest, expected) {
			return pushResult{}, fmt.Errorf("collection member %s has artifact type %q, want %q", targetRef, descriptorManifestArtifactType(desc, manifest), expected)
		}
		desc.MediaType = ocispec.MediaTypeImageManifest
		desc.ArtifactType = descriptorManifestArtifactType(desc, manifest)
		desc.Annotations = descriptorAnnotations(opts.domain, entry, targetRef, manifest)
		descriptors = append(descriptors, desc)
	}

	index := ocispec.Index{
		Versioned:    specsVersioned(),
		MediaType:    ocispec.MediaTypeImageIndex,
		ArtifactType: opts.collectionType,
		Manifests:    descriptors,
		Annotations:  collectionAnnotations(opts, collection),
	}
	indexBytes, err := json.Marshal(index)
	if err != nil {
		return pushResult{}, fmt.Errorf("marshal collection index: %w", err)
	}
	desc := ocispec.Descriptor{
		MediaType:    ocispec.MediaTypeImageIndex,
		ArtifactType: opts.collectionType,
		Digest:       digest.FromBytes(indexBytes),
		Size:         int64(len(indexBytes)),
	}
	repo, _, err := openRepository(opts)
	if err != nil {
		return pushResult{}, err
	}
	if err := repo.Push(ctx, desc, strings.NewReader(string(indexBytes))); err != nil && !errors.Is(err, errdef.ErrAlreadyExists) {
		return pushResult{}, wrapUploadError(opts.registry, fmt.Errorf("push collection index: %w", err))
	}
	if err := repo.Tag(ctx, desc, targetTag(opts.targetRef)); err != nil {
		return pushResult{}, fmt.Errorf("tag collection: %w", err)
	}
	return pushResult{digest: desc.Digest.String()}, nil
}

func collectionEntries(collection collectionFile) []collectionEntry {
	entries := append([]collectionEntry{}, collection.Items...)
	for _, ref := range collection.Refs {
		entries = append(entries, collectionEntry{Ref: ref})
	}
	return entries
}

func descriptorAnnotations(domain string, entry collectionEntry, ref string, manifest ocispec.Manifest) map[string]string {
	name := strings.TrimSpace(entry.Name)
	if name == "" {
		if domain == "skill" {
			name = manifest.Annotations[skillNameAnnotation]
		} else {
			name = manifest.Annotations[loopNameAnnotation]
		}
	}
	if name == "" {
		name = packageNameFromRef(ref)
	}
	annotations := map[string]string{
		ocispec.AnnotationTitle:   name,
		ocispec.AnnotationVersion: targetTag(ref),
	}
	if entry.Description != "" {
		annotations[ocispec.AnnotationDescription] = entry.Description
	}
	if domain == "skill" {
		annotations[skillNameAnnotation] = name
		annotations[skillRefAnnotation] = ref
	} else {
		annotations[loopNameAnnotation] = name
		annotations[loopRefAnnotation] = ref
	}
	return annotations
}

func collectionAnnotations(opts options, collection collectionFile) map[string]string {
	name := strings.TrimSpace(collection.Name)
	if name == "" {
		name = packageNameFromRef(opts.targetRef)
	}
	annotations := map[string]string{
		ocispec.AnnotationCreated: fixedCreatedTime,
		ocispec.AnnotationTitle:   name,
	}
	if collection.Title != "" {
		annotations[ocispec.AnnotationTitle] = collection.Title
	}
	if collection.Description != "" {
		annotations[ocispec.AnnotationDescription] = collection.Description
	}
	if collection.Version != "" {
		annotations[ocispec.AnnotationVersion] = collection.Version
	}
	if opts.domain == "skill" {
		annotations["io.agentskills.collection.name"] = name
	} else {
		annotations["io.agentloops.collection.name"] = name
	}
	if sourceURL, ok := githubSourceURL(opts); ok {
		annotations[sourceAnnotation] = sourceURL
	}
	return annotations
}

func specsVersioned() specs.Versioned {
	return specs.Versioned{SchemaVersion: 2}
}

func manifestAnnotations(opts options) map[string]string {
	annotations := map[string]string{
		ocispec.AnnotationCreated: fixedCreatedTime,
		ocispec.AnnotationTitle:   packageNameFromRef(opts.targetRef),
	}
	if opts.domain == "loop" {
		annotations[loopNameAnnotation] = packageNameFromRef(opts.targetRef)
	}
	if sourceURL, ok := githubSourceURL(opts); ok {
		annotations[sourceAnnotation] = sourceURL
	}
	return annotations
}

func packageNameFromRef(ref string) string {
	name := targetName(ref)
	parts := strings.Split(name, "/")
	if len(parts) == 0 {
		return name
	}
	return parts[len(parts)-1]
}

func remoteYAMLLayerMatches(ctx context.Context, repo artifactRepository, reference string, opts options, localLayer ocispec.Descriptor) (ocispec.Descriptor, bool, error) {
	manifest, err := repo.Resolve(ctx, reference)
	if err != nil {
		if errors.Is(err, errdef.ErrNotFound) {
			return ocispec.Descriptor{}, false, nil
		}
		return ocispec.Descriptor{}, false, fmt.Errorf("resolve existing package: %w", err)
	}

	successors, err := content.Successors(ctx, repo, manifest)
	if err != nil {
		return ocispec.Descriptor{}, false, fmt.Errorf("read existing manifest: %w", err)
	}
	remoteLayer, err := selectYAMLLayer(successors, opts.layerType)
	if err != nil {
		return manifest, false, nil
	}
	return manifest, sameContent(remoteLayer, localLayer), nil
}

func sameContent(a ocispec.Descriptor, b ocispec.Descriptor) bool {
	return a.Digest == b.Digest && a.Size == b.Size
}

func githubSourceURL(opts options) (string, bool) {
	if !strings.EqualFold(strings.TrimSuffix(opts.registry, "/"), defaultRegistry) {
		return "", false
	}

	name := targetName(opts.targetRef)
	parts := strings.Split(name, "/")
	if len(parts) < 4 {
		return "", false
	}
	return fmt.Sprintf("https://github.com/%s/%s", parts[1], parts[2]), true
}

func pushGraph(ctx context.Context, src content.ReadOnlyStorage, dst content.Pusher, desc ocispec.Descriptor, pushed map[string]struct{}) error {
	key := desc.Digest.String()
	if _, ok := pushed[key]; ok {
		return nil
	}

	successors, err := content.Successors(ctx, src, desc)
	if err != nil {
		return fmt.Errorf("find successors for %s: %w", desc.Digest, err)
	}
	for _, successor := range successors {
		if err := pushGraph(ctx, src, dst, successor, pushed); err != nil {
			return err
		}
	}

	reader, err := src.Fetch(ctx, desc)
	if err != nil {
		return fmt.Errorf("fetch %s: %w", desc.Digest, err)
	}
	defer reader.Close()

	if err := dst.Push(ctx, desc, reader); err != nil && !errors.Is(err, errdef.ErrAlreadyExists) {
		return fmt.Errorf("push %s: %w", desc.Digest, err)
	}
	pushed[key] = struct{}{}
	return nil
}
