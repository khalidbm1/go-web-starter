import { useEffect, useState, type FormEvent, type ReactNode } from 'react'
import { AccountHeader, Metrics, useAccount } from './account'
import { api, message, type User } from './api'

type Account=User&{active:boolean;createdAt:string}
type Summary={users:number;active:number;disabled:number;admins:number;records:number}
type Mode={kind:'list'}|{kind:'new'}|{kind:'edit'|'delete';account:Account}
type Change={email:string;displayName:string;role:User['role'];active:boolean;password:string}
function UserForm({initial,self,busy,save,cancel}:{
  initial?:Account;self:boolean;busy:boolean;save:(v:Change)=>void;cancel:()=>void
}){
  const [v,set]=useState<Change>({email:initial?.email||'',displayName:initial?.displayName||'',role:initial?.role||'user',active:initial?.active??true,password:''})
  return <form className="form-card grid gap-5" onSubmit={e=>{e.preventDefault();save(v)}}>
    <label>Display name<input value={v.displayName} maxLength={80} onChange={e=>set({...v,displayName:e.target.value})}/></label>
    <label>Email address<input type="email" value={v.email} maxLength={254} required disabled={self} onChange={e=>set({...v,email:e.target.value})}/></label>
    <label>Role<select value={v.role} disabled={self} onChange={e=>set({...v,role:e.target.value as User['role']})}><option value="user">User</option><option value="admin">Administrator</option></select></label>
    <label>Account access<select value={v.active?'active':'disabled'} disabled={self} onChange={e=>set({...v,active:e.target.value==='active'})}><option value="active">Active</option><option value="disabled">Disabled</option></select></label>
    {!self&&<label>{initial?'Reset password (optional)':'Initial password'}<input type="password" autoComplete="new-password" value={v.password} minLength={12} required={!initial} onChange={e=>set({...v,password:e.target.value})}/></label>}
    {self&&<p>Manage your email and password in <a className="link" href="/profile">your profile</a>.</p>}
    <div className="actions"><button className="button primary" disabled={busy}>{initial?'Save account':'Create account'}</button><button type="button" className="button outline" disabled={busy} onClick={cancel}>Cancel</button></div>
  </form>
}
export function AdminPage({brand}:{brand:ReactNode}){
  const {user,error:sessionError}=useAccount()
  const [summary,setSummary]=useState<Summary|null>(null),[items,setItems]=useState<Account[]>([])
  const [more,setMore]=useState(false),[offset,setOffset]=useState(0),[search,setSearch]=useState(''),[filter,setFilter]=useState('')
  const [mode,setMode]=useState<Mode>({kind:'list'}),[busy,setBusy]=useState(false),[notice,setNotice]=useState(''),[error,setError]=useState('')
  async function refresh(q:string,start:number){
    const params=new URLSearchParams({q,offset:String(start)})
    const [s,p]=await Promise.all([api<Summary>('/api/admin/overview'),api<{items:Account[];more:boolean}>('/api/admin/users?'+params)])
    setSummary(s);setItems(p.items);setMore(p.more);setOffset(start);setFilter(q)
  }
  async function run(f:()=>Promise<void>){setBusy(true);setError('');setNotice('');try{await f()}catch(e){setError(message(e))}finally{setBusy(false)}}
  useEffect(()=>{if(user?.role==='admin')void run(()=>refresh('',0))},[user?.role])
  function view(v:Mode){setMode(v);setError('');setNotice('')}
  function save(v:Change){
    const m=mode;if(m.kind!=='new'&&m.kind!=='edit')return
    void run(async()=>{await api(m.kind==='new'?'/api/admin/users':'/api/admin/users/'+m.account.id,m.kind==='new'?'POST':'PUT',v)
      setMode({kind:'list'});await refresh(filter,0);setNotice(m.kind==='new'?'Account created.':'Account saved.')})
  }
  function remove(){
    if(mode.kind!=='delete')return
    const id=mode.account.id
    void run(async()=>{await api('/api/admin/users/'+id,'DELETE');setMode({kind:'list'});await refresh(filter,0);setNotice('Account deleted.')})
  }
  function submitSearch(e:FormEvent){e.preventDefault();void run(()=>refresh(search.trim(),0))}
  return <div className="shell"><AccountHeader brand={brand} user={user}/><main className="work">
    <div className="work-heading"><h1>{mode.kind==='new'?'Create user':mode.kind==='edit'?'Manage user':mode.kind==='delete'?'Delete account?':'Admin dashboard'}</h1></div>
    {(sessionError||error)&&<p role="alert" className="error">{sessionError||error} <button className="link" onClick={()=>window.location.reload()}>Reload</button></p>}
    {user&&user.role!=='admin'&&<section className="form-card"><h2 className="section-title">Administrator access required</h2><a href="/app" className="link">Return to your dashboard</a></section>}
    {(!user&&!sessionError||busy||notice)&&<p role="status">{busy?'Working…':notice||'Loading account…'}</p>}
    {user?.role==='admin'&&mode.kind==='list'&&<>
      {summary&&<Metrics label="Site totals" items={[[ 'Accounts',summary.users],['Active accounts',summary.active],['Disabled',summary.disabled],['Administrators',summary.admins],['Total records',summary.records]]}/>}
      <section className="admin-panel" aria-label="User management">
        <div className="work-heading"><h2 className="section-title">User management</h2><button className="button primary" disabled={busy} onClick={()=>view({kind:'new'})}>Create user</button></div>
        <form className="search-form" onSubmit={submitSearch}><label>Find users<input type="search" maxLength={160} value={search} onChange={e=>setSearch(e.target.value)} placeholder="Email or display name"/></label><button className="button outline" disabled={busy}>Search</button></form>
        <ul className="list-panel">{items.map(v=><li key={v.id} className="user-row"><div className="user-id"><strong>{v.displayName||v.email}</strong>{v.displayName&&<small>{v.email}</small>}</div><div className="tags"><span className={'badge '+(v.active?'active':'done')}>{v.active?'Active':'Disabled'}</span><small>{v.role==='admin'?'Administrator':'User'}</small></div>
          <button className="link" aria-label={'Manage '+v.email} disabled={busy} onClick={()=>void run(async()=>{const latest=await api<Account>('/api/admin/users/'+v.id);setMode({kind:'edit',account:latest})})}>Manage</button></li>)}</ul>
        {summary&&!busy&&!items.length&&<p className="empty">No users match this search.</p>}
        {(offset>0||more)&&<div className="actions mt-4">
          <button className="button outline" disabled={busy||offset===0} onClick={()=>void run(()=>refresh(filter,Math.max(0,offset-25)))}>Previous</button>
          <button className="button outline" disabled={busy||!more} onClick={()=>void run(()=>refresh(filter,offset+25))}>Next</button></div>}
      </section></>}
    {user?.role==='admin'&&(mode.kind==='new'||mode.kind==='edit')&&<>
      <UserForm key={mode.kind==='edit'?mode.account.id:'new'} initial={mode.kind==='edit'?mode.account:undefined} self={mode.kind==='edit'&&mode.account.id===user.id} busy={busy} save={save} cancel={()=>view({kind:'list'})}/>
      {mode.kind==='edit'&&mode.account.id!==user.id&&<button className="button danger mt-5" disabled={busy} onClick={()=>view({kind:'delete',account:mode.account})}>Delete account</button>}
    </>}
    {user?.role==='admin'&&mode.kind==='delete'&&<section className="form-card"><p>This permanently deletes {mode.account.email}, their sessions, and their records.</p><div className="actions">
      <button className="button danger" disabled={busy} onClick={remove}>Confirm delete account</button>
      <button className="button outline" disabled={busy} onClick={()=>view({kind:'edit',account:mode.account})}>Cancel</button></div></section>}
  </main></div>
}
