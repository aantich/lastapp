#!/usr/bin/env python3
"""Write private settings for a DISPOSABLE CI MariaDB with empty root password.
Server must be bound to loopback. This does not change the normal .local stack.
"""
import base64, json, os, pathlib, secrets, sys
directory=pathlib.Path(sys.argv[1]).resolve()
directory.mkdir(mode=0o700,parents=True,exist_ok=True)
port=int(sys.argv[2]) if len(sys.argv)>2 else 33317
state={key:secrets.token_urlsafe(32) for key in ['registry_password','provisioner_password','legacy_password']}
config={'address':'127.0.0.1:8787','public_url':'http://127.0.0.1:8787','development_ui_origin':'http://127.0.0.1:5173','registry_dsn':f"root@tcp(127.0.0.1:{port})/lastapp_control",'encryption_key':base64.b64encode(secrets.token_bytes(32)).decode(),'oidc_issuer':'http://127.0.0.1:8788','oidc_client_id':'lastapp-local','development':True,'allowed_networks':['127.0.0.0/8'],'runtime_account_host':'%'}
def write(name,value):
    path=directory/name
    fd=os.open(path,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
    with os.fdopen(fd,'w') as out: json.dump(value,out,indent=2)
write('state.json',state)
write('migrate.json',config)
config=dict(config,registry_dsn=f"lastapp_control:{state['registry_password']}@tcp(127.0.0.1:{port})/lastapp_control")
write('config.json',config)
