#!/usr/bin/env python3
"""Create a standalone project from the packaged template."""
import argparse
import html
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile

TEMPLATE = Path(__file__).resolve().parents[1] / "assets" / "template"
KEY = re.compile(r"[a-z][a-z0-9]*(?:-[a-z0-9]+)*\Z")
RESERVED = {"auth", "health", "ready"}


def main():
    cli = argparse.ArgumentParser(description=__doc__)
    cli.add_argument("destination", type=Path)
    cli.add_argument("--name", default="Fieldbook")
    cli.add_argument("--resource", default="project", help="singular URL key")
    cli.add_argument("--plural", default=None, help="plural collection URL key")
    cli.add_argument("--label", help="singular display label")
    cli.add_argument("--plural-label", help="plural display label")
    args = cli.parse_args()
    plural = args.plural if args.plural is not None else args.resource + "s"
    label = (args.label if args.label is not None else args.resource.replace("-", " ")).strip()
    labels = (args.plural_label if args.plural_label is not None else plural.replace("-", " ")).strip()
    for key in (args.resource, plural):
        if len(key) > 48 or not KEY.fullmatch(key) or key in RESERVED:
            cli.error("resource keys must be lowercase URL slugs up to 48 characters; auth, health, and ready are reserved")
    for value in (args.name.strip(), label, labels):
        if not value or len(value) > 80 or not value.isprintable():
            cli.error("name and labels must contain 1 to 80 printable characters")

    target = args.destination.expanduser().absolute()
    if target.is_symlink() or (target.exists() and (not target.is_dir() or any(target.iterdir()))):
        cli.error("choose an empty or new destination directory")
    target.parent.mkdir(parents=True, exist_ok=True)
    examples = ["Website refresh", "Customer onboarding", "Quarterly review"] if args.resource == "project" else [
        f"First {label}", f"Second {label}", f"Third {label}",
    ]
    config = {"name": args.name.strip(), "resource": args.resource, "collection": plural,
              "label": label, "pluralLabel": labels, "examples": examples}
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
