const BASE_URL = '/api';

export interface ZSetMember {
	member: string;
	score: number;
}

export interface GeoMember {
	member: string;
	longitude: number;
	latitude: number;
}

export interface StreamEntry {
	id: string;
	fields: Record<string, string>;
}

export interface HashPair {
	field: string;
	value: string;
}

export interface PaginationInfo {
	page: number;
	pageSize: number;
	total: number;
	hasMore: boolean;
}

export type KeyType = 'string' | 'list' | 'set' | 'hash' | 'zset' | 'stream' | 'hyperloglog';

export interface HLLData {
	count: number;
}

export interface KeyInfo {
	key: string;
	type: KeyType;
	value:
		| string
		| string[]
		| HashPair[]
		| Record<string, string>
		| ZSetMember[]
		| GeoMember[]
		| StreamEntry[]
		| HLLData;
	ttl: number;
	length?: number;
	pagination?: PaginationInfo;
}

export interface ServerInfo {
	info: string;
	dbSize: number;
}

export interface KeyMeta {
	key: string;
	type: string;
	ttl: number;
}

export interface KeysResponse {
	keys: string[] | KeyMeta[];
	cursor: number;
}

export interface AppConfig {
	readOnly: boolean;
	prefix: string;
	disableFlush: boolean;
}

export interface PrefixEntry {
	prefix: string;
	count: number;
	isLeaf: boolean;
	fullKey?: string;
	type?: string;
}

export interface PrefixResponse {
	entries: PrefixEntry[];
	prefix: string;
}

export interface HealthResponse {
	status: 'ok' | 'degraded';
	database: boolean;
	timestamp: number;
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
	const res = await fetch(`${BASE_URL}${path}`, {
		...options,
		headers: {
			'Content-Type': 'application/json',
			...options?.headers
		}
	});

	if (!res.ok) {
		const error = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(error.error || 'Request failed');
	}

	return res.json();
}

export const api = {
	getHealth(): Promise<HealthResponse> {
		return request('/health');
	},

	getConfig(): Promise<AppConfig> {
		return request('/config');
	},

	getInfo(section?: string): Promise<ServerInfo> {
		const params = section ? `?section=${section}` : '';
		return request(`/info${params}`);
	},

	getKeys(
		pattern = '*',
		cursor = 0,
		count = 100,
		type?: string,
		meta = false,
		regex = false
	): Promise<KeysResponse> {
		let url = `/keys?pattern=${encodeURIComponent(pattern)}&cursor=${cursor}&count=${count}`;
		if (type) url += `&type=${encodeURIComponent(type)}`;
		if (meta) url += '&meta=1';
		if (regex) url += '&regex=1';
		return request(url);
	},

	getPrefixes(prefix = '', delimiter = ':'): Promise<PrefixResponse> {
		return request(
			`/prefixes?prefix=${encodeURIComponent(prefix)}&delimiter=${encodeURIComponent(delimiter)}`
		);
	},

	getKey(key: string, page?: number, pageSize?: number): Promise<KeyInfo> {
		let url = `/key/${encodeURIComponent(key)}`;
		const params = new URLSearchParams();
		if (page !== undefined) params.set('page', page.toString());
		if (pageSize !== undefined) params.set('pageSize', pageSize.toString());
		if (params.toString()) url += `?${params.toString()}`;
		return request(url);
	},

	setKey(key: string, value: string, ttl = 0): Promise<void> {
		return request(`/key/${encodeURIComponent(key)}`, {
			method: 'PUT',
			body: JSON.stringify({ value, ttl })
		});
	},

	deleteKey(key: string): Promise<{ deleted: number }> {
		return request(`/key/${encodeURIComponent(key)}`, {
			method: 'DELETE'
		});
	},

	expireKey(key: string, ttl: number): Promise<{ ok: boolean }> {
		return request(`/key/${encodeURIComponent(key)}/expire`, {
			method: 'POST',
			body: JSON.stringify({ ttl })
		});
	},

	renameKey(key: string, newKey: string): Promise<void> {
		return request(`/key/${encodeURIComponent(key)}/rename`, {
			method: 'POST',
			body: JSON.stringify({ newKey })
		});
	},

	flushDb(): Promise<void> {
		return request('/flush', { method: 'POST' });
	},

	getNotifications(): Promise<{ enabled: boolean; value: string }> {
		return request('/notifications');
	},

	setNotifications(enabled: boolean): Promise<{ ok: boolean; enabled: boolean }> {
		return request('/notifications', {
			method: 'POST',
			body: JSON.stringify({ enabled })
		});
	},

	// List operations
	listPush(key: string, value: string, position: 'head' | 'tail'): Promise<void> {
		return request(`/key/${encodeURIComponent(key)}/list`, {
			method: 'POST',
			body: JSON.stringify({ value, position })
		});
	},

	listSet(key: string, index: number, value: string): Promise<void> {
		return request(`/key/${encodeURIComponent(key)}/list/${index}`, {
			method: 'PUT',
			body: JSON.stringify({ value })
		});
	},

	listRemove(key: string, index: number): Promise<void> {
		return request(`/key/${encodeURIComponent(key)}/list/${index}`, {
			method: 'DELETE'
		});
	},

	// Set operations
	setAdd(key: string, member: string): Promise<void> {
		return request(`/key/${encodeURIComponent(key)}/set`, {
			method: 'POST',
			body: JSON.stringify({ member })
		});
	},

	setRemove(key: string, member: string): Promise<void> {
		return request(`/key/${encodeURIComponent(key)}/set/${encodeURIComponent(member)}`, {
			method: 'DELETE'
		});
	},

	// Hash operations
	hashSet(key: string, field: string, value: string): Promise<void> {
		return request(`/key/${encodeURIComponent(key)}/hash`, {
			method: 'POST',
			body: JSON.stringify({ field, value })
		});
	},

	hashRemove(key: string, field: string): Promise<void> {
		return request(`/key/${encodeURIComponent(key)}/hash/${encodeURIComponent(field)}`, {
			method: 'DELETE'
		});
	},

	// ZSet operations
	zsetAdd(key: string, member: string, score: number): Promise<void> {
		return request(`/key/${encodeURIComponent(key)}/zset`, {
			method: 'POST',
			body: JSON.stringify({ member, score })
		});
	},

	zsetRemove(key: string, member: string): Promise<void> {
		return request(`/key/${encodeURIComponent(key)}/zset/${encodeURIComponent(member)}`, {
			method: 'DELETE'
		});
	},

	// Geo operations (view zset as coordinates)
	geoGet(key: string, page?: number, pageSize?: number): Promise<KeyInfo> {
		let url = `/key/${encodeURIComponent(key)}/geo`;
		const params = new URLSearchParams();
		if (page !== undefined) params.set('page', page.toString());
		if (pageSize !== undefined) params.set('pageSize', pageSize.toString());
		if (params.toString()) url += `?${params.toString()}`;
		return request(url);
	},

	geoAdd(key: string, member: string, longitude: number, latitude: number): Promise<void> {
		return request(`/key/${encodeURIComponent(key)}/geo`, {
			method: 'POST',
			body: JSON.stringify({ member, longitude, latitude })
		});
	},

	// Stream operations
	streamAdd(key: string, fields: Record<string, string>): Promise<{ id: string }> {
		return request(`/key/${encodeURIComponent(key)}/stream`, {
			method: 'POST',
			body: JSON.stringify({ fields })
		});
	},

	// HyperLogLog operations
	hllAdd(key: string, element: string): Promise<void> {
		return request(`/key/${encodeURIComponent(key)}/hll`, {
			method: 'POST',
			body: JSON.stringify({ element })
		});
	}
};
