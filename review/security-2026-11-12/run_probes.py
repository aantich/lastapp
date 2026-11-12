#!/usr/bin/env python3
"""Run secure regressions for the original security findings.
The original vulnerable-behavior probes are preserved as historical .txt evidence.
"""
import argparse, os, pathlib, subprocess
ROOT=pathlib.Path(__file__).resolve().parents[2]
parser=argparse.ArgumentParser(description=__doc__)
parser.add_argument('--config',type=pathlib.Path,default=ROOT/'.local/config.json')
args=parser.parse_args()
env=dict(os.environ,LASTAPP_SECURITY_CONFIG=str(args.config.resolve()))
env.setdefault('GOCACHE',str(ROOT/'.local/go-cache'))
raise SystemExit(subprocess.run(['go','test','-race','./internal/control','-run','^TestSecurityRegressions$','-v','-count=1'],cwd=ROOT,env=env).returncode)
