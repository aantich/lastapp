#!/usr/bin/env python3
"""Write private settings for a DISPOSABLE CI MariaDB with empty root password.
Server must be bound to loopback. This does not change the normal .local stack.
Usage: ci_config.py DIR [DB_PORT] [--api-port N] [--oidc-port N] [--ui-port N] [--artifact-directory PATH]
"""
import argparse, base64, json, os, pathlib, secrets
parser=argparse.ArgumentParser()
parser.add_argument('directory');parser.add_argument('port',nargs='?',type=int,default=33317)
parser.add_argument('--api-port',type=int,default=8787);parser.add_argument('--oidc-port',type=int,default=8788);parser.add_argument('--ui-port',type=int,default=5173)
parser.add_argument('--artifact-directory')
args=parser.parse_args()
directory=pathlib.Path(args.directory).resolve()
directory.mkdir(mode=0o700,parents=True,exist_ok=True)
port=args.port
api=f'http://127.0.0.1:{args.api_port}'
state={key:secrets.token_urlsafe(32) for key in ['registry_password','provisioner_password','legacy_password']}
config={'address':f'127.0.0.1:{args.api_port}','public_url':api,'development_ui_origin':f'http://127.0.0.1:{args.ui_port}','registry_dsn':f"root@tcp(127.0.0.1:{port})/lastapp_control",'encryption_key':base64.b64encode(secrets.token_bytes(32)).decode(),'oidc_issuer':f'http://127.0.0.1:{args.oidc_port}','oidc_client_id':'lastapp-local','development':True,'allowed_networks':['127.0.0.0/8'],'runtime_account_host':'%'}
if args.artifact_directory:config['artifact_directory']=str(pathlib.Path(args.artifact_directory).resolve())
def write(name,value):
    path=directory/name
    fd=os.open(path,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
    with os.fdopen(fd,'w') as out: json.dump(value,out,indent=2)
write('state.json',state)
write('migrate.json',config)
config=dict(config,registry_dsn=f"lastapp_control:{state['registry_password']}@tcp(127.0.0.1:{port})/lastapp_control")
write('config.json',config)
