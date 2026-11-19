import { useEffect, useState, type FormEvent, type ReactNode } from 'react'
import { api, capital, collection, message, modules, site, statuses, statusName,
  type User, type Draft, type Entry, type Page, type Status } from './api'

import { AccountHeader, Metrics } from './account'
type Counts={total:number;planned:number;active:number;done:number}

type Screen = { kind: 'list' } | { kind: 'new' } | { kind: 'detail' | 'edit' | 'delete'; entry: Entry }
const blank: Draft = { title: '', notes: '', status: 'planned' }
const date = (value: string) => new Date(value).toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })

function Editor({ initial, busy, save, cancel }: {
  initial: Draft; busy: boolean; save: (draft: Draft) => void; cancel: () => void
}) {
  const [draft, setDraft] = useState<Draft>({ title: initial.title, notes: initial.notes, status: initial.status })
  function submit(e: FormEvent) { e.preventDefault(); save(draft) }
  return <form className="form-card grid gap-5" onSubmit={submit}>
    <label>Title<input required maxLength={160} value={draft.title} onChange={e=>setDraft({...draft,title:e.target.value})}/></label>
    <div className="field"><label htmlFor="record-status">Status</label><select id="record-status" value={draft.status} onChange={e=>setDraft({...draft,status:e.target.value as Status})}>
      {statuses.map(s=><option key={s} value={s}>{statusName[s]}</option>)}</select></div>
    <div className="field"><label htmlFor="record-notes">Notes</label><textarea id="record-notes" rows={5} maxLength={4000} value={draft.notes} onChange={e=>setDraft({...draft,notes:e.target.value})}/></div>
    <div className="actions"><button className="button primary" disabled={busy || !draft.title.trim()}>Save {site.label}</button><button className="button outline" type="button" onClick={cancel} disabled={busy}>Cancel</button></div>
  </form>
}

export function Workspace({brand}: {brand: ReactNode}) {
  const [user,setUser]=useState<User|null>(null)
  const [entries,setEntries]=useState<Entry[]>([])
  const [more,setMore]=useState(false)
  const [counts,setCounts]=useState<Counts|null>(null)
  const [screen,setScreen]=useState<Screen>({kind:'list'})
  const [busy,setBusy]=useState(true)
  const [loaded,setLoaded]=useState(false)
  const [error,setError]=useState('')
  const [notice,setNotice]=useState('')

  async function reload() {
    const [page,summary]=await Promise.all([api<Page>(collection),api<Counts>('/api/dashboard?module='+encodeURIComponent(site.collection))]);setCounts(summary)
    setEntries(page.items);setMore(page.more);setLoaded(true)
  }
  async function task(fn:()=>Promise<void>) {
    setError('');setNotice('');setBusy(true)
    try{await fn()}catch(e){setError(message(e))}finally{setBusy(false)}
  }
  function start() {
    void task(async()=>{const identity=await api<User>('/api/auth/me');setUser(identity);await reload()})
  }
  useEffect(()=>{ start() }, [])
  function move(s:Screen) { setScreen(s);setError('');setNotice('') }
  function open(id:string) { void task(async()=>{const entry=await api<Entry>(collection+'/'+id);setScreen({kind:'detail',entry})}) }
  function save(draft:Draft) {
    const current=screen
    if(current.kind!=='new'&&current.kind!=='edit')return
    void task(async()=>{
      const entry=await api<Entry>(current.kind==='new'?collection:collection+'/'+current.entry.id,current.kind==='new'?'POST':'PUT',draft)
      setScreen({kind:'detail',entry});setNotice('Saved.');setLoaded(false)
      await reload()
    })
  }
  function remove() {
    if(screen.kind!=='delete')return
    const id=screen.entry.id
    void task(async()=>{await api(collection+'/'+id,'DELETE');setScreen({kind:'list'});setLoaded(false);setNotice('Deleted.');await reload()})
  }
  const back=()=>{move({kind:'list'});if(!loaded)void task(reload)}
  const onscreen=screen.kind==='detail'||screen.kind==='edit'||screen.kind==='delete'?screen.entry:null
  const heading=screen.kind==='list'?capital(site.pluralLabel):screen.kind==='new'?'New '+site.label:
    screen.kind==='edit'?'Edit '+site.label:screen.kind==='delete'?'Delete this '+site.label+'?':onscreen?.title
  return <div className="shell"><AccountHeader brand={brand} user={user}/><main className="work">
      {user&&modules.length>1&&<nav className="module-nav" aria-label="Modules">{modules.map(m=><a key={m.collection} href={'/app?module='+encodeURIComponent(m.collection)} aria-current={m.collection===site.collection?'page':undefined}>{capital(m.pluralLabel)}</a>)}</nav>}
      {screen.kind!=='list'&&<button className="link mb-5" disabled={busy} onClick={back}>← All {site.pluralLabel}</button>}
      <div className="work-heading"><h1>{heading}</h1>
        {user&&screen.kind==='list'&&<button className="button primary" disabled={busy} onClick={()=>move({kind:'new'})}>New {site.label}</button>}</div>
      {error&&<div className="error" role="alert">{error} {(!user||!loaded)&&<button disabled={busy} className="link" onClick={start}>Retry</button>}</div>}
      <div role="status" aria-live="polite">{busy?'Working…':notice}</div>
      {user&&screen.kind==='list'&&counts&&<Metrics label="Your record totals" items={[[capital(site.pluralLabel),counts.total],['Planned',counts.planned],['Active',counts.active],['Done',counts.done]]}/>}
      {user&&screen.kind==='list'&&loaded&&
        <section className="list-panel" aria-label={'Your '+site.pluralLabel}>{!entries.length?<div className="empty"><h2>No {site.pluralLabel} yet.</h2><p>Create your first {site.label} to get started.</p></div>:
          <ul>{entries.map(entry=><li className="entry-row" key={entry.id}>
            <button className="entry-title" disabled={busy} onClick={()=>open(entry.id)}>{entry.title}</button>
            <span className={'badge '+entry.status}>{statusName[entry.status]}</span>
            <span className="entry-date">{date(entry.updatedAt)}</span></li>)}</ul>}
          {more&&<button className="link p-4" disabled={busy} onClick={()=>void task(async()=>{
            const page=await api<Page>(collection+'?offset='+entries.length);setEntries([...entries,...page.items]);setMore(page.more)
          })}>Load more</button>}
        </section>}
      {user&&(screen.kind==='new'||screen.kind==='edit')&&<Editor key={screen.kind+('entry' in screen?screen.entry.id:'')} busy={busy} initial={screen.kind==='edit'?screen.entry:blank} save={save} cancel={()=>screen.kind==='edit'?move({kind:'detail',entry:screen.entry}):back()}/>}
      {screen.kind==='detail'&&<article className="form-card"><span className={'badge '+screen.entry.status}>{statusName[screen.entry.status]}</span>
        <h2 className="subheading">Notes</h2><p className="whitespace-pre-wrap break-words">{screen.entry.notes||'No notes added.'}</p>
        <div className="detail-date">Last updated {date(screen.entry.updatedAt)}</div>
        <div className="actions"><button disabled={busy} className="button primary" onClick={()=>move({kind:'edit',entry:screen.entry})}>Edit {site.label}</button>
          <button disabled={busy} className="button outline" onClick={()=>move({kind:'delete',entry:screen.entry})}>Delete {site.label}</button></div></article>}
      {screen.kind==='delete'&&<section className="form-card"><p>“{screen.entry.title}” will be permanently deleted.</p><div className="actions">
        <button disabled={busy} className="button danger" onClick={remove}>Confirm delete</button>
        <button disabled={busy} className="button outline" onClick={()=>move({kind:'detail',entry:screen.entry})}>Cancel</button></div></section>}
    </main></div>
}
