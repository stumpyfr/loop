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

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gopkg.in/yaml.v3"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
)

type skillMetadata struct {
	Name          string            `yaml:"name" json:"name"`
	Description   string            `yaml:"description" json:"description,omitempty"`
	License       string            `yaml:"license" json:"license,omitempty"`
	Compatibility string            `yaml:"compatibility" json:"compatibility,omitempty"`
	AllowedTools  []string          `yaml:"allowed-tools" json:"allowedTools,omitempty"`
	Metadata      map[string]string `yaml:"metadata" json:"metadata,omitempty"`
}

type skillConfig struct {
	SchemaVersion string            `json:"schemaVersion"`
	Name          string            `json:"name"`
	Version       string            `json:"version,omitempty"`
	Description   string            `json:"description,omitempty"`
	License       string            `json:"license,omitempty"`
	Compatibility string            `json:"compatibility,omitempty"`
	AllowedTools  []string          `json:"allowedTools,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

func validateSkillDir(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("inspect skill directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("skill path must be a directory: %s", dir)
	}
	meta, err := readSkillMetadata(dir)
	if err != nil {
		return err
	}
	if strings.TrimSpace(meta.Name) == "" {
		return errors.New("skill SKILL.md frontmatter must include name")
	}
	return nil
}

func publishSkill(ctx context.Context, opts options) (pushResult, error) {
	meta, err := readSkillMetadata(opts.filename)
	if err != nil {
		return pushResult{}, err
	}
	if strings.TrimSpace(meta.Name) == "" {
		return pushResult{}, errors.New("skill SKILL.md frontmatter must include name")
	}
	archive, err := createSkillArchive(opts.filename, meta.Name)
	if err != nil {
		return pushResult{}, err
	}
	defer os.Remove(archive)

	storage, err := file.New("")
	if err != nil {
		return pushResult{}, fmt.Errorf("create local OCI storage: %w", err)
	}
	defer storage.Close()
	layer, err := storage.Add(ctx, meta.Name+".tar.gz", opts.layerType, archive)
	if err != nil {
		return pushResult{}, fmt.Errorf("add skill layer: %w", err)
	}
	layer.Annotations = map[string]string{ocispec.AnnotationTitle: meta.Name + ".tar.gz"}

	configBytes, err := json.Marshal(skillConfig{
		SchemaVersion: "1",
		Name:          meta.Name,
		Version:       targetTag(opts.targetRef),
		Description:   meta.Description,
		License:       meta.License,
		Compatibility: meta.Compatibility,
		AllowedTools:  meta.AllowedTools,
		Metadata:      meta.Metadata,
	})
	if err != nil {
		return pushResult{}, fmt.Errorf("marshal skill config: %w", err)
	}
	configFile, err := os.CreateTemp("", "agentkit-skill-config-*.json")
	if err != nil {
		return pushResult{}, fmt.Errorf("create skill config: %w", err)
	}
	if _, err := configFile.Write(configBytes); err != nil {
		configFile.Close()
		return pushResult{}, fmt.Errorf("write skill config: %w", err)
	}
	if err := configFile.Close(); err != nil {
		return pushResult{}, fmt.Errorf("close skill config: %w", err)
	}
	defer os.Remove(configFile.Name())
	config, err := storage.Add(ctx, "config.json", skillConfigType, configFile.Name())
	if err != nil {
		return pushResult{}, fmt.Errorf("add skill config: %w", err)
	}

	repo, reference, err := openRepository(opts)
	if err != nil {
		return pushResult{}, err
	}
	packOptions := oras.PackOptions{
		PackImageManifest:   true,
		ConfigDescriptor:    &config,
		ManifestAnnotations: skillManifestAnnotations(opts, meta),
	}
	manifest, err := oras.Pack(ctx, storage, opts.artifactType, []ocispec.Descriptor{layer}, packOptions)
	if err != nil {
		return pushResult{}, fmt.Errorf("pack skill artifact: %w", err)
	}
	if err := pushGraph(ctx, storage, repo, manifest, map[string]struct{}{}); err != nil {
		return pushResult{}, wrapUploadError(opts.registry, err)
	}
	if err := repo.Tag(ctx, manifest, reference); err != nil {
		return pushResult{}, fmt.Errorf("tag skill: %w", err)
	}
	return pushResult{digest: manifest.Digest.String()}, nil
}

func skillManifestAnnotations(opts options, meta skillMetadata) map[string]string {
	annotations := map[string]string{
		ocispec.AnnotationCreated: fixedCreatedTime,
		ocispec.AnnotationTitle:   meta.Name,
		ocispec.AnnotationVersion: targetTag(opts.targetRef),
		skillNameAnnotation:       meta.Name,
	}
	if meta.Description != "" {
		annotations[ocispec.AnnotationDescription] = meta.Description
	}
	if meta.License != "" {
		annotations[ocispec.AnnotationLicenses] = meta.License
	}
	if meta.Compatibility != "" {
		annotations["io.agentskills.skill.compatibility"] = meta.Compatibility
	}
	if sourceURL, ok := githubSourceURL(opts); ok {
		annotations[sourceAnnotation] = sourceURL
	}
	return annotations
}

func readSkillMetadata(dir string) (skillMetadata, error) {
	data, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return skillMetadata{}, fmt.Errorf("read SKILL.md: %w", err)
	}
	text := string(data)
	if !strings.HasPrefix(text, "---\n") {
		return skillMetadata{}, errors.New("SKILL.md must start with YAML frontmatter")
	}
	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return skillMetadata{}, errors.New("SKILL.md frontmatter is not closed")
	}
	var meta skillMetadata
	if err := yaml.Unmarshal([]byte(text[4:4+end]), &meta); err != nil {
		return skillMetadata{}, fmt.Errorf("parse SKILL.md frontmatter: %w", err)
	}
	return meta, nil
}

func createSkillArchive(dir string, skillName string) (string, error) {
	entries, err := skillArchiveEntries(dir)
	if err != nil {
		return "", err
	}
	out, err := os.CreateTemp("", "agentkit-skill-*.tar.gz")
	if err != nil {
		return "", fmt.Errorf("create skill archive: %w", err)
	}
	defer out.Close()

	gw, err := gzip.NewWriterLevel(out, gzip.BestCompression)
	if err != nil {
		return "", fmt.Errorf("create gzip writer: %w", err)
	}
	gw.Name = ""
	gw.ModTime = time.Unix(0, 0).UTC()
	tw := tar.NewWriter(gw)
	for _, entry := range entries {
		if err := addSkillArchiveEntry(tw, dir, skillName, entry); err != nil {
			tw.Close()
			gw.Close()
			return "", err
		}
	}
	if err := tw.Close(); err != nil {
		gw.Close()
		return "", fmt.Errorf("close tar archive: %w", err)
	}
	if err := gw.Close(); err != nil {
		return "", fmt.Errorf("close gzip archive: %w", err)
	}
	return out.Name(), nil
}

func skillArchiveEntries(dir string) ([]string, error) {
	var entries []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == dir {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == ".agents" || strings.HasPrefix(rel, ".agents/") || rel == ".git" || strings.HasPrefix(rel, ".git/") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		entries = append(entries, rel)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk skill directory: %w", err)
	}
	sort.Strings(entries)
	return entries, nil
}

func addSkillArchiveEntry(tw *tar.Writer, base string, skillName string, rel string) error {
	path := filepath.Join(base, filepath.FromSlash(rel))
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("stat skill file: %w", err)
	}
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return fmt.Errorf("create tar header: %w", err)
	}
	header.Name = filepath.ToSlash(filepath.Join(skillName, rel))
	header.ModTime = time.Unix(0, 0).UTC()
	header.AccessTime = time.Unix(0, 0).UTC()
	header.ChangeTime = time.Unix(0, 0).UTC()
	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("write tar header: %w", err)
	}
	if info.IsDir() {
		return nil
	}
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open skill file: %w", err)
	}
	defer file.Close()
	if _, err := io.Copy(tw, file); err != nil {
		return fmt.Errorf("write skill file to archive: %w", err)
	}
	return nil
}

func digestFromBytes(data []byte) digest.Digest {
	return digest.FromBytes(data)
}
