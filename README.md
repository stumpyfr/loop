# loop_cli

`loop_cli` validates a YAML file, packages it as an OCI artifact, and uploads it
to a Docker-compatible package registry.

## Usage

```bash
go run . push ./package.yml ghcr.io/github-owner/repo-name/package-name:latest
go run . init
go run . validate ./package.yml
go run . render ./package.yml
go run . pull ghcr.io/github-owner/repo-name/package-name:latest
go run . run ghcr.io/github-owner/repo-name/package-name:latest
go run . quickstart
go run . prime
```

Initialize agent instructions by creating or updating a managed `AGENTS.md`
block that points agents to `loop_cli prime`:

```bash
go run . init
```

Validate checks YAML syntax, duplicate mapping keys, and the embedded loop JSON
Schema:

```bash
go run . validate ./package.yml
```

Render displays phases, transitions, self-loops, and root escalation inputs as a
terminal flowchart:

```bash
go run . render ./package.yml
go run . render --no-color ./package.yml
go run . render --details ghcr.io/github-owner/repo-name/package-name:latest
```

Optional flags:

```bash
go run . \
  push \
  --artifact-type application/vnd.arkham.loop.package.v1+yaml \
  --layer-media-type application/vnd.arkham.loop.package.config.v1+yaml \
  ./package.yml \
  ghcr.io/github-owner/repo-name/package-name:v1.0.0
```

The target reference follows the same shape as `docker push`:

```text
registry/namespace/package_name:tag
```

For example, multiple packages can live under the same GitHub repository path:

```bash
go run . push ./package-one.yml ghcr.io/arkham-advisory/test-loophub/package-one:latest
go run . push ./package-two.yml ghcr.io/arkham-advisory/test-loophub/package-two:latest
```

Pulling stores the packaged YAML in the local cache and prints Docker-like
status output:

```bash
go run . pull ghcr.io/arkham-advisory/test-loophub/package-one:latest
```

Use `--output` to also copy it to a file:

```bash
go run . pull \
  --output package-one.yml \
  ghcr.io/arkham-advisory/test-loophub/package-one:latest
```

Running displays the cached YAML. If the package is not cached locally, it is
pulled first:

```bash
go run . run ghcr.io/arkham-advisory/test-loophub/package-one:latest
```

Quickstart prints a human-friendly getting-started guide:

```bash
go run . quickstart
```

Prime prints agent-facing workflow context for authenticating to the registry,
pulling packages, and displaying loop YAML:

```bash
go run . prime
```

This helps an agent handle prompts like:

```text
run the loop ghcr.io/Arkham-Advisory/test-loophub/test:0.1.0 with the jira ticket TEST-42
```

For GHCR targets in `ghcr.io/<owner>/<repo>/<package>:<tag>` form, the CLI adds
`org.opencontainers.image.source=https://github.com/<owner>/<repo>` to the OCI
manifest so GitHub can associate the package with the repository. Packages that
were already published without this metadata may need to be connected manually
from the organization package page, or republished with the annotation.

Authentication uses Docker-compatible registry credentials. For GHCR, login
before pushing:

```bash
docker login ghcr.io
```

For CI or non-Docker environments, GHCR credentials can also be supplied through
environment variables:

```bash
export GHCR_USERNAME=github-user-or-org
export GHCR_TOKEN=github-token-with-write-packages
go run . push ./package.yml ghcr.io/github-owner/repo-name/package-name:latest
```

In GitHub Actions, `GITHUB_ACTOR` and `GITHUB_TOKEN` are used automatically when
the `GHCR_*` variables are not set.

The `push` command requires exactly two positional parameters:

1. A `.yml` or `.yaml` file.
2. A tagged OCI package reference in `registry/namespace/package_name:tag` form.

The `validate` command requires one `.yml` or `.yaml` file and validates it
against the embedded loop schema. `push` runs the same validation before
packaging.

The `render` command requires one local `.yml`/`.yaml` file or tagged OCI
package reference. It validates the loop, then prints an ANSI-colored flowchart.
Use `--no-color` or `NO_COLOR=1` for plain output, and `--details` for compact
phase actions, completion, and outputs.

The `init` command takes no positional arguments and creates or updates a
managed Loop CLI block in `AGENTS.md`. Use `--agents-file <path>` to target a
different agent instruction file.

The `pull` command requires one tagged OCI package reference and accepts
`--output <file>` when you want to copy the cached YAML to a file.

The `run` command requires one tagged OCI package reference and prints the YAML
to stdout, pulling it first when it is not already cached.

The `help` command takes an optional command name and prints grouped command
help. Agent-oriented help points agents to `loop_cli prime` before executing a
loop package.

The `quickstart` command takes no arguments and prints instructions that another
human can follow.

The `prime` command takes no arguments and prints instructions that another
agent can follow, including how to resolve `spec.inputs` before execution and
run loop phases with sub-agents. It also describes the orchestrator role and
the `.loop/runs/yyyy/mm/dd/hh/mm` artifact layout for loop runs.
