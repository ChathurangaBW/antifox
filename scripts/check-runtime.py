#!/usr/bin/env python3
"""Report local runtime availability. Never installs packages or contacts targets."""

import argparse
import importlib.metadata
import json
import os
from pathlib import Path
import platform
import shutil
import subprocess


# This inventory mirrors the current Dockerfile, not the entire Kali catalog.
# Presence and successful identity checks are deliberately separate statuses.
EXECUTABLES = (
    "rg", "curl", "wget", "vim", "git", "jq", "unzip", "tar", "perl",
    "ruby", "dig", "ping", "nc", "whois", "nmap", "whatweb", "wafw00f",
    "sqlmap", "gobuster", "ffuf", "sslscan", "hydra", "node", "npm",
    "python3", "bash", "playwright", "playwright-cli",
)
IDENTITY_ARGS = {
    "rg": ["--version"], "curl": ["--version"], "git": ["--version"],
    "node": ["--version"], "npm": ["--version"], "python3": ["--version"],
    "ffuf": ["-V"], "gobuster": ["version"], "nmap": ["--version"],
    "sslscan": ["--version"], "sqlmap": ["--version"],
    "wafw00f": ["--version"], "playwright": ["--version"],
    "playwright-cli": ["--version"],
}


def identity(path, args):
    try:
        result = subprocess.run(
            [path, *args], stdin=subprocess.DEVNULL, capture_output=True,
            text=True, errors="replace", timeout=15, check=False,
        )
        return {
            "status": "passed" if result.returncode == 0 else "failed",
            "exit_code": result.returncode,
            "output": (result.stdout + result.stderr)[:4096],
        }
    except (OSError, subprocess.TimeoutExpired) as exc:
        return {"status": "failed", "error": str(exc)}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--app-dir", default="/app")
    parser.add_argument("--image-ref", default="unknown",
                        help="Operator-supplied image reference; not independently verified")
    parser.add_argument("--resource-path", action="append", default=[],
                        help="Optional resource path to check for existence; repeat for each path")
    args = parser.parse_args()
    app = Path(args.app_dir)
    tools = []
    for name in EXECUTABLES:
        path = shutil.which(name)
        row = {"name": name, "path": path, "status": "present" if path else "missing"}
        if path and name in IDENTITY_ARGS:
            row["identity_check"] = identity(path, IDENTITY_ARGS[name])
        tools.append(row)

    try:
        import docx
        docx.Document()
        docx_check = {"status": "passed", "version": importlib.metadata.version("python-docx")}
    except Exception as exc:
        docx_check = {"status": "failed", "error": str(exc)}

    browser_root = Path(os.environ.get("PLAYWRIGHT_BROWSERS_PATH", str(Path.home() / ".cache/ms-playwright")))
    chromium = list(browser_root.glob("chromium*/**/chrome")) if browser_root.is_dir() else []
    checks = {
        "antifox_executable": os.access(app / "antifox", os.X_OK) and (app / "antifox").is_file(),
        "skills_directory": (app / "skills").is_dir(),
        "python_docx": docx_check,
        "chromium_files": [str(path) for path in chromium],
    }
    failures = [row["name"] for row in tools if row["status"] == "missing" or row.get("identity_check", {}).get("status") == "failed"]
    for name in ("antifox_executable", "skills_directory"):
        if not checks[name]:
            failures.append(name)
    if docx_check["status"] == "failed":
        failures.append("python_docx")
    if not chromium:
        failures.append("chromium_files")
    print(json.dumps({
        "schema_version": 1,
        "image_ref_supplied": args.image_ref,
        "architecture": platform.machine(),
        "uid": os.getuid() if hasattr(os, "getuid") else None,
        "tools": tools,
        "dependencies": checks,
        "resource_paths": [
            {"path": path, "exists": Path(path).exists()}
            for path in args.resource_path
        ],
        "failures": failures,
        "limitations": [
            "Presence is not a successful identity check.",
            "Identity checks do not validate target-facing functionality.",
            "Chromium file presence does not prove browser startup.",
            "Resource paths are checked only when explicitly provided; their contents are not inspected.",
            "Only the reported architecture and runtime user were checked.",
        ],
    }, indent=2))
    return 1 if failures else 0


if __name__ == "__main__":
    raise SystemExit(main())
