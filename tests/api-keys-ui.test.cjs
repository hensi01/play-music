const test=require('node:test'), assert=require('node:assert/strict'),vm=require('node:vm'),fs=require('node:fs')
const source=fs.readFileSync('web/assets/admin.js','utf8')
const code=source.slice(source.indexOf('function apiKeyTokenUser('),source.indexOf('function renderNotifications('))
function setup(){
 const nodes=[]
 function el(tag,attrs={},...children){const node={tag,attrs,children,hidden:false,disabled:false,innerHTML:'',value:attrs.value||'',append(...items){this.children.push(...items)}};nodes.push(node);return node}
 const api={get:async()=>[],post:async()=>({key:'pmk_example_secret',id:'one'}),del:async()=>{}}
 const context={el,api,confirm:()=>true,getToken:()=>`x.${Buffer.from(JSON.stringify({uid:'admin'})).toString('base64url')}.x`,atob:s=>Buffer.from(s,'base64').toString('binary'),refreshApp(){},navigator:{clipboard:{writeText:async()=>{}}},Date,encodeURIComponent}
 vm.createContext(context);vm.runInContext('let pendingAPIKey=null; let apiKeyCreating=false; let apiKeyMessage=""; let apiKeySession=0; function clearPendingAPIKey(){pendingAPIKey=null;apiKeyCreating=false;apiKeyMessage="";apiKeySession++}\n'+code,context)
 return {context,nodes}
}
test('chave permanece somente em memória entre renderizações e some ao trocar conta',async()=>{
 const f=setup(),wrap={append(){}};f.context.renderAPIKeys(wrap)
 const form=f.nodes.find(n=>n.tag==='form');await form.attrs.onsubmit({preventDefault(){}})
 assert(f.nodes.some(n=>n.tag==='input'&&n.attrs.value==='pmk_example_secret'))
 const count=f.nodes.length;f.context.renderAPIKeys(wrap);assert(f.nodes.slice(count).some(n=>n.tag==='input'&&n.attrs.value==='pmk_example_secret'))
 f.context.clearPendingAPIKey();const next=f.nodes.length;f.context.renderAPIKeys(wrap);assert(!f.nodes.slice(next).some(n=>n.tag==='input'&&n.attrs.value==='pmk_example_secret'))
})

test('rerenderização durante criação não gera chave duplicada',async()=>{
 const f=setup(),wrap={append(){}};let resolve,requests=0
 f.context.api.post=()=>{requests++;return new Promise(r=>resolve=r)}
 f.context.renderAPIKeys(wrap);const first=f.nodes.filter(n=>n.tag==='form').at(-1)
 const pending=first.attrs.onsubmit({preventDefault(){}})
 f.context.renderAPIKeys(wrap);const second=f.nodes.filter(n=>n.tag==='form').at(-1)
 await second.attrs.onsubmit({preventDefault(){}});assert.equal(requests,1)
 resolve({key:'pmk_one',id:'one'});await pending
})

test('resposta atrasada não expõe chave após troca de conta',async()=>{
 const f=setup(),wrap={append(){}};let resolve
 f.context.api.post=()=>new Promise(r=>resolve=r)
 f.context.renderAPIKeys(wrap);const form=f.nodes.find(n=>n.tag==='form')
 const pending=form.attrs.onsubmit({preventDefault(){}})
 f.context.getToken=()=>`x.${Buffer.from(JSON.stringify({uid:'outro'})).toString('base64url')}.x`
 resolve({key:'pmk_secret',id:'one'});await pending
 f.context.renderAPIKeys(wrap);assert(!f.nodes.some(n=>n.attrs.value==='pmk_secret'))
})


test('limpeza do armazenamento em outra aba encerra a sessão anterior',()=>{
 const appSource=fs.readFileSync('web/assets/app.js','utf8')
 const start=appSource.indexOf('// Sessions share localStorage across tabs.')
 const end=appSource.indexOf('// Lê a claim `exp`',start)
 let listener, signedOut=false
 const context={window:{addEventListener(_name,fn){listener=fn}},auth:{user:{id:'admin'}},getToken:()=>null,setAuth(value){signedOut=value.user===null},atob:s=>Buffer.from(s,'base64').toString('binary')}
 vm.runInNewContext(appSource.slice(start,end),context)
 listener({key:null});assert.equal(signedOut,true)
})

test('erro ao criar chave continua visível após renderização',async()=>{
 const f=setup(),wrap={append(){}}
 f.context.api.post=async()=>{throw new Error('Falha de conexão')}
 f.context.renderAPIKeys(wrap);const form=f.nodes.find(n=>n.tag==='form');await form.attrs.onsubmit({preventDefault(){}})
 const count=f.nodes.length;f.context.renderAPIKeys(wrap)
 assert(f.nodes.slice(count).some(n=>n.attrs.role==='status'&&n.children.includes('Falha de conexão')))
})
