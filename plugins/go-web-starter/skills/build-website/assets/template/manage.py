#!/usr/bin/env python3
"""Manage the site's registered CRUD resources."""
import argparse
from contextlib import contextmanager
import json
import os
from pathlib import Path
import re
import tempfile

KEY = re.compile(r"[a-z][a-z0-9]*(?:-[a-z0-9]+)*\Z")
RESERVED = {"admin", "auth", "dashboard", "health", "profile", "ready"}


def resource_spec(singular, plural=None, label=None, plural_label=None):
    collection = singular + "s" if plural is None else plural
    for key in (singular, collection):
        if len(key) > 48 or not KEY.fullmatch(key) or key in RESERVED:
            raise ValueError("use lowercase URL slugs up to 48 characters; reserved: " + ", ".join(sorted(RESERVED)))
    names = [(label if label is not None else singular.replace("-", " ")).strip(),
             (plural_label if plural_label is not None else collection.replace("-", " ")).strip()]
    if any(not name or len(name) > 80 or not name.isprintable() for name in names):
        raise ValueError("display labels must contain 1 to 80 printable characters")
    return {"resource": singular, "collection": collection, "label": names[0], "pluralLabel": names[1]}


@contextmanager
def locked_config(root):
    # Separate lock file keeps the lock valid when the JSON file is atomically replaced.
    with (root / ".site-resources.lock").open("a+b") as stream:
        if os.name == "nt":
            import msvcrt
            if stream.tell() == 0:
                stream.write(b"0")
                stream.flush()
            stream.seek(0)
            msvcrt.locking(stream.fileno(), msvcrt.LK_LOCK, 1)
        else:
            import fcntl
            fcntl.flock(stream, fcntl.LOCK_EX)
        yield


def main():
    cli = argparse.ArgumentParser(description=__doc__)
    commands = cli.add_subparsers(dest="command", required=True)
    add = commands.add_parser("add-resource", help="add a private CRUD module")
    add.add_argument("resource")
    add.add_argument("--plural")
    add.add_argument("--label")
    add.add_argument("--plural-label")
    commands.add_parser("list-resources", help="list the mounted collections")
    args = cli.parse_args()
    root = Path(__file__).resolve().parent
    path = root / "site.json"
    if not path.is_file():
        cli.error("run manage.py from a generated website")
    if args.command == "list-resources":
        site = json.loads(path.read_text(encoding="utf-8"))
        for item in [site, *site.get("resources", [])]:
            print(f"{item['label']}: /api/{item['collection']}")
        return
    try:
        spec = resource_spec(args.resource, args.plural, args.label, args.plural_label)
    except ValueError as exc:
        cli.error(str(exc))
    with locked_config(root):
        site = json.loads(path.read_text(encoding="utf-8"))
        for item in [site, *site.get("resources", [])]:
            if all(item.get(k) == v for k,v in spec.items()):
                print(f"Already registered /api/{spec['collection']}")
                return
            if item["resource"] == spec["resource"] or item["collection"] == spec["collection"]:
                cli.error("a module already uses this resource key or collection URL")
        site.setdefault("resources", []).append(spec)
        with tempfile.NamedTemporaryFile("w", encoding="utf-8", dir=root, delete=False) as out:
            pending = Path(out.name)
            try:
                json.dump(site, out, ensure_ascii=False, indent=2)
                out.write("\n")
                out.flush()
                os.fsync(out.fileno())
            except BaseException:
                pending.unlink(missing_ok=True)
                raise
        try:
            os.replace(pending, path)
        finally:
            pending.unlink(missing_ok=True)
    print(f"Added /api/{spec['collection']}. Rebuild the site with: docker compose up --build")


if __name__ == "__main__":
    main()
