package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	agentkitIntegrationBegin = "<!-- BEGIN AGENTKIT INTEGRATION v:1 -->"
	agentkitIntegrationEnd   = "<!-- END AGENTKIT INTEGRATION -->"
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
	block := agentkitAgentsBlock()
	begin := strings.Index(current, agentkitIntegrationBegin)
	end := strings.Index(current, agentkitIntegrationEnd)

	switch {
	case begin >= 0 && end >= 0 && begin < end:
		end += len(agentkitIntegrationEnd)
		next := current[:begin] + block + current[end:]
		return next, next != current, nil
	case begin >= 0 || end >= 0:
		return "", false, errors.New("AGENTS.md has a partial agentkit integration block")
	}

	if strings.TrimSpace(current) == "" {
		return block + "\n", true, nil
	}
	next := strings.TrimRight(current, "\n") + "\n\n" + block + "\n"
	return next, true, nil
}

func agentkitAgentsBlock() string {
	return strings.Join([]string{
		agentkitIntegrationBegin,
		"## Agentkit",
		"",
		"This project uses **agentkit** for OCI-backed Agent Loop and Agent Skill artifacts.",
		"",
		"### Agent Rules",
		"",
		"- Run `agentkit prime` before executing an Agent Loop artifact.",
		"- Use `agentkit loop validate <loop.yml>` before pushing local loop files.",
		"- Use `agentkit skill validate <skill-dir>` before pushing local skill directories.",
		"- Use `agentkit loop pull <ref>` and `agentkit skill pull <ref>` to pull artifacts into `.agents/`.",
		"- When running a loop, act only as the orchestrator described by `agentkit prime`.",
		"- Do not copy the full prime instructions here; `agentkit prime` is the source of current workflow guidance.",
		"",
		"### Quick Reference",
		"",
		"```bash",
		"agentkit prime",
		"agentkit loop validate ./loop.yml",
		"agentkit loop render ./loop.yml",
		"agentkit loop pull ghcr.io/owner/repo/package:tag",
		"agentkit skill pull ghcr.io/owner/repo/skill:tag",
		"```",
		agentkitIntegrationEnd,
	}, "\n")
}
