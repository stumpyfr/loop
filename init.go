package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	loopIntegrationBegin = "<!-- BEGIN LOOP CLI INTEGRATION v:1 -->"
	loopIntegrationEnd   = "<!-- END LOOP CLI INTEGRATION -->"
)

func updateAgentsFile(filename string) (string, error) {
	current, mode, exists, err := readOptionalFile(filename)
	if err != nil {
		return "", err
	}

	next, changed, err := updateAgentsContent(current)
	if err != nil {
		return "", err
	}
	if !changed {
		return "updated", nil
	}

	if err := os.WriteFile(filename, []byte(next), mode); err != nil {
		return "", fmt.Errorf("write %s: %w", filename, err)
	}
	if !exists {
		return "created", nil
	}
	return "updated", nil
}

func readOptionalFile(filename string) (string, os.FileMode, bool, error) {
	info, err := os.Stat(filename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", 0o644, false, nil
		}
		return "", 0, false, fmt.Errorf("stat %s: %w", filename, err)
	}
	if info.IsDir() {
		return "", 0, false, fmt.Errorf("%s is a directory", filename)
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return "", 0, false, fmt.Errorf("read %s: %w", filename, err)
	}
	return string(data), info.Mode().Perm(), true, nil
}

func updateAgentsContent(current string) (string, bool, error) {
	block := loopAgentsBlock()
	begin := strings.Index(current, loopIntegrationBegin)
	end := strings.Index(current, loopIntegrationEnd)

	switch {
	case begin >= 0 && end >= 0 && begin < end:
		end += len(loopIntegrationEnd)
		next := current[:begin] + block + current[end:]
		return next, next != current, nil
	case begin >= 0 || end >= 0:
		return "", false, errors.New("AGENTS.md has a partial loop_cli integration block")
	}

	if strings.TrimSpace(current) == "" {
		return block + "\n", true, nil
	}
	next := strings.TrimRight(current, "\n") + "\n\n" + block + "\n"
	return next, true, nil
}

func loopAgentsBlock() string {
	return strings.Join([]string{
		loopIntegrationBegin,
		"## Loop CLI",
		"",
		"This project uses **loop_cli** for OCI-backed YAML loop packages.",
		"",
		"### Agent Rules",
		"",
		"- Run `loop_cli prime` before executing a loop package.",
		"- Use `loop_cli validate <loop.yml>` before packaging or publishing local loop files.",
		"- Use `loop_cli pull <ref>` to cache a package and `loop_cli run <ref>` to print the loop YAML.",
		"- When running a loop, act only as the orchestrator described by `loop_cli prime`.",
		"- Do not copy the full prime instructions here; `loop_cli prime` is the source of current workflow guidance.",
		"",
		"### Quick Reference",
		"",
		"```bash",
		"loop_cli prime",
		"loop_cli validate ./loop.yml",
		"loop_cli render ./loop.yml",
		"loop_cli pull ghcr.io/owner/repo/package:tag",
		"loop_cli run ghcr.io/owner/repo/package:tag",
		"```",
		loopIntegrationEnd,
	}, "\n")
}
