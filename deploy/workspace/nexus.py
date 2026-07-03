#!/usr/bin/env python3
"""Nexus workspace scaffolder and coding-agent launcher."""

from __future__ import annotations

import argparse
import os
import re
import shlex
import shutil
import subprocess
from pathlib import Path

WORKSPACE = Path(os.getenv("NEXUS_WORKSPACE", "/workspace")).resolve()


def slug(value: str) -> str:
    cleaned = re.sub(r"[^a-z0-9]+", "-", value.lower()).strip("-")
    if not cleaned:
        raise ValueError("project name must contain letters or numbers")
    return cleaned


def write_once(path: Path, content: str) -> None:
    if not path.exists():
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content, encoding="utf-8")


def create_base(target: Path, name: str, template: str, prompt: str) -> None:
    package = name.replace("-", "_")
    write_once(
        target / "README.md",
        f"# {name}\n\n{prompt or 'Generated with the Nexus workspace scaffolder.'}\n\n"
        "## Development\n\n```bash\npython -m pip install -e '.[dev]'\npytest\n```\n",
    )
    write_once(target / ".gitignore", ".venv/\n__pycache__/\n.pytest_cache/\n*.pyc\n.env\n")
    write_once(
        target / "pyproject.toml",
        f"""[build-system]
requires = ["setuptools>=69"]
build-backend = "setuptools.build_meta"

[project]
name = "{name}"
version = "0.1.0"
requires-python = ">=3.11"
dependencies = []

[project.optional-dependencies]
dev = ["pytest>=8"]
""",
    )
    write_once(target / "src" / package / "__init__.py", f'"""The {name} package."""\n')
    write_once(
        target / "tests" / "test_smoke.py",
        f"def test_package_imports():\n    import {package}\n    assert {package} is not None\n",
    )
    if template == "ml":
        write_once(
            target / "src" / package / "train.py",
            '"""Training entry point."""\n\n\ndef train() -> dict[str, float]:\n    return {"accuracy": 0.0}\n',
        )
    elif template == "agent":
        write_once(
            target / "src" / package / "agent.py",
            '"""Agent graph entry point."""\n\n\ndef build():\n    raise NotImplementedError("Define the agent graph")\n',
        )
    elif template == "api":
        write_once(
            target / "src" / package / "app.py",
            'from fastapi import FastAPI\n\napp = FastAPI(title="Generated API")\n\n@app.get("/health")\ndef health():\n    return {"status": "ok"}\n',
        )
        pyproject = target / "pyproject.toml"
        pyproject.write_text(pyproject.read_text().replace("dependencies = []", 'dependencies = ["fastapi>=0.115", "uvicorn>=0.34"]'))
    if not (target / ".git").exists() and shutil.which("git"):
        subprocess.run(["git", "init", "-q"], cwd=target, check=True)


def agent_command(agent: str, target: Path, prompt: str) -> list[str]:
    instruction = (
        f"{prompt}\n\nWork only inside {target}. Build on the generated starter, add tests, "
        "and leave the project runnable. Do not access credentials or other workspace projects."
    )
    if agent == "codex":
        return ["codex", "exec", "--sandbox", "workspace-write", "--skip-git-repo-check", "-C", str(target), instruction]
    if agent == "claude":
        return ["claude", "-p", "--permission-mode", "acceptEdits", instruction]
    if agent == "custom":
        configured = os.getenv("NEXUS_CUSTOM_AGENT_COMMAND", "")
        if not configured:
            raise RuntimeError("NEXUS_CUSTOM_AGENT_COMMAND is not configured")
        return [*shlex.split(configured), instruction]
    raise ValueError(f"unsupported agent: {agent}")


def scaffold(args: argparse.Namespace) -> None:
    name = slug(args.name)
    target = (WORKSPACE / name).resolve()
    if WORKSPACE not in target.parents:
        raise ValueError("project must be created inside the workspace")
    target.mkdir(parents=True, exist_ok=True)
    create_base(target, name, args.template, args.prompt)
    print(f"Created {target}")
    if args.agent == "none":
        print("Starter generated without an agent. Open it in JupyterLab or the IDE.")
        return
    if args.agent != "custom" and not shutil.which(args.agent):
        raise RuntimeError(f"{args.agent} CLI is not installed")
    subprocess.run(agent_command(args.agent, target, args.prompt or f"Complete the {name} starter"), cwd=target, check=True)


def agents(_: argparse.Namespace) -> None:
    for name, key in (("codex", "OPENAI_API_KEY"), ("claude", "ANTHROPIC_API_KEY")):
        installed = "installed" if shutil.which(name) else "missing"
        authenticated = "configured" if os.getenv(key) else "login or API key required"
        print(f"{name:8} {installed:10} {authenticated}")
    custom = "configured" if os.getenv("NEXUS_CUSTOM_AGENT_COMMAND") else "not configured"
    print(f"{'custom':8} {'external':10} {custom}")


def parser() -> argparse.ArgumentParser:
    root = argparse.ArgumentParser(prog="nexus", description="Nexus AI workspace tools")
    commands = root.add_subparsers(dest="command", required=True)
    make = commands.add_parser("scaffold", help="generate and optionally complete a starter codebase")
    make.add_argument("name")
    make.add_argument("--prompt", default="")
    make.add_argument("--template", choices=("python", "ml", "agent", "api"), default="python")
    make.add_argument("--agent", choices=("none", "codex", "claude", "custom"), default="none")
    make.set_defaults(run=scaffold)
    available = commands.add_parser("agents", help="show coding-agent availability")
    available.set_defaults(run=agents)
    return root


if __name__ == "__main__":
    arguments = parser().parse_args()
    try:
        arguments.run(arguments)
    except (RuntimeError, ValueError, subprocess.CalledProcessError) as error:
        raise SystemExit(f"nexus: {error}") from error
