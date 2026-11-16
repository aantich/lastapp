#!/usr/bin/env python3
"""Run the browser regressions against a disposable stack, never the development data.

Starts a fresh MariaDB (docker-compose.e2e.yaml, tmpfs), prepares it exactly as
CI does (ci_config.py + TestPrepareFixture), builds the console and API, starts a
separate API and development OIDC issuer on their own ports, runs Playwright,
then stops the processes and removes the database container. Secrets, binaries and logs live in
.local/e2e/, which is recreated on every run. Extra arguments go to Playwright:
  python3 scripts/e2e.py tests/security.spec.ts
"""
import json, os, pathlib, shutil, signal, socket, subprocess, sys, time, urllib.request
ROOT=pathlib.Path(__file__).resolve().parents[1]
STATE=ROOT/'.local'/'e2e'
# The only place the e2e ports are chosen; everything else derives from STATE/e2e.json and config.json.
PORTS={'db':33318,'api':8797,'oidc':8798,'ui':5183,'app':8799}
SERVICE='mariadb-e2e'
ENV=dict(os.environ,LASTAPP_E2E_DB_PORT=str(PORTS['db']))
COMPOSE=['docker','compose','-f',str(ROOT/'docker-compose.e2e.yaml')]
def run(args,**kw):return subprocess.run([str(v) for v in args],cwd=kw.pop('cwd',ROOT),env=kw.pop('env',ENV),check=True,**kw)
def listening(port):
 with socket.socket() as s:return s.connect_ex(('127.0.0.1',port))==0
def wait_url(url,log):
 for _ in range(150):
  try:
   with urllib.request.urlopen(url,timeout=1) as response:
    if response.status==200:return
  except Exception:pass
  time.sleep(.2)
 raise RuntimeError(f'Service did not start: {url}; inspect {log}')
def main():
 processes=[]
 signal.signal(signal.SIGTERM,lambda *_:sys.exit(143)) # still clean up when terminated
 run(COMPOSE+['down','-v','--remove-orphans'],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
 busy=[f'{name}={port}' for name,port in PORTS.items() if listening(port)]
 if busy:raise RuntimeError('e2e ports already in use: '+', '.join(busy))
 if STATE.exists():shutil.rmtree(STATE)
 STATE.mkdir(mode=0o700,parents=True)
 try:
  run(COMPOSE+['up','-d','--wait',SERVICE])
  root_sql=COMPOSE+['exec','-T',SERVICE,'mariadb','-uroot']
  run(root_sql,input="CREATE DATABASE lastapp_control CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci",text=True)
  run([sys.executable,ROOT/'scripts/ci_config.py',STATE,PORTS['db'],'--api-port',PORTS['api'],'--oidc-port',PORTS['oidc'],'--ui-port',PORTS['ui'],'--artifact-directory',STATE/'artifacts'])
  run(['go','test','./internal/control','-run','^TestPrepareFixture$','-count=1'],env=dict(ENV,LASTAPP_PREPARE_FIXTURE_CONFIG=str(STATE/'migrate.json')))
  # The API embeds the console, so build it from the current source first.
  if not (ROOT/'web/admin/node_modules').exists():run(['npm','ci'],cwd=ROOT/'web/admin')
  run(['npm','run','build'],cwd=ROOT/'web/admin')
  run(['go','build','-o',STATE/'lastapp','./cmd/lastapp']);run(['go','build','-o',STATE/'devoidc','./cmd/devoidc'])
  config=json.loads((STATE/'config.json').read_text())
  def start(name,args):
   log=open(STATE/(name+'.log'),'ab',buffering=0)
   processes.append(subprocess.Popen([str(v) for v in args],cwd=ROOT,env=ENV,stdin=subprocess.DEVNULL,stdout=log,stderr=log,start_new_session=True))
  start('devoidc',[STATE/'devoidc','-address',config['oidc_issuer'].removeprefix('http://'),'-app-origin',config['public_url'],'-development-ui-origin',config['development_ui_origin']])
  wait_url(config['oidc_issuer']+'/.well-known/openid-configuration',STATE/'devoidc.log')
  start('lastapp',[STATE/'lastapp','serve','-config',STATE/'config.json'])
  wait_url(config['public_url']+'/readyz',STATE/'lastapp.log')
  # Tests read this file (via LASTAPP_E2E) for what the API config does not describe.
  (STATE/'e2e.json').write_text(json.dumps({'config':str(STATE/'config.json'),'app_port':PORTS['app'],'root_sql':root_sql},indent=2))
  result=subprocess.run(['npm','run','test:e2e','--',*sys.argv[1:]],cwd=ROOT/'web/admin',env=dict(ENV,LASTAPP_E2E=str(STATE/'e2e.json')))
  return result.returncode
 finally:
  for proc in processes:
   if proc.poll() is None:proc.send_signal(signal.SIGTERM)
  for proc in processes:
   try:proc.wait(timeout=10)
   except subprocess.TimeoutExpired:proc.kill()
  subprocess.run(COMPOSE+['down','-v','--remove-orphans'],cwd=ROOT,env=ENV,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
if __name__=='__main__':sys.exit(main())
