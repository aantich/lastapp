#!/usr/bin/env python3
"""Run Go tests against a disposable MariaDB, never the development data.

Starts a fresh MariaDB (service mariadb-test of docker-compose.yaml, profile test,
tmpfs), writes per-run secrets and configuration into a new temporary directory
(ci_config.py), prepares the registry exactly as CI does (TestPrepareFixture),
runs `go test` with LASTAPP_TEST_CONFIG naming that configuration, then removes
the container and the directory. Arguments go to `go test`:
  python3 scripts/testdb.py -race -count=1 ./...
  python3 scripts/testdb.py ./internal/control -run TestAgent
scripts/e2e.py starts its own server (mariadb-e2e) through disposable() below.
Go tests refuse a server without MARKER (internal/testdb).
"""
import contextlib, os, pathlib, shutil, signal, socket, subprocess, sys, tempfile, time
ROOT=pathlib.Path(__file__).resolve().parents[1]
COMPOSE=['docker','compose','-f',str(ROOT/'docker-compose.yaml'),'--profile','test']
# Created only on servers started here; internal/testdb.Marker is the same name.
MARKER='lastapp_disposable_test'
# The only place the disposable database ports are chosen (the compose defaults
# match). They stay below the Linux ephemeral range (32768-60999): a port in it
# can be held at random by any outgoing connection (CI once failed to bind 33318
# right after the Go tests). The development MariaDB is 33317 (scripts/dev.py).
PORTS={'mariadb-test':13319,'mariadb-e2e':13318}
PORT_ENV={'mariadb-test':'LASTAPP_TEST_DB_PORT','mariadb-e2e':'LASTAPP_E2E_DB_PORT'}
def listening(port):
 with socket.socket() as s:return s.connect_ex(('127.0.0.1',port))==0
@contextlib.contextmanager
def disposable(service,directory,*config_args):
 """Start service on a fresh tmpfs, prepare its registry with secrets in directory
 and yield the root SQL command; always remove only that container afterwards.
 Never `down`: the compose project also owns the development database volume."""
 env=dict(os.environ,**{PORT_ENV[service]:str(PORTS[service])})
 remove=COMPOSE+['rm','--stop','--force','--volumes',service]
 def run(args,**kw):return subprocess.run([str(v) for v in args],cwd=ROOT,check=True,**{'env':env,**kw})
 signal.signal(signal.SIGTERM,lambda *_:sys.exit(143)) # still clean up when terminated
 subprocess.run(remove,cwd=ROOT,env=env,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
 if listening(PORTS[service]):raise RuntimeError(f'{service} port {PORTS[service]} already in use')
 try:
  for attempt in range(3): # a port can still be briefly held; remove and retry
   try:run(COMPOSE+['up','-d','--wait',service]);break
   except subprocess.CalledProcessError:
    if attempt==2:raise
    subprocess.run(remove,cwd=ROOT,env=env,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL);time.sleep(3)
  root_sql=COMPOSE+['exec','-T',service,'mariadb','-uroot']
  run(root_sql,input=f"CREATE DATABASE {MARKER}; CREATE DATABASE lastapp_control CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci",text=True)
  run([sys.executable,ROOT/'scripts/ci_config.py',directory,PORTS[service],*config_args])
  prepare={k:v for k,v in env.items() if k!='LASTAPP_TEST_CONFIG'}
  run(['go','test','./internal/control','-run','^TestPrepareFixture$','-count=1'],env=dict(prepare,LASTAPP_PREPARE_FIXTURE_CONFIG=str(pathlib.Path(directory)/'migrate.json')))
  yield root_sql
 finally:
  subprocess.run(remove,cwd=ROOT,env=env,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
def main(args):
 directory=pathlib.Path(tempfile.mkdtemp(prefix='lastapp-test-'))
 try:
  with disposable('mariadb-test',directory/'stack'):
   env={k:v for k,v in os.environ.items() if k!='LASTAPP_PREPARE_FIXTURE_CONFIG'}
   return subprocess.run(['go','test',*args],cwd=ROOT,env=dict(env,LASTAPP_TEST_CONFIG=str(directory/'stack'/'config.json'))).returncode
 finally:shutil.rmtree(directory,ignore_errors=True)
if __name__=='__main__':sys.exit(main(sys.argv[1:] or ['-count=1','./...']))
