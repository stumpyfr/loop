<!-- BEGIN LOOP INTEGRATION v:1 -->
## Loop

This project uses **loop** for OCI-backed YAML loop packages.

### Agent Rules

- Run `loop prime` before executing a loop package.
- Use `loop validate <loop.yml>` before packaging or publishing local loop files.
- Use `loop pull <ref>` to cache a package and `loop run <ref>` to print the loop YAML.
- When running a loop, act only as the orchestrator described by `loop prime`.
- Do not copy the full prime instructions here; `loop prime` is the source of current workflow guidance.

### Quick Reference

```bash
loop prime
loop validate ./loop.yml
loop pull ghcr.io/owner/repo/package:tag
loop run ghcr.io/owner/repo/package:tag
```
<!-- END LOOP INTEGRATION -->
