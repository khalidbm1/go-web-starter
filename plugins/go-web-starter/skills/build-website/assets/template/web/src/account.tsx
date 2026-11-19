import { useEffect, useState, type FormEvent, type ReactNode } from 'react'
import { api, message, type User } from './api'

export function useAccount() {
  const [user, setUser] = useState<User|null>(null)
  const [error,setError] = useState('')
  useEffect(()=>{api<User>('/api/auth/me').then(setUser).catch(e=>setError(message(e)))},[])
  return {user,setUser,error}
}
export function AccountHeader({brand,user}: {brand:ReactNode;user:User|null}) {
  const [error,setError]=useState('')
  const [busy,setBusy]=useState(false)
  const current=window.location.pathname
  function link(path:string,label:string) {
    return <a href={path} className="account-link" aria-current={(path===current||(path==='/app'&&current==='/dashboard'))?'page':undefined}>{label}</a>
  }
  return <><header className="top member-top">{brand}
    {user&&<nav className="account-nav" aria-label="Account">{link('/app','Dashboard')}{link('/profile','Profile')}{user.role==='admin'&&link('/admin','Admin panel')}
      <button className="link" disabled={busy} onClick={async()=>{
        setBusy(true);setError('')
        try{await api('/api/auth/logout','POST');window.location.replace('/')}catch(e){setError(message(e));setBusy(false)}
      }}>Sign out</button>
    </nav>}</header>{error&&<p className="error" role="alert">{error}</p>}</>
}
export function Metrics({items,label}: {items:[string,number][];label:string}) {
  return <dl className="metric-grid" aria-label={label}>{items.map(([key,n])=><div key={key}><dt>{key}</dt><dd>{n}</dd></div>)}</dl>
}

type Profile=User & {bio:string}
export function ProfilePage({brand}: {brand:ReactNode}){
  const [profile,setProfile]=useState<Profile|null>(null)
  const [email,setEmail]=useState(''),[displayName,setName]=useState(''),[bio,setBio]=useState('')
  const [busy,setBusy]=useState(false),[notice,setNotice]=useState(''),[error,setError]=useState('')
  function read(){setError('');api<Profile>('/api/profile').then(p=>{setProfile(p);setEmail(p.email);setName(p.displayName);setBio(p.bio)}).catch(e=>setError(message(e)))}
  useEffect(()=>{read()},[])
  async function save(e:FormEvent<HTMLFormElement>,passwordOnly:boolean){
    e.preventDefault();if(!profile)return
    const form=e.currentTarget
    const fields=new FormData(form)
    const currentPassword=String(fields.get('currentPassword')||'')
    setBusy(true);setError('');setNotice('')
    try{
      const result=await api<{signInRequired:boolean}>(passwordOnly?'/api/profile/password':'/api/profile','PUT',
        passwordOnly?{currentPassword,newPassword:String(fields.get('newPassword'))}:{email,displayName,bio,currentPassword})
      if(result.signInRequired){window.location.replace('/signin?updated=1');return}
      const updated=await api<Profile>('/api/profile');setProfile(updated);setEmail(updated.email);setName(updated.displayName);setBio(updated.bio);setNotice('Your profile was saved.')
    }catch(err){setError(message(err))}finally{setBusy(false)}
  }
  const emailChanged=profile!==null&&email.trim().toLowerCase()!==profile.email
  return <div className="shell"><AccountHeader brand={brand} user={profile}/><main className="work">
    <div className="work-heading"><h1>Your profile</h1></div>
    {error&&<p className="error" role="alert">{error} {!profile&&<button className="link" onClick={read}>Retry</button>}</p>}
    <p role="status">{notice||(!profile&&!error?'Loading profile…':'')}</p>
    {profile&&<div className="profile-layout">
      <form className="form-card grid gap-5" onSubmit={e=>void save(e,false)}>
        <h2 className="section-title">Profile details</h2>
        <label>Display name<input maxLength={80} autoComplete="name" value={displayName} onChange={e=>setName(e.target.value)}/></label>
        <div className="field"><label htmlFor="profile-bio">Bio</label><textarea id="profile-bio" maxLength={500} rows={4} value={bio} onChange={e=>setBio(e.target.value)}/></div>
        <label>Email address<input type="email" required maxLength={254} autoComplete="email" value={email} onChange={e=>setEmail(e.target.value)}/></label>
        {emailChanged&&<><label>Current password to change email<input name="currentPassword" type="password" autoComplete="current-password" required/></label><small>Changing your email signs out all sessions.</small></>}
        <button className="button primary" disabled={busy}>Save profile</button>
      </form>
      <form className="form-card grid gap-5" onSubmit={e=>void save(e,true)}>
        <h2 className="section-title">Change password</h2>
        <label>Current password<input type="password" name="currentPassword" autoComplete="current-password" required/></label>
        <label>New password<input type="password" name="newPassword" autoComplete="new-password" minLength={12} required/></label>
        <small>Use at least 12 characters. Changing your password signs out all sessions.</small>
        <button className="button outline" disabled={busy}>Change password</button>
      </form>
    </div>}</main></div>
}
