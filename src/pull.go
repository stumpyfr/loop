package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
)

func pullPackage(ctx context.Context, opts options) (pullResult, error) {
	repo, reference, err := openRepository(opts)
	if err != nil {
		return pullResult{}, err
	}

	manifest, err := repo.Resolve(ctx, reference)
	if err != nil {
		return pullResult{}, wrapPullError(opts.registry, fmt.Errorf("resolve package: %w", err))
	}

	successors, err := content.Successors(ctx, repo, manifest)
	if err != nil {
		return pullResult{}, wrapPullError(opts.registry, fmt.Errorf("read manifest: %w", err))
	}

	layer, err := selectYAMLLayer(successors, opts.layerType)
	if err != nil {
		return pullResult{}, err
	}

	cachePath, err := cachedFilePath(opts.targetRef)
	if err != nil {
		return pullResult{}, err
	}

	result := pullResult{
		ref:            opts.targetRef,
		manifestDigest: manifest.Digest.String(),
		cachePath:      cachePath,
	}
	if cachedLayerMatches(cachePath, layer) {
		if opts.output != "" {
			if err := copyFile(cachePath, opts.output); err != nil {
				return pullResult{}, err
			}
		}
		return result, nil
	}

	reader, err := repo.Fetch(ctx, layer)
	if err != nil {
		return pullResult{}, wrapPullError(opts.registry, fmt.Errorf("fetch yaml layer: %w", err))
	}
	defer reader.Close()

	if err := writePulledFile(cachePath, reader); err != nil {
		return pullResult{}, err
	}
	if opts.output != "" {
		if err := copyFile(cachePath, opts.output); err != nil {
			return pullResult{}, err
		}
	}
	result.updated = true
	return result, nil
}

func runPackage(ctx context.Context, opts options, stdout io.Writer) error {
	cachePath, err := cachedFilePath(opts.targetRef)
	if err != nil {
		return err
	}
	if _, err := os.Stat(cachePath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect cache: %w", err)
		}
		if _, err := pullPackage(ctx, opts); err != nil {
			return err
		}
	}

	file, err := os.Open(cachePath)
	if err != nil {
		return fmt.Errorf("open cached package: %w", err)
	}
	defer file.Close()
	_, err = io.Copy(stdout, file)
	return err
}

func printPullResult(stdout io.Writer, result pullResult) {
	fmt.Fprintf(stdout, "%s: Pulling from %s\n", targetDisplayReference(result.ref), targetRepository(result.ref))
	fmt.Fprintf(stdout, "Digest: %s\n", result.manifestDigest)
	if result.updated {
		fmt.Fprintf(stdout, "Status: Downloaded newer image for %s\n", result.ref)
	} else {
		fmt.Fprintf(stdout, "Status: Image is up to date for %s\n", result.ref)
	}
	fmt.Fprintln(stdout, result.ref)
}

func targetDisplayReference(ref string) string {
	if tag := targetTag(ref); tag != "" {
		return tag
	}
	if digest := targetDigest(ref); digest != "" {
		return digest
	}
	return "latest"
}

func selectYAMLLayer(descs []ocispec.Descriptor, layerType string) (ocispec.Descriptor, error) {
	for _, desc := range descs {
		if desc.MediaType == layerType {
			return desc, nil
		}
	}
	for _, desc := range descs {
		title := strings.ToLower(desc.Annotations[ocispec.AnnotationTitle])
		if strings.HasSuffix(title, ".yaml") || strings.HasSuffix(title, ".yml") {
			return desc, nil
		}
	}
	return ocispec.Descriptor{}, errors.New("package does not contain a YAML layer")
}
