const test = require('node:test')
const assert = require('node:assert/strict')
const vm = require('node:vm')
const fs = require('node:fs')
const source = fs.readFileSync('web/assets/app.js', 'utf8')
const code = source.slice(source.indexOf('// Serialize device operations'), source.indexOf('\nasync function renderSettings(container)'))
function fixture() {
  const data = new Map(), calls = [], nodes = []
  let failPost = false, release, started
  let hold = false
  const storage = { getItem:k=>data.has(k)?data.get(k):null, setItem:(k,v)=>data.set(k,String(v)), removeItem:k=>data.delete(k) }
  const subscription = { endpoint:'https://push.test/new', toJSON(){return {endpoint:this.endpoint,keys:{auth:'a',p256dh:'b'}}}, async unsubscribe(){calls.push('unsubscribe')} }
  const registration = {pushManager:{ async getSubscription(){return subscription} }}
  const api={async get(){return {enabled:true,publicKey:'AQ'}}, async fetch(_path,options){calls.push('delete:'+JSON.parse(options.body).endpoint)},async post(){calls.push('post');if(hold)await new Promise(r=>{release=r;started()});if(failPost)throw Error('offline')}}
  const auth={user:{id:'alice'}}
  function el(tag,attrs,...children){const n={tag,attrs,children,disabled:false,addEventListener(type,fn){this[type]=fn}};nodes.push(n);return n}
  const navigator={serviceWorker:{ready:Promise.resolve(registration)}, locks:{request:async(_name,fn)=>fn()}}
  const Notification={permission:'granted',requestPermission:async()=> 'granted'}
  let tokenUser = null
  const getToken=()=>`header.${Buffer.from(JSON.stringify({uid:tokenUser || auth.user.id})).toString('base64url')}.signature`
  const ctx={auth,api,el,getToken,localStorage:storage,navigator,Notification,window:{PushManager:function(){},Notification,isSecureContext:true},atob:s=>Buffer.from(s,'base64').toString('binary'),Uint8Array,Promise,Set,Error}
  vm.createContext(ctx);vm.runInContext(code,ctx)
  return {ctx,data,calls,nodes,auth,Notification,setTokenUser(v){tokenUser=v},setFail(v){failPost=v},holdPost(){hold=true;return new Promise(r=>started=r)},release(){release()}}
}
function buttons(f){return {enable:f.nodes.find(n=>n.children.includes('Ativar notificações')),disable:f.nodes.find(n=>n.children.includes('Desativar notificações'))}}
const key=id=>'pm_push_state:'+id

test('falha de cadastro preserva endpoint para nova tentativa',async()=>{
 const f=fixture();f.data.set('pm_push_owner','alice');f.data.set(key('alice'),JSON.stringify({enabled:true,endpoint:'https://push.test/old'}));f.setFail(true)
 await f.ctx.reconcilePushSubscription('alice');let state=JSON.parse(f.data.get(key('alice')));assert.equal(state.endpoint,'https://push.test/old');assert.equal(state.pendingEndpoint,'https://push.test/new')
 f.setFail(false);await f.ctx.reconcilePushSubscription('alice');state=JSON.parse(f.data.get(key('alice')));assert.equal(state.endpoint,'https://push.test/new');assert.equal(state.pendingEndpoint,undefined)
})

test('desativar durante cadastro remove a inscrição depois da resposta',async()=>{
 const f=fixture();f.data.set('pm_push_owner','alice');f.data.set(key('alice'),JSON.stringify({enabled:true,endpoint:'https://push.test/old'}));f.ctx.notificationSettingsCard();const b=buttons(f)
 const started=f.holdPost();const recon=f.ctx.reconcilePushSubscription('alice');await started
 const disabling=b.disable.click();assert.equal(JSON.parse(f.data.get(key('alice'))).enabled,false);f.release();await Promise.all([recon,disabling]);assert.equal(JSON.parse(f.data.get(key('alice'))).enabled,false);assert(f.calls.includes('delete:https://push.test/new'));assert(f.calls.includes('unsubscribe'))
})

test('outra conta não desativa dispositivo da conta anterior',async()=>{
 const f=fixture();f.data.set('pm_push_owner','alice');f.auth.user.id='bob';f.ctx.notificationSettingsCard();await buttons(f).disable.click();assert.equal(f.calls.length,0);assert.equal(f.data.get('pm_push_owner'),'alice')
})


test('desativar durante solicitação de permissão impede ativação posterior',async()=>{
 const f=fixture();f.ctx.notificationSettingsCard();const b=buttons(f);let resolve
 f.Notification.requestPermission=()=>new Promise(r=>resolve=r)
 const enabling=b.enable.click();const disabling=b.disable.click();resolve('granted');await Promise.all([enabling,disabling])
 assert.equal(JSON.parse(f.data.get(key('alice'))).enabled,false);assert(!f.calls.includes('post'))
})

test('revogação remove endpoint anterior e pendente',async()=>{
 const f=fixture();f.data.set('pm_push_owner','alice');f.data.set(key('alice'),JSON.stringify({enabled:true,endpoint:'https://push.test/old',pendingEndpoint:'https://push.test/new'}));f.Notification.permission='denied'
 await f.ctx.reconcilePushSubscription('alice');assert(f.calls.includes('delete:https://push.test/old'));assert(f.calls.includes('delete:https://push.test/new'));assert.equal(JSON.parse(f.data.get(key('alice'))).enabled,false)
})


test('token de outra conta não autoriza reconciliação da aba anterior',async()=>{
 const f=fixture();f.data.set('pm_push_owner','alice');f.data.set(key('alice'),JSON.stringify({enabled:true,endpoint:'https://push.test/old'}));f.setTokenUser('bob')
 await f.ctx.reconcilePushSubscription('alice');assert.equal(f.calls.length,0);assert.equal(JSON.parse(f.data.get(key('alice'))).endpoint,'https://push.test/old')
})
