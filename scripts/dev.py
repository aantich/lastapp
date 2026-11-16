#!/usr/bin/env python3
"""Build and start an isolated, persistent loopback Lastapp development stack."""
import argparse, base64, json, os, pathlib, secrets, shutil, socket, subprocess, sys, time, urllib.request
ROOT=pathlib.Path(__file__).resolve().parents[1]
REGISTRY='lastapp_foundation'
COMPOSE=['docker','compose','-f',str(ROOT/'docker-compose.yaml')]
LOCAL=ROOT/'.local'; LOCAL.mkdir(mode=0o700,exist_ok=True)
ENV=os.environ.copy();ENV['GOCACHE']=str(LOCAL/'go-cache')
node=pathlib.Path.home()/'.nvm/versions/node/v24.20.0/bin'
try:
 node_works=subprocess.run(['node','--version'],env=ENV,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,timeout=5).returncode==0
except (OSError,subprocess.TimeoutExpired):node_works=False
if not node_works and node.exists(): ENV['PATH']=str(node)+':'+ENV['PATH']
def run(args,**kw):return subprocess.run([str(v) for v in args],cwd=kw.pop("cwd",ROOT),env=kw.pop("env",ENV),check=True,**kw)
def write(path,value):path.write_text(value);path.chmod(0o600)
def alive(pid):
 try:os.kill(int(pid),0);return True
 except (OSError,ValueError):return False
def listening(port):
 with socket.socket() as s:return s.connect_ex(('127.0.0.1',port))==0
def start(name,args):
 pidfile=LOCAL/(name+'.pid')
 if pidfile.exists() and alive(pidfile.read_text()):
  port={'lastapp':8787,'devoidc':8788,'mariadb':33317}[name]
  if listening(port):return
 with open(LOCAL/(name+'.log'),'ab',buffering=0) as log:
  proc=subprocess.Popen([str(v) for v in args],cwd=ROOT,env=ENV,stdin=subprocess.DEVNULL,stdout=log,stderr=log,start_new_session=True)
 write(pidfile,str(proc.pid))
def wait_url(url):
 for _ in range(100):
  try:
   with urllib.request.urlopen(url,timeout=1) as response:
    if response.status==200:return
  except Exception:pass
  time.sleep(.2)
 raise RuntimeError('Service did not start: '+url+'; inspect .local/*.log')
def use_docker():
 """A locally installed mariadbd wins; otherwise docker-compose.yaml runs the same image as CI."""
 if os.environ.get('LASTAPP_MARIADB')=='docker':return True
 if shutil.which('mariadbd'):return False
 if shutil.which('docker'):return True
 raise RuntimeError('MariaDB is required: install mariadbd (e.g. brew install mariadb) or Docker')
def container_running():
 if not (LOCAL/'mariadb.env').exists():return False
 out=subprocess.run(COMPOSE+['ps','--status','running','-q','mariadb'],stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,text=True,cwd=ROOT)
 return out.returncode==0 and out.stdout.strip()!=''
def main():
 parser=argparse.ArgumentParser();parser.add_argument('--skip-build',action='store_true');parser.add_argument('--stop',action='store_true');args=parser.parse_args()
 if args.stop:
  for name in ['lastapp','devoidc','mariadb']:
   path=LOCAL/(name+'.pid')
   if path.exists() and alive(path.read_text()):os.kill(int(path.read_text()),15)
  if shutil.which('docker') and container_running():run(COMPOSE+['stop','mariadb'],stdout=subprocess.DEVNULL)
  print('Local services stopped. Persistent data preserved.');return
 docker=use_docker()
 for port,name in [(8787,'lastapp'),(8788,'devoidc'),(33317,'mariadb')]:
  pid=LOCAL/(name+'.pid')
  if name=='mariadb' and docker and container_running():continue
  if listening(port) and not(pid.exists() and alive(pid.read_text())):raise RuntimeError(f'Port {port} already in use by an unrelated process')
 statefile=LOCAL/'state.json'
 if not statefile.exists():write(statefile,json.dumps({k:secrets.token_urlsafe(32) for k in ['registry_password','provisioner_password','legacy_password']}))
 state=json.loads(statefile.read_text())
 if docker and 'root_password' not in state:state['root_password']=secrets.token_urlsafe(32);write(statefile,json.dumps(state))
 # Inside a container, published-port connections arrive from the Docker gateway, not 127.0.0.1.
 host='%' if docker else '127.0.0.1'
 if docker:
  write(LOCAL/'mariadb.env',f"MARIADB_ROOT_PASSWORD={state['root_password']}\n")
  run(COMPOSE+['up','-d','--wait','mariadb'])
  def sql(text):return run(COMPOSE+['exec','-T','-e','MYSQL_PWD','mariadb','mariadb','-uroot'],input=text,text=True,stdout=subprocess.PIPE,stderr=subprocess.PIPE,env=dict(ENV,MYSQL_PWD=state['root_password'])).stdout
  root_dsn=f"root:{state['root_password']}@tcp(127.0.0.1:33317)/{REGISTRY}"
 else:
  data=LOCAL/'mariadb-data';sock=LOCAL/'mariadb.sock';daemon=pathlib.Path(shutil.which('mariadbd')).resolve();base=daemon.parent.parent
  if not data.exists():run(['mariadb-install-db','--no-defaults',f'--basedir={base}',f'--datadir={data}','--auth-root-authentication-method=normal','--skip-test-db'],stdout=subprocess.DEVNULL)
  start('mariadb',[daemon,'--no-defaults',f'--basedir={base}',f'--datadir={data}',f'--socket={sock}',f'--pid-file={LOCAL/"mariadb-server.pid"}','--bind-address=127.0.0.1','--port=33317','--skip-name-resolve','--innodb-buffer-pool-size=64M','--performance-schema=OFF',f'--log-error={LOCAL/"mariadb-error.log"}'])
  def sql(text):return run(['mariadb','--no-defaults','--protocol=SOCKET',f'--socket={sock}','-uroot'],input=text,text=True,stdout=subprocess.PIPE,stderr=subprocess.PIPE).stdout
  root_dsn=f'root@unix({sock})/{REGISTRY}'
 for _ in range(300 if docker else 100):
  try:sql('SELECT 1;');break
  except subprocess.CalledProcessError:time.sleep(.2)
 else:raise RuntimeError('Private MariaDB failed to start')
 sql(f"CREATE DATABASE IF NOT EXISTS {REGISTRY} CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci; CREATE USER IF NOT EXISTS 'lastapp_control'@'{host}' IDENTIFIED BY '{state['registry_password']}'; GRANT SELECT,INSERT ON {REGISTRY}.* TO 'lastapp_control'@'{host}'; REVOKE UPDATE,DELETE ON {REGISTRY}.* FROM 'lastapp_control'@'{host}'; CREATE USER IF NOT EXISTS 'lastapp_provisioner'@'{host}' IDENTIFIED BY '{state['provisioner_password']}'; GRANT CREATE USER ON *.* TO 'lastapp_provisioner'@'{host}'; GRANT CREATE,ALTER,INDEX,REFERENCES,SELECT,INSERT,UPDATE,DELETE ON `la\\_%`.* TO 'lastapp_provisioner'@'{host}' WITH GRANT OPTION; CREATE DATABASE IF NOT EXISTS legacy_demo; CREATE TABLE IF NOT EXISTS legacy_demo.contacts (id INT PRIMARY KEY,name VARCHAR(100)); INSERT IGNORE INTO legacy_demo.contacts VALUES (1,'Existing customer'); CREATE USER IF NOT EXISTS 'legacy_runtime'@'{host}' IDENTIFIED BY '{state['legacy_password']}'; GRANT SELECT,INSERT,UPDATE,DELETE ON `legacy\\_demo`.* TO 'legacy_runtime'@'{host}';")
 configfile=LOCAL/'config.json'
 if configfile.exists():config=json.loads(configfile.read_text())
 else:
  config={'address':'127.0.0.1:8787','public_url':'http://127.0.0.1:8787','registry_dsn':f"lastapp_control:{state['registry_password']}@tcp(127.0.0.1:33317)/{REGISTRY}",'encryption_key':base64.b64encode(secrets.token_bytes(32)).decode(),'oidc_issuer':'http://127.0.0.1:8788','oidc_client_id':'lastapp-local','development':True,'allowed_networks':['127.0.0.0/8'],'runtime_account_host':host};write(configfile,json.dumps(config,indent=2))
 if config['registry_dsn'].rsplit('/',1)[-1] != REGISTRY:
  previous=LOCAL/'config-before-foundation.json'
  if not previous.exists():write(previous,json.dumps(config,indent=2))
  config['registry_dsn']=config['registry_dsn'].rsplit('/',1)[0]+'/'+REGISTRY
 config.setdefault('development_ui_origin','http://127.0.0.1:5173');write(configfile,json.dumps(config,indent=2))
 migrate=dict(config);migrate['registry_dsn']=root_dsn;write(LOCAL/'migrate.json',json.dumps(migrate));write(LOCAL/'provisioner.secret',state['provisioner_password'])
 if not args.skip_build:
  if not (ROOT/'web/admin/node_modules').exists():run(['npm','ci'],cwd=ROOT/'web/admin')
  run(['npm','run','build'],cwd=ROOT/'web/admin')
  run(['go','build','-o',LOCAL/'lastapp','./cmd/lastapp']);run(['go','build','-o',LOCAL/'devoidc','./cmd/devoidc'])
 if not args.skip_build:
  for name in ['lastapp','devoidc']:
   pid=LOCAL/(name+'.pid')
   if pid.exists() and alive(pid.read_text()):os.kill(int(pid.read_text()),15)
  for _ in range(100):
   if not listening(8787) and not listening(8788):break
   time.sleep(.1)
 run([LOCAL/'lastapp','migrate','-config',LOCAL/'migrate.json']);# Runtime credentials can append/read audit history but cannot change it.
 for table in ['users','sessions','plans','tenants','tenant_memberships','database_targets','database_resources','spaces','apps','microapp_installations','groups','group_memberships','local_accounts','roles','role_permissions','access_grants','permissions','entity_definitions','entity_versions','agent_grants','agent_oauth_requests','agent_token_sessions','agent_oauth_clients','development_workspaces','workspace_items','artifact_uploads','release_deployments']:
  sql(f"GRANT UPDATE,DELETE ON {REGISTRY}.`{table}` TO 'lastapp_control'@'{host}';")
 run([LOCAL/'lastapp','seed-development','-config',configfile,'-provisioner-file',LOCAL/'provisioner.secret'])
 start('devoidc',[LOCAL/'devoidc']);wait_url('http://127.0.0.1:8788/.well-known/openid-configuration')
 start('lastapp',[LOCAL/'lastapp','serve','-config',configfile]);wait_url('http://127.0.0.1:8787/readyz')
 print('Lastapp: http://127.0.0.1:8787/console/\nAccount: Platform operator\nLocal password: lastapp-local\nLogs and private persistent data: .local/')
if __name__=='__main__':main()
