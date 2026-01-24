const BASE_URL = '/api'

export interface ZSetMember {
  member: string
  score: number
}

export interface StreamEntry {
  id: string
  fields: Record<string, string>
}

export type KeyType = 'string' | 'list' | 'set' | 'hash' | 'zset' | 'stream'

export interface KeyInfo {
  key: string
  type: KeyType
  value: string | string[] | Record<string, string> | ZSetMember[] | StreamEntry[]
  ttl: number
  length?: number
}

export interface ServerInfo {
  info: string
  dbSize: number
}

export interface KeyMeta {
  key: string
  type: string
  ttl: number
}

export interface KeysResponse {
  keys: string[] | KeyMeta[]
  cursor: number
}

export interface AppConfig {
  readOnly: boolean
  prefix: string
  disableFlush: boolean
}

export interface PrefixEntry {
  prefix: string
  count: number
  isLeaf: boolean
  fullKey?: string
  type?: string
}

export interface PrefixResponse {
  entries: PrefixEntry[]
  prefix: string
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE_URL}${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
  })

  if (!res.ok) {
    const error = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(error.error || 'Request failed')
  }

  return res.json()
}

export const api = {
  getConfig(): Promise<AppConfig> {
    return request('/config')
  },

  getInfo(section?: string): Promise<ServerInfo> {
    const params = section ? `?section=${section}` : ''
    return request(`/info${params}`)
  },

  getKeys(pattern = '*', cursor = 0, count = 100, type?: string, meta = false): Promise<KeysResponse> {
    let url = `/keys?pattern=${encodeURIComponent(pattern)}&cursor=${cursor}&count=${count}`
    if (type) url += `&type=${encodeURIComponent(type)}`
    if (meta) url += '&meta=1'
    return request(url)
  },

  getPrefixes(prefix = '', delimiter = ':'): Promise<PrefixResponse> {
    return request(`/prefixes?prefix=${encodeURIComponent(prefix)}&delimiter=${encodeURIComponent(delimiter)}`)
  },

  getKey(key: string): Promise<KeyInfo> {
    return request(`/key/${encodeURIComponent(key)}`)
  },

  setKey(key: string, value: string, ttl = 0): Promise<void> {
    return request(`/key/${encodeURIComponent(key)}`, {
      method: 'PUT',
      body: JSON.stringify({ value, ttl }),
    })
  },

  deleteKey(key: string): Promise<{ deleted: number }> {
    return request(`/key/${encodeURIComponent(key)}`, {
      method: 'DELETE',
    })
  },

  expireKey(key: string, ttl: number): Promise<{ ok: boolean }> {
    return request(`/key/${encodeURIComponent(key)}/expire`, {
      method: 'POST',
      body: JSON.stringify({ ttl }),
    })
  },

  renameKey(key: string, newKey: string): Promise<void> {
    return request(`/key/${encodeURIComponent(key)}/rename`, {
      method: 'POST',
      body: JSON.stringify({ newKey }),
    })
  },

  flushDb(): Promise<void> {
    return request('/flush', { method: 'POST' })
  },
}
