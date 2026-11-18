import { useState, type FormEvent } from 'react'
import { createRoot } from 'react-dom/client'
import { api, capital, message, site, statusName, type Status } from './api'
import { Workspace } from './workspace'
import './style.css'

export function Brand() {
  return <a className="brand" href="/" aria-label={site.name + ' home'}><span className="brand-mark" aria-hidden="true"><i/><i/><i/></span>{site.name}</a>
}

function Landing() {
  const examples: Status[] = ['active', 'planned', 'done']
  return <div className="shell">
    <header className="top"><Brand/><nav aria-label="Account"><a className="link" href="/signin">Sign in</a></nav></header>
    <main className="hero">
      <section className="hero-copy">
        <h1>A clearer view of your <em>{site.pluralLabel}.</em></h1>
        <p>Keep notes, track progress, and find every {site.label} in one private workspace.</p>
        <div className="actions"><a className="button primary" href="/signup">Create an account <span aria-hidden="true">↗</span></a><a className="button outline" href="/app">Open workspace</a></div>
      </section>
      <figure className="preview" aria-label={'An example of the ' + site.pluralLabel + ' workspace'}>
        <figcaption>Workspace preview</figcaption>
        <div className="paper">
          <div className="paper-top"><span className="mini-mark" aria-hidden="true"></span><span>{site.name}</span></div>
          <h2>{capital(site.pluralLabel)}</h2>
          <div className="preview-rows">{site.examples.slice(0,3).map((example,i)=><div className="preview-row" key={i}><span>{example}</span><span className={'badge '+examples[i]}>{statusName[examples[i]]}</span></div>)}</div>
        </div>
      </figure>
    </main>
  </div>
}

function Auth({ register }: { register: boolean }) {
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const data = new FormData(e.currentTarget)
    setError(''); setBusy(true)
    try {
      await api('/api/auth/' + (register ? 'register' : 'login'), 'POST',
        { email: String(data.get('email')), password: String(data.get('password')) })
      window.location.assign('/app')
    } catch (err) { setError(message(err)); setBusy(false) }
  }
  return <div className="shell"><header className="top"><Brand/><a href="/" className="link">Home</a></header>
    <main className="auth-panel"><h1>{register ? 'Create an account' : 'Welcome back.'}</h1>
      <p>{register ? 'Start your private workspace.' : 'Sign in to your workspace.'}</p>
      <form className="grid gap-5" onSubmit={submit}>
        <label>Email address<input type="email" name="email" autoComplete="email" maxLength={254} required/></label>
        <label>Password<input type="password" name="password" autoComplete={register ? 'new-password' : 'current-password'} minLength={register ? 12 : undefined} required aria-describedby={register ? 'password-help' : undefined}/></label>
        {register && <small id="password-help">Use at least 12 characters.</small>}
        {error && <p className="error" role="alert">{error}</p>}
        <button className="button primary" disabled={busy}>{busy ? 'Please wait…' : register ? 'Create account' : 'Sign in'}</button>
      </form>
      <a className="link" href={register ? '/signin' : '/signup'}>{register ? 'Already have an account? Sign in' : 'Create an account'}</a>
    </main></div>
}

const path = window.location.pathname
const page = path === '/app' ? <Workspace brand={<Brand/>}/> :
  path === '/signin' || path === '/signup' ? <Auth register={path === '/signup'}/> : <Landing/>
createRoot(document.getElementById('root')!).render(page)
