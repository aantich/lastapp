#!/usr/bin/env python3
"""Verify the pinned fork snapshot without depending on a sibling checkout."""
import hashlib, json, pathlib
root=pathlib.Path(__file__).resolve().parents[1]/'third_party/go-mariadb'
manifest=json.loads((root/'SNAPSHOT.json').read_text())
for name,expected in manifest['files'].items():
    assert hashlib.sha256((root/name).read_bytes()).hexdigest()==expected, name
print(f"Verified {len(manifest['files'])} pinned driver files")
