import importlib.util
from pathlib import Path
from types import SimpleNamespace

import pytest


MODULE_PATH = Path(__file__).parents[2] / "deploy" / "workspace" / "nexus.py"
SPEC = importlib.util.spec_from_file_location("nexus_workspace", MODULE_PATH)
assert SPEC and SPEC.loader
nexus = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(nexus)


def test_project_sync_clones_connected_repository(tmp_path, monkeypatch) -> None:
    calls = []
    monkeypatch.setattr(nexus, "WORKSPACE", tmp_path)
    monkeypatch.setattr(
        nexus,
        "api_get",
        lambda path: {
            "namespace": "fraud-model",
            "repository": {
                "url": "https://github.com/acme/fraud-model.git",
                "default_branch": "main",
            },
        },
    )
    monkeypatch.setattr(nexus.shutil, "which", lambda _: "/usr/bin/git")
    monkeypatch.setattr(
        nexus.subprocess,
        "run",
        lambda command, **kwargs: calls.append((command, kwargs))
        or SimpleNamespace(stdout=""),
    )

    nexus.project_sync(SimpleNamespace(project_id="prj/one"))

    assert calls[0][0] == [
        "git",
        "clone",
        "--single-branch",
        "--branch",
        "main",
        "https://github.com/acme/fraud-model.git",
        str(tmp_path / "fraud-model"),
    ]


def test_project_sync_rejects_workspace_escape(tmp_path, monkeypatch) -> None:
    monkeypatch.setattr(nexus, "WORKSPACE", tmp_path)
    monkeypatch.setattr(
        nexus,
        "api_get",
        lambda _: {
            "namespace": "../outside",
            "repository": {
                "url": "https://github.com/acme/fraud-model.git",
                "default_branch": "main",
            },
        },
    )

    with pytest.raises(RuntimeError, match="escaped"):
        nexus.project_sync(SimpleNamespace(project_id="prj-one"))


def test_fullstack_scaffold_is_container_and_git_ready(tmp_path) -> None:
    target = tmp_path / "risk-app"
    nexus.create_base(target, "risk-app", "fullstack", "Score risk events")

    assert (target / "Dockerfile").exists()
    assert (target / ".github" / "workflows" / "ci.yml").exists()
    assert "/predict" in (target / "src" / "risk_app" / "app.py").read_text()
    assert "fetch('/predict'" in (target / "src" / "risk_app" / "web" / "index.html").read_text()
