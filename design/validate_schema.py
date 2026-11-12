#!/usr/bin/env python3
"""Check schema parity and reuse the real Go foundation SQL/API tests.

Requires `make dev`; uses a disposable registry on that existing development
server. Does not start another MariaDB or maintain a separate authorization spec.
"""
import os
from pathlib import Path
import shutil
import subprocess

ROOT = Path(__file__).resolve().parents[1]
source = (ROOT / 'internal/control/migrations/00001_control.sql').read_text().split('-- +goose Down')[0]
expected = '\n'.join(line for line in source.splitlines() if not line.startswith('-- +goose ')).strip()
actual = (ROOT / 'design/control_plane.sql').read_text().strip()
if actual != expected:
    raise SystemExit('Canonical design DDL differs from the executable bootstrap schema.')
config = Path(os.environ.get('LASTAPP_TEST_CONFIG', ROOT / '.local/config.json')).resolve()
if not config.is_file():
    raise SystemExit('Run make dev first, or set LASTAPP_TEST_CONFIG to the local development config.')
env = dict(os.environ, LASTAPP_TEST_CONFIG=str(config), GOCACHE=str(ROOT / '.local/go-cache'))
go = shutil.which('go')
if not go:
    raise SystemExit('Go is required.')
print('Canonical DDL matches bootstrap. Running current foundation SQL/API checks.', flush=True)
subprocess.run([go, 'test', '-count=1', '-v', '-run', '^TestAuthorizationFoundation$', './internal/control'], cwd=ROOT, env=env, check=True)
