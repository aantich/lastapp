#!/usr/bin/env python3
"""Run secure regressions for the original security findings.
The original vulnerable-behavior probes are preserved as historical .txt evidence.
"""
import os, pathlib, subprocess, sys
ROOT=pathlib.Path(__file__).resolve().parents[2]
# On the disposable test MariaDB of scripts/testdb.py, never the development data.
env=dict(os.environ);env.setdefault('GOCACHE',str(ROOT/'.local/go-cache'))
raise SystemExit(subprocess.run([sys.executable,ROOT/'scripts/testdb.py','-race','./internal/control','-run','^TestSecurityRegressions$','-v','-count=1'],cwd=ROOT,env=env).returncode)
