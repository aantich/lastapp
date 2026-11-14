#!/usr/bin/env python3
"""Generate/check the DDL document and reuse the Go foundation SQL/API tests.

--write regenerates from migrations; --check verifies parity offline. The default
also requires `make dev` and uses a disposable registry on that existing server.
Does not start another MariaDB or maintain a separate authorization spec.
"""
import argparse
import os
from pathlib import Path
import shutil
import subprocess

ROOT = Path(__file__).resolve().parents[1]
parser = argparse.ArgumentParser(description=__doc__)
mode = parser.add_mutually_exclusive_group()
mode.add_argument('--write', action='store_true', help='regenerate the SQL document from migrations, without database access')
mode.add_argument('--check', action='store_true', help='check SQL document parity, without database access')
options = parser.parse_args()
sources = [p.read_text().split('-- +goose Down')[0] for p in sorted((ROOT / 'internal/control/migrations').glob('*.sql'))]
expected = '\n\n'.join('\n'.join(line for line in source.splitlines() if not line.startswith('-- +goose ')).strip() for source in sources)
document = ROOT / 'design/control_plane.sql'
if options.write:
    document.write_text(expected + '\n')
    print('Regenerated design/control_plane.sql from executable migrations.')
    raise SystemExit(0)
actual = document.read_text().strip()
if actual != expected:
    raise SystemExit('Canonical design DDL differs from executable migrations. Run python3 design/validate_schema.py --write.')
if options.check:
    print('Canonical DDL matches executable migrations.')
    raise SystemExit(0)
config = Path(os.environ.get('LASTAPP_TEST_CONFIG', ROOT / '.local/config.json')).resolve()
if not config.is_file():
    raise SystemExit('Run make dev first, or set LASTAPP_TEST_CONFIG to the local development config.')
env = dict(os.environ, LASTAPP_TEST_CONFIG=str(config), GOCACHE=str(ROOT / '.local/go-cache'))
go = shutil.which('go')
if not go:
    raise SystemExit('Go is required.')
print('Canonical DDL matches bootstrap. Running current foundation SQL/API checks.', flush=True)
subprocess.run([go, 'test', '-count=1', '-v', '-run', '^TestAuthorizationFoundation$', './internal/control'], cwd=ROOT, env=env, check=True)
