#!/usr/bin/env python3
"""Nexus workspace scaffolder and coding-agent launcher."""

from __future__ import annotations

import argparse
import json
import os
import re
import shlex
import shutil
import subprocess
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path

WORKSPACE = Path(os.getenv("NEXUS_WORKSPACE", "/workspace")).resolve()
NEXUS_URL = os.getenv("MLAIOPS_URL", "http://gateway:8080").rstrip("/")


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
    elif template in {"function", "fullstack"}:
        route = "predict" if template == "fullstack" else "invoke"
        write_once(
            target / "src" / package / "app.py",
            f'''from fastapi import FastAPI\nfrom pydantic import BaseModel\n\napp = FastAPI(title="{name}")\n\nclass Payload(BaseModel):\n    values: list[float] = []\n\n@app.get("/health")\ndef health():\n    return {{"status": "ok"}}\n\n@app.post("/{route}")\ndef run(payload: Payload):\n    return {{"result": sum(payload.values), "count": len(payload.values)}}\n''',
        )
        write_once(
            target / "Dockerfile",
            f'''FROM python:3.11-slim\nWORKDIR /app\nCOPY . .\nRUN pip install --no-cache-dir .\nUSER 65532:65532\nEXPOSE 8080\nCMD ["uvicorn", "{package}.app:app", "--host", "0.0.0.0", "--port", "8080"]\n''',
        )
        write_once(
            target / ".github" / "workflows" / "ci.yml",
            "name: ci\non: [push, pull_request]\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: actions/checkout@v4\n      - uses: actions/setup-python@v5\n        with: {python-version: '3.11'}\n      - run: pip install -e '.[dev]'\n      - run: pytest\n",
        )
        if template == "fullstack":
            write_once(
                target / "src" / package / "web" / "index.html",
                f'''<!doctype html>\n<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>{name}</title></head>\n<body><main><h1>{name}</h1><form id="form"><input id="values" value="1,2,3"><button>Predict</button></form><pre id="output"></pre></main><script>form.onsubmit=async(e)=>{{e.preventDefault();const values=document.querySelector('#values').value.split(',').map(Number);const response=await fetch('/predict',{{method:'POST',headers:{{'content-type':'application/json'}},body:JSON.stringify({{values}})}});output.textContent=JSON.stringify(await response.json(),null,2)}};</script></body></html>\n''',
            )
            app_path = target / "src" / package / "app.py"
            app_path.write_text(
                app_path.read_text()
                + '''\nfrom pathlib import Path\nfrom fastapi.responses import FileResponse\n\n@app.get("/")\ndef index():\n    return FileResponse(Path(__file__).parent / "web" / "index.html")\n'''
            )
        pyproject = target / "pyproject.toml"
        pyproject.write_text(pyproject.read_text().replace("dependencies = []", 'dependencies = ["fastapi>=0.115", "uvicorn>=0.34", "pydantic>=2"]'))
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


def api_get(path: str) -> dict:
    request = urllib.request.Request(NEXUS_URL + path, headers={"Accept": "application/json"})
    if token := os.getenv("MLAIOPS_TOKEN"):
        request.add_header("Authorization", f"Bearer {token}")
    try:
        with urllib.request.urlopen(request, timeout=15) as response:
            return json.load(response)
    except urllib.error.HTTPError as error:
        detail = error.read().decode("utf-8", "replace")
        raise RuntimeError(f"control plane returned {error.code}: {detail}") from error
    except urllib.error.URLError as error:
        raise RuntimeError(f"control plane is unavailable: {error.reason}") from error


def project_sync(args: argparse.Namespace) -> None:
    project = api_get(f"/api/v1/projects/{urllib.parse.quote(args.project_id, safe='')}")
    repository = project.get("repository")
    if not repository:
        raise RuntimeError("project has no Git repository; connect one in the Nexus console")
    target = (WORKSPACE / project["namespace"]).resolve()
    if WORKSPACE not in target.parents:
        raise RuntimeError("project target escaped the workspace")
    if not shutil.which("git"):
        raise RuntimeError("git is not installed in this workspace")
    url, branch = repository["url"], repository["default_branch"]
    if (target / ".git").exists():
        origin = subprocess.run(["git", "remote", "get-url", "origin"], cwd=target, check=True, capture_output=True, text=True).stdout.strip()
        if origin != url:
            raise RuntimeError(f"workspace origin is {origin!r}, expected {url!r}")
        subprocess.run(["git", "fetch", "--prune", "origin"], cwd=target, check=True)
        subprocess.run(["git", "checkout", branch], cwd=target, check=True)
        subprocess.run(["git", "pull", "--ff-only", "origin", branch], cwd=target, check=True)
        print(f"Updated {target} from {url} ({branch})")
        return
    if target.exists() and any(target.iterdir()):
        raise RuntimeError(f"target already exists and is not a Git checkout: {target}")
    target.parent.mkdir(parents=True, exist_ok=True)
    subprocess.run(["git", "clone", "--single-branch", "--branch", branch, url, str(target)], check=True)
    print(f"Cloned {url} to {target} ({branch})")


def parser() -> argparse.ArgumentParser:
    root = argparse.ArgumentParser(prog="nexus", description="Nexus AI workspace tools")
    commands = root.add_subparsers(dest="command", required=True)
    make = commands.add_parser("scaffold", help="generate and optionally complete a starter codebase")
    make.add_argument("name")
    make.add_argument("--prompt", default="")
    make.add_argument("--template", choices=("python", "ml", "agent", "api", "function", "fullstack"), default="python")
    make.add_argument("--agent", choices=("none", "codex", "claude", "custom"), default="none")
    make.set_defaults(run=scaffold)
    available = commands.add_parser("agents", help="show coding-agent availability")
    available.set_defaults(run=agents)
    projects = commands.add_parser("project", help="work with Git-native Nexus projects")
    project_commands = projects.add_subparsers(dest="project_command", required=True)
    sync = project_commands.add_parser("sync", help="clone or fast-forward a project's connected repository")
    sync.add_argument("project_id")
    sync.set_defaults(run=project_sync)
    return root


if __name__ == "__main__":
    arguments = parser().parse_args()
    try:
        arguments.run(arguments)
    except (RuntimeError, ValueError, subprocess.CalledProcessError) as error:
        raise SystemExit(f"nexus: {error}") from error
