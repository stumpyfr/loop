<!-- BEGIN AGENTKIT INTEGRATION v:1 -->
## Agentkit

This project uses **agentkit** for OCI-backed Agent Loop and Agent Skill artifacts.

### Agent Rules

- Run `agentkit prime` before executing an Agent Loop artifact.
- Use `agentkit loop validate <loop.yml>` before pushing local loop files.
- Use `agentkit skill validate <skill-dir>` before pushing local skill directories.
- Use `agentkit loop pull <ref>` and `agentkit skill pull <ref>` to pull artifacts into `.agents/`.
- When running a loop, act only as the orchestrator described by `agentkit prime`.
- Do not copy the full prime instructions here; `agentkit prime` is the source of current workflow guidance.

### Quick Reference

```bash
agentkit prime
agentkit loop validate ./loop.yml
agentkit loop render ./loop.yml
agentkit loop pull ghcr.io/owner/repo/package:tag
agentkit skill pull ghcr.io/owner/repo/skill:tag
```
<!-- END AGENTKIT INTEGRATION -->
