#!/usr/bin/env python3
"""Create fresh local development settings for this checkout."""
from pathlib import Path
import os
import secrets

path = Path(__file__).resolve().with_name(".env")
try:
    fd = os.open(path, os.O_CREAT | os.O_WRONLY | os.O_EXCL, 0o600)
except FileExistsError:
    print("Using the existing .env")
else:
    with os.fdopen(fd, "w", encoding="utf-8") as output:
        output.write("POSTGRES_PASSWORD=" + secrets.token_hex(32) + "\nAPP_ORIGIN=http://localhost:8080\n")
    print("Created .env with a fresh local database password")
