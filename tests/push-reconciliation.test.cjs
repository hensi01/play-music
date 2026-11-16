const test = require('node:test')
const assert = require('node:assert/strict')
const vm = require('node:vm')
const fs = require('node:fs')
const source = fs.readFileSync('web/assets/app.js', 'utf8')
const code = source.slice(source.indexOf('// Avoid leaving the activation screen'), source.indexOf('\nasync function renderSettings(container)'))
function fixture() {
  const data = new Map(), calls = [], nodes = []
  let failPost = false, release, started
  let hold = false
  const storage = { getItem:k=>data.has(k)?data.get(k):null, setItem:(k,v)=>data.set(k,String(v)), removeItem:k=>data.delete(k) }
  const subscription = { endpoint:'https://push.test/new', toJSON(){return {endpoint:this.endpoint,keys:{auth:'a',p256dh:'b'}}}, async unsubscribe(){calls.push('unsubscribe')} }
  const registration = {pushManager:{ async getSubscription(){return subscription} }}
  const api={async get(){return {enabled:true,publicKey:'AQ'}}, async fetch(path,options){if(options.method==='GET') return this.get(path);if(options.method==='POST') return this.post(path,JSON.parse(options.body));calls.push('delete:'+JSON.parse(options.body).endpoint)},async post(path){if(path === '/api/push/status') return {active:true};calls.push('post');if(hold)await new Promise(r=>{release=r;started()});if(failPost)throw Error('offline')}}
  const auth={user:{id:'alice'}}
  function el(tag,attrs,...children){const n={tag,attrs,children,disabled:false,addEventListener(type,fn){this[type]=fn}};nodes.push(n);return n}
  const navigator={serviceWorker:{ready:Promise.resolve(registration)}, locks:{request:async(_name,fn)=>fn()}}
  const Notification={permission:'granted',requestPermission:async()=> 'granted'}
  let tokenUser = null
  const getToken=()=>`header.${Buffer.from(JSON.stringify({uid:tokenUser || auth.user.id})).toString('base64url')}.signature`
  const ctx={auth,api,el,getToken,localStorage:storage,setTimeout,clearTimeout,AbortController,navigator,Notification,document:{addEventListener(){}},window:{PushManager:function(){},Notification,isSecureContext:true,location:{hash:'#/'},addEventListener(){}}, notificationGateSeq:0,notificationGateNeedsRedirect:false,notificationGate:{},navigate(path){ calls.push('navigate:'+path) },render(){},player:{clearSession(){calls.push('clearPlayer')}},loadPlaylists:async()=>{},atob:s=>Buffer.from(s,'base64').toString('binary'),Uint8Array,Promise,Set,Error}
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
 await new Promise(setImmediate)
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


test('acesso só é liberado após verificar inscrição do aparelho', async () => {
 const f=fixture(); f.data.set('pm_push_owner','alice'); f.data.set(key('alice'),JSON.stringify({enabled:true,endpoint:'https://push.test/new'}))
 await f.ctx.verifyNotificationGate('alice'); assert.equal(f.ctx.notificationGate.status,'allowed')
 f.Notification.permission='denied'; await f.ctx.verifyNotificationGate('alice'); assert.equal(f.ctx.notificationGate.status,'required')
})

test('administrador sem notificações não tem reprodução interrompida', async () => {
 const f=fixture(); f.auth.user.isAdmin=true; f.Notification.permission='denied'
 await f.ctx.verifyNotificationGate('alice')
 assert.equal(f.ctx.notificationGate.status,'required'); assert(!f.calls.includes('clearPlayer'))
})

test('verificação antiga não substitui estado da conta atual', async () => {
 const f=fixture(); f.auth.user.id='bob'; f.ctx.notificationGate={userId:'bob',status:'allowed'}
 await f.ctx.verifyNotificationGate('alice'); assert.equal(f.ctx.notificationGate.userId,'bob'); assert.equal(f.ctx.notificationGate.status,'allowed')
})

test('service worker indisponível devolve tela recuperável', async () => {
 const f=fixture(); f.data.set('pm_push_owner','alice'); f.data.set(key('alice'),JSON.stringify({enabled:true,endpoint:'https://push.test/new'}))
 f.ctx.navigator.serviceWorker.ready=new Promise(()=>{}); f.ctx.setTimeout=(fn)=>{fn();return 1}; f.ctx.clearTimeout=()=>{}
 await f.ctx.verifyNotificationGate('alice'); assert.equal(f.ctx.notificationGate.status,'required')
})

test('falha de rede durante verificação devolve tela recuperável', async () => {
 const f=fixture(); f.data.set('pm_push_owner','alice'); f.data.set(key('alice'),JSON.stringify({enabled:true,endpoint:'https://push.test/new'}))
 f.ctx.api.fetch=(_path,opts)=>new Promise((_,reject)=>opts.signal.addEventListener('abort',()=>reject(new Error('timeout'))))
 f.ctx.setTimeout=(fn)=>{queueMicrotask(fn);return 1}; f.ctx.clearTimeout=()=>{}
 await f.ctx.verifyNotificationGate('alice'); assert.equal(f.ctx.notificationGate.status,'required')
})


test('após ativação confirmada o usuário vai à página inicial', async () => {
 const f=fixture(); f.data.set('pm_push_owner','alice'); f.data.set(key('alice'),JSON.stringify({enabled:true,endpoint:'https://push.test/new'}))
 f.ctx.notificationGateNeedsRedirect=true; f.ctx.window.location.hash='#/search'
 await f.ctx.verifyNotificationGate('alice')
 assert.equal(f.ctx.notificationGate.status,'allowed'); assert(f.calls.includes('navigate:/'))
})

test('permissão recusada não libera nem redireciona', async () => {
 const f=fixture(); f.data.set('pm_push_owner','alice'); f.data.set(key('alice'),JSON.stringify({enabled:true,endpoint:'https://push.test/new'}))
 f.ctx.notificationGateNeedsRedirect=true; f.ctx.window.location.hash='#/search'; f.Notification.permission='denied'
 await f.ctx.verifyNotificationGate('alice')
 assert.equal(f.ctx.notificationGate.status,'required'); assert(!f.calls.includes('navigate:/'))
})

function visibleText(node) {
  if (node == null) return ''
  if (typeof node === 'string') return node
  return (node.children || []).map(visibleText).join(' ')
}

test('inscrição confirmada mostra estado ativo e não oferece ativação novamente',()=>{
 const f=fixture();f.data.set('pm_push_owner','alice');f.data.set(key('alice'),JSON.stringify({enabled:true,endpoint:'https://push.test/new'}));f.ctx.notificationGate={userId:'alice',status:'allowed'}
 const text=visibleText(f.ctx.notificationSettingsCard())
 assert.match(text,/notificações já estão ativadas/i);assert.doesNotMatch(text,/Ativar notificações/)
})

test('instalação sem permissão não é apresentada como notificações ativadas',()=>{
 const f=fixture();f.ctx.window.navigator={standalone:true};f.Notification.permission='default';f.ctx.notificationGate={userId:'alice',status:'required'}
 const text=visibleText(f.ctx.notificationSettingsCard(true))
 assert.match(text,/Ativar notificações/);assert.doesNotMatch(text,/já estão ativadas/i)
})
