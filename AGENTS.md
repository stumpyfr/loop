<!-- BEGIN LOOP CLI INTEGRATION v:1 -->
## Loop CLI

This project uses **loop_cli** for OCI-backed YAML loop packages.

### Agent Rules

- Run `loop_cli prime` before executing a loop package.
- Use `loop_cli validate <loop.yml>` before packaging or publishing local loop files.
- Use `loop_cli pull <ref>` to cache a package and `loop_cli run <ref>` to print the loop YAML.
- When running a loop, act only as the orchestrator described by `loop_cli prime`.
- Do not copy the full prime instructions here; `loop_cli prime` is the source of current workflow guidance.

### Quick Reference

```bash
loop_cli prime
loop_cli validate ./loop.yml
loop_cli pull ghcr.io/owner/repo/package:tag
loop_cli run ghcr.io/owner/repo/package:tag
```
<!-- END LOOP CLI INTEGRATION -->
