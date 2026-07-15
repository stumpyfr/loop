package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func writePulledFile(filename string, reader io.Reader) error {
	if strings.TrimSpace(filename) == "" {
		return errors.New("output path must not be empty")
	}
	out, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open output file: %w", err)
	}
	defer out.Close()
	if _, err := io.Copy(out, reader); err != nil {
		return fmt.Errorf("write output file: %w", err)
	}
	return nil
}

func copyFile(source string, dest string) error {
	in, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open cached file: %w", err)
	}
	defer in.Close()
	return writePulledFile(dest, in)
}

func cachedFilePath(ref string) (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("find user cache dir: %w", err)
	}
	sum := sha256.Sum256([]byte(ref))
	dir := filepath.Join(cacheDir, "loop", "refs")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create cache dir: %w", err)
	}
	return filepath.Join(dir, hex.EncodeToString(sum[:])+".yml"), nil
}

func cachedLayerMatches(filename string, layer ocispec.Descriptor) bool {
	file, err := os.Open(filename)
	if err != nil {
		return false
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil || info.Size() != layer.Size {
		return false
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return false
	}
	return "sha256:"+hex.EncodeToString(hasher.Sum(nil)) == layer.Digest.String()
}
