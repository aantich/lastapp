import {Lastapp} from './client.js';
const el=id=>document.getElementById(id);let client;const run=async(fn)=>{try{el('status').textContent='Working…';await fn();el('status').textContent='Ready.'}catch(e){el('status').textContent=e.message}};
el('connect').onsubmit=e=>{e.preventDefault();void run(async()=>{const form=new FormData(e.target);client=new Lastapp(form.get('origin'),form.get('app'));await client.connect();const schema=await client.schema();el('entity').replaceChildren(...schema.entities.map(({entity})=>{const option=document.createElement('option');option.value=entity.key;option.textContent=entity.label;return option}));el('data').hidden=false})};
el('refresh').onclick=()=>run(async()=>{const result=await client.query(el('entity').value,{});el('records').textContent=JSON.stringify(result,null,2)});
el('create').onclick=()=>run(async()=>{el('records').textContent=JSON.stringify(await client.create(el('entity').value,JSON.parse(el('values').value)),null,2)});
const grant=()=>({permission_id:el('permission-id').value,recipient_kind:el('recipient-kind').value,recipient_id:el('recipient-id').value});
el('shares').onclick=()=>run(async()=>{el('share-result').textContent=JSON.stringify(await client.shares(el('entity').value,el('record-id').value),null,2)});
for(const [id,revoke] of [['share',false],['revoke',true]])el(id).onclick=()=>run(async()=>{el('share-result').textContent=JSON.stringify(await client.share(el('entity').value,el('record-id').value,grant(),revoke),null,2)});
