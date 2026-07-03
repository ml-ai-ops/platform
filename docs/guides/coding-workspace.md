# AI coding and IDE workspace

Nexus treats `/workspace` as the primary development surface. JupyterLab and
the optional browser IDE mount the same persistent volume, so a project created
in either interface appears immediately in the other.

## Start the IDE

```bash
make ide-up
```

Open <http://localhost:13337> and use `IDE_PASSWORD` (the local default is
`mlaiops-local`). Stop it without deleting projects:

```bash
make ide-down
```

The IDE is code-server, the open-source browser build of VS Code. It is an
opt-in Compose profile and is not exposed by the public deployment overlay.

## Generate a starter

Run this in either the Jupyter terminal or the IDE terminal:

```bash
nexus scaffold churn-service \
  --template ml \
  --agent codex \
  --prompt "Train, evaluate, and expose a churn classifier with tests"
```

Templates are `python`, `ml`, `agent`, and `api`. Use `--agent none` for a
deterministic starter without an LLM, or choose `codex` or `claude`.

Other command-line agents can be connected without changing Nexus:

```bash
export NEXUS_CUSTOM_AGENT_COMMAND="your-agent --non-interactive"
nexus scaffold experiment --template ml --agent custom \
  --prompt "Build a reproducible classification experiment"
```

Nexus appends the task as the final command argument and runs it from the new
project directory. Only configure agents whose permission model you understand.

```bash
nexus agents
```

shows whether each CLI and its API credential are available.

## Direct agent use

Pinned Codex and Claude Code CLIs are installed in both development surfaces:

```bash
codex
claude
```

For unattended scaffolding, Nexus runs Codex with workspace-write sandboxing.
Claude Code starts in the generated project with edit acceptance. Review agent
changes and tests before committing.

## Security boundary

- Agent API keys are injected from `.env`; they are not written into projects.
- Agent configuration volumes persist login/config state separately from code.
- The IDE has password authentication and binds only to the configured local
  port. Never expose it directly to the internet.
- Jupyter and the IDE share source files, but only Jupyter receives FUSE and
  `SYS_ADMIN` for the object-store mount.

For production multi-user deployments, replace the single shared code-server
container with isolated Coder workspaces or per-user Kubernetes pods.
