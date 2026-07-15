package main

import (
	"fmt"
	"strings"
)

func wrapUploadError(registry string, err error) error {
	if strings.EqualFold(strings.TrimSuffix(registry, "/"), defaultRegistry) && strings.Contains(err.Error(), "response status code 403") {
		return fmt.Errorf("upload package: %w\nGHCR denied the upload. Authenticate with a token that has package write access, for example: docker login ghcr.io, or set GHCR_USERNAME and GHCR_TOKEN", err)
	}
	return fmt.Errorf("upload package: %w", err)
}

func wrapPullError(registry string, err error) error {
	if strings.EqualFold(strings.TrimSuffix(registry, "/"), defaultRegistry) && strings.Contains(err.Error(), "response status code 403") {
		return fmt.Errorf("%w\nGHCR denied the pull. Authenticate with a token that has package read access, for example: docker login ghcr.io, or set GHCR_USERNAME and GHCR_TOKEN", err)
	}
	return err
}
