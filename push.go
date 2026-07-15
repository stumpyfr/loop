package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/errdef"
	"oras.land/oras-go/v2/registry/remote"
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

	repo, err := newRemoteRepository(opts)
	if err != nil {
		return pushResult{}, err
	}
	existingManifest, matches, err := remoteYAMLLayerMatches(ctx, repo, opts, layer)
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
	if err := repo.Tag(ctx, manifest, repo.Reference.Reference); err != nil {
		return pushResult{}, fmt.Errorf("tag package: %w", err)
	}
	return pushResult{digest: manifest.Digest.String()}, nil
}

func manifestAnnotations(opts options) map[string]string {
	annotations := map[string]string{
		ocispec.AnnotationCreated: fixedCreatedTime,
	}
	if sourceURL, ok := githubSourceURL(opts); ok {
		annotations[sourceAnnotation] = sourceURL
	}
	return annotations
}

func remoteYAMLLayerMatches(ctx context.Context, repo *remote.Repository, opts options, localLayer ocispec.Descriptor) (ocispec.Descriptor, bool, error) {
	manifest, err := repo.Resolve(ctx, repo.Reference.Reference)
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
