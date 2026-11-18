import spec from '../../site.json'

type Module = {resource:string;collection:string;label:string;pluralLabel:string}
const config=spec as Module & {name:string;examples:string[];resources?:Module[]}
export const modules:Module[]=[config,...(config.resources||[])]
const requested=new URLSearchParams(window.location.search).get('module')
const active=['/app','/dashboard'].includes(window.location.pathname)?modules.find(m=>m.collection===requested):undefined
export const site={...config,...active}
export const collection = '/api/' + site.collection
export const statuses = ['planned', 'active', 'done'] as const
export type Status = typeof statuses[number]
export const statusName: Record<Status, string> = { planned: 'Planned', active: 'Active', done: 'Done' }
export type User = { id: string; email: string; role: 'user'|'admin'; displayName: string }
export type Draft = { title: string; notes: string; status: Status }
export type Entry = Draft & { id: string; createdAt: string; updatedAt: string }
export type Page = { items: Entry[]; more: boolean }
export const capital = (s: string) => s.charAt(0).toLocaleUpperCase() + s.slice(1)

export class APIError extends Error {
  constructor(public status: number, message: string) { super(message) }
}
export function message(error: unknown): string {
  return error instanceof Error ? error.message : 'The request could not be completed.'
}
export async function api<T>(url: string, method = 'GET', data?: unknown): Promise<T> {
  const headers: Record<string, string> = { Accept: 'application/json' }
  if (method !== 'GET') headers['X-Requested-With'] = 'GoWebStarter'
  if (data !== undefined) headers['Content-Type'] = 'application/json'
  const res = await fetch(url, { method, headers, credentials: 'same-origin',
    body: data === undefined ? undefined : JSON.stringify(data) })
  if (!res.ok) {
    const error = await res.json().catch(() => ({}))
    if (res.status === 401 && ['/app','/dashboard','/profile','/admin'].includes(window.location.pathname)) window.location.replace('/signin')
    throw new APIError(res.status, error.error || 'The request could not be completed.')
  }
  return res.status === 204 ? undefined as T : res.json() as Promise<T>
}
