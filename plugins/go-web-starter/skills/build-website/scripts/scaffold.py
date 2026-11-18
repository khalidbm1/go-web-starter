#!/usr/bin/env python3
"""Create a standalone project from the packaged template."""
import argparse
import html
import json
import os
from pathlib import Path
import runpy
import shutil
import subprocess
import sys
import tempfile

TEMPLATE = Path(__file__).resolve().parents[1] / "assets" / "template"


def main():
    cli = argparse.ArgumentParser(description=__doc__)
    cli.add_argument("destination", type=Path)
    cli.add_argument("--name", default="Fieldbook")
    cli.add_argument("--resource", default="project", help="singular URL key")
    cli.add_argument("--plural", default=None, help="plural collection URL key")
    cli.add_argument("--label", help="singular display label")
    cli.add_argument("--plural-label", help="plural display label")
    args = cli.parse_args()
    try:
        spec = runpy.run_path(str(TEMPLATE / "manage.py"))["resource_spec"](args.resource, args.plural, args.label, args.plural_label)
    except ValueError as exc:
        cli.error(str(exc))
    label = spec["label"]
    if not args.name.strip() or len(args.name.strip()) > 80 or not args.name.strip().isprintable():
        cli.error("site name must contain 1 to 80 printable characters")
    target = args.destination.expanduser().absolute()
    if target.is_symlink() or (target.exists() and (not target.is_dir() or any(target.iterdir()))):
        cli.error("choose an empty or new destination directory")
    target.parent.mkdir(parents=True, exist_ok=True)
    examples = ["Website refresh", "Customer onboarding", "Quarterly review"] if args.resource == "project" else [
        f"First {label}", f"Second {label}", f"Third {label}",
    ]
    config = {"name": args.name.strip(), **spec, "examples": examples, "resources": []}
    with tempfile.TemporaryDirectory(prefix=".go-web-", dir=target.parent) as temp:
        stage = Path(temp) / "project"
        shutil.copytree(TEMPLATE, stage)
        # The dotfile source names are portable across plugin packaging hosts.
        for name in ("gitignore", "dockerignore"):
            (stage / (name + ".txt")).rename(stage / ("." + name))
        (stage / "site.json").write_text(json.dumps(config, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        index = stage / "web" / "index.html"
        index.write_text(index.read_text(encoding="utf-8").replace("__SITE_TITLE__", html.escape(config["name"])), encoding="utf-8")
        subprocess.run([sys.executable, str(stage / "setup.py")], cwd=stage, check=True, stdout=subprocess.DEVNULL)
        # POSIX can atomically replace an empty directory. On Windows an existing
        # empty directory must be removed first; rmdir refuses a nonempty one.
        if os.name == "nt" and target.exists():
            target.rmdir()
        stage.rename(target)
    print(f"Created {config['name']} in {target}")
    print("Start in that directory with: docker compose up --build")
    print("Open http://localhost:8080")


if __name__ == "__main__":
    main()
