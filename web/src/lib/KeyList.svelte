<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { ListTree, Regex, X } from '@lucide/svelte';
	import { onMount } from 'svelte';
	import { api, type KeyMeta } from './api';
	import KeyTree from './KeyTree.svelte';
	import { deleteOps, getErrorMessage, modifyOps, toastError } from './utils';
	import { ws } from './ws';

	interface Props {
		selected: string | null;
		onselect: (key: string) => void;
		oncreated: () => void;
		readOnly: boolean;
		prefix: string;
		dbSize: number;
	}

	let { selected, onselect, oncreated, readOnly, prefix, dbSize }: Props = $props();

	let viewMode = $state<'list' | 'tree'>('list');
	let keys = $state<KeyMeta[]>([]);
	let pattern = $state('*');
	let useRegex = $state(false);
	let regexError = $state('');
	let typeFilter = $state('');
	let sortBy = $state<'key' | 'type' | 'ttl'>('key');
	let sortAsc = $state(true);
	let loading = $state(false);
	let cursor = $state(0);
	let hasMore = $state(false);
	let showNewKey = $state(false);
	let newKeyName = $state('');
	let debounceTimer: ReturnType<typeof setTimeout> | null = null;
	let showHistory = $state(false);

	interface HistoryEntry {
		pattern: string;
		regex: boolean;
	}
	let searchHistory = $state<HistoryEntry[]>([]);

	const keyTypes = ['', 'string', 'hash', 'list', 'set', 'zset', 'stream'] as const;
	const sortOptions = [
		{ value: 'key', label: 'Name' },
		{ value: 'type', label: 'Type' },
		{ value: 'ttl', label: 'TTL' }
	] as const;

	function sortKeys(items: KeyMeta[]): KeyMeta[] {
		const sorted = [...items].sort((a, b) => {
			switch (sortBy) {
				case 'key':
					return a.key.localeCompare(b.key);
				case 'type':
					return a.type.localeCompare(b.type) || a.key.localeCompare(b.key);
				case 'ttl':
					// -1 means no TTL, sort those last (or first when descending)
					const aTtl = a.ttl < 0 ? Infinity : a.ttl;
					const bTtl = b.ttl < 0 ? Infinity : b.ttl;
					return aTtl - bTtl || a.key.localeCompare(b.key);
				default:
					return 0;
			}
		});
		return sortAsc ? sorted : sorted.reverse();
	}

	let sortedKeys = $derived(sortKeys(keys));

	const HISTORY_KEY = 'kvweb:search-history';
	const MAX_HISTORY = 20;

	function loadHistory() {
		try {
			const stored = localStorage.getItem(HISTORY_KEY);
			if (!stored) {
				searchHistory = [];
				return;
			}
			const parsed = JSON.parse(stored);
			// Migrate old string[] format to HistoryEntry[]
			if (Array.isArray(parsed) && parsed.length > 0 && typeof parsed[0] === 'string') {
				searchHistory = parsed.map((p: string) => ({ pattern: p, regex: false }));
				saveHistory();
			} else {
				searchHistory = parsed;
			}
		} catch {
			searchHistory = [];
		}
	}

	function saveHistory() {
		localStorage.setItem(HISTORY_KEY, JSON.stringify(searchHistory));
	}

	function addToHistory(p: string, isRegex: boolean) {
		if (!p || p === '*') return;
		const entry: HistoryEntry = { pattern: p, regex: isRegex };
		searchHistory = [
			entry,
			...searchHistory.filter((h) => !(h.pattern === p && h.regex === isRegex))
		].slice(0, MAX_HISTORY);
		saveHistory();
	}

	function removeFromHistory(entry: HistoryEntry) {
		searchHistory = searchHistory.filter(
			(h) => !(h.pattern === entry.pattern && h.regex === entry.regex)
		);
		saveHistory();
	}

	function clearHistory() {
		searchHistory = [];
		saveHistory();
	}

	function selectHistory(entry: HistoryEntry) {
		pattern = entry.pattern;
		useRegex = entry.regex;
		showHistory = false;
	}

	// Load history on init
	loadHistory();

	// Subscribe to WebSocket key events for live updates
	onMount(() => {
		return ws.onKeyEvent((event) => {
			if (deleteOps.has(event.op)) {
				// Remove deleted/expired keys from list
				keys = keys.filter((k) => k.key !== event.key);
			} else if (modifyOps.has(event.op)) {
				// Check if key already exists in list
				const exists = keys.some((k) => k.key === event.key);
				if (!exists) {
					// New key - reload to get metadata (type, ttl)
					loadKeys(true);
				}
				// If key exists and is selected, parent will handle refresh
			} else if (event.op === 'rename_from') {
				// Remove the old key name
				keys = keys.filter((k) => k.key !== event.key);
			} else if (event.op === 'rename_to') {
				// New key name appeared - reload to get it
				loadKeys(true);
			}
		});
	});

	// Debounced search when pattern, type filter, or regex mode changes
	$effect(() => {
		pattern; // track dependency
		typeFilter; // track dependency
		useRegex; // track dependency
		if (debounceTimer) clearTimeout(debounceTimer);
		debounceTimer = setTimeout(() => {
			loadKeys(true);
			addToHistory(pattern, useRegex);
		}, 300);
		return () => {
			if (debounceTimer) clearTimeout(debounceTimer);
		};
	});

	async function loadKeys(reset = false) {
		loading = true;
		regexError = '';
		try {
			const c = reset ? 0 : cursor;
			const result = await api.getKeys(pattern, c, 100, typeFilter || undefined, true, useRegex);
			const newKeys = result.keys as KeyMeta[];
			if (reset) {
				keys = newKeys;
			} else {
				keys = [...keys, ...newKeys];
			}
			cursor = result.cursor;
			hasMore = result.cursor !== 0;
		} catch (e) {
			const msg = getErrorMessage(e, 'Failed to load keys');
			if (useRegex && msg.includes('Invalid regex')) {
				regexError = msg;
			} else {
				toastError(e, 'Failed to load keys');
			}
		} finally {
			loading = false;
		}
	}

	async function createKey() {
		if (!newKeyName.trim()) return;
		try {
			const fullKeyName = prefix + newKeyName;
			await api.setKey(fullKeyName, '');
			newKeyName = '';
			showNewKey = false;
			await loadKeys(true);
			onselect(fullKeyName);
			oncreated();
		} catch (e) {
			toastError(e, 'Failed to create key');
		}
	}
</script>

{#if viewMode === 'tree'}
	<KeyTree {selected} {onselect} onclose={() => (viewMode = 'list')} />
{:else}
	<div class="flex h-full flex-col gap-3 p-4">
		<div class="flex items-center justify-between">
			<span class="text-xs text-muted-foreground">{dbSize} total keys</span>
		</div>
		<div class="flex gap-2">
			<div class="relative flex-1">
				<Input
					type="text"
					bind:value={pattern}
					placeholder={useRegex ? 'Regex (e.g., ^user:\\d+)' : 'Pattern (e.g., user:*)'}
					onfocus={() => (showHistory = true)}
					onblur={() => setTimeout(() => (showHistory = false), 150)}
				/>
				{#if showHistory && searchHistory.length > 0}
					<div
						class="absolute top-full right-0 left-0 z-10 mt-1 max-h-60 overflow-auto rounded border border-border bg-popover shadow-lg"
					>
						<div class="flex items-center justify-between border-b border-border px-3 py-2">
							<span class="text-xs text-muted-foreground">Recent searches</span>
							<button
								type="button"
								class="cursor-pointer text-xs text-muted-foreground hover:text-destructive"
								onmousedown={() => clearHistory()}
								title="Clear search history"
							>
								Clear all
							</button>
						</div>
						{#each searchHistory as h}
							<div class="group flex items-center hover:bg-muted">
								<button
									type="button"
									class="flex flex-1 cursor-pointer items-center gap-2 px-3 py-2 text-left font-mono text-sm"
									onmousedown={() => selectHistory(h)}
									title="Use this search pattern"
								>
									<span class="flex-1 overflow-hidden text-ellipsis">{h.pattern}</span>
									{#if h.regex}
										<span class="text-xs text-primary opacity-70">.*</span>
									{/if}
								</button>
								<button
									type="button"
									class="cursor-pointer px-2 py-1 text-muted-foreground opacity-0 group-hover:opacity-100 hover:text-destructive"
									onmousedown={() => removeFromHistory(h)}
									title="Remove from history"
								>
									<X size={14} />
								</button>
							</div>
						{/each}
					</div>
				{/if}
			</div>
			<button
				type="button"
				onclick={() => (useRegex = !useRegex)}
				class="cursor-pointer rounded border px-3 py-2 font-mono text-sm {useRegex
					? 'border-primary bg-primary/10 text-primary'
					: 'border-border bg-card hover:bg-muted'}"
				title={useRegex ? 'Regex mode (click for glob)' : 'Glob mode (click for regex)'}
			>
				<Regex size={18} />
			</button>
			<button
				type="button"
				onclick={() => (viewMode = 'tree')}
				class="cursor-pointer rounded border border-border bg-card px-3 py-2 font-mono text-sm hover:bg-muted"
				title="Switch to tree view"
			>
				<ListTree size={18} />
			</button>
		</div>
		{#if regexError}
			<div class="text-sm text-destructive">{regexError}</div>
		{/if}

		<div class="flex gap-2">
			<select
				bind:value={typeFilter}
				class="flex-1 rounded border border-border bg-card px-3 py-2 text-sm"
			>
				<option value="">All types</option>
				{#each keyTypes.slice(1) as t}
					<option value={t}>{t}</option>
				{/each}
			</select>
			<select bind:value={sortBy} class="rounded border border-border bg-card px-3 py-2 text-sm">
				{#each sortOptions as opt}
					<option value={opt.value}>Sort: {opt.label}</option>
				{/each}
			</select>
			<button
				type="button"
				onclick={() => (sortAsc = !sortAsc)}
				class="cursor-pointer rounded border border-border bg-card px-3 py-2 text-sm hover:bg-muted"
				title={sortAsc
					? 'Sorting ascending (click for descending)'
					: 'Sorting descending (click for ascending)'}
			>
				{sortAsc ? '↑' : '↓'}
			</button>
		</div>

		{#if !readOnly}
			<div class="flex gap-2">
				<Button variant="secondary" onclick={() => (showNewKey = !showNewKey)}>+ New Key</Button>
			</div>
		{/if}

		{#if showNewKey && !readOnly}
			<div class="flex gap-2 rounded bg-muted p-2">
				{#if prefix}
					<Badge variant="secondary" class="font-mono text-muted-foreground">{prefix}</Badge>
				{/if}
				<Input
					type="text"
					bind:value={newKeyName}
					placeholder="Key name"
					onkeydown={(e) => e.key === 'Enter' && createKey()}
					class="flex-1"
				/>
				<Button onclick={createKey}>Create</Button>
			</div>
		{/if}

		<ul class="flex-1 list-none overflow-y-auto">
			{#each sortedKeys as item, i (item.key)}
				{@const hasTtlBoundary =
					sortBy === 'ttl' &&
					i < sortedKeys.length - 1 &&
					item.ttl >= 0 !== sortedKeys[i + 1].ttl >= 0}
				<li class={hasTtlBoundary ? 'mb-1 border-b border-border pb-1' : ''}>
					<Button
						variant="ghost"
						class="w-full cursor-pointer justify-start overflow-hidden rounded p-2 font-mono text-sm text-ellipsis whitespace-nowrap text-foreground hover:bg-primary/10 {item.key ===
						selected
							? 'bg-primary/20 hover:bg-primary/20'
							: ''}"
						onclick={() => onselect(item.key)}
						title={`View key: ${item.key}`}
					>
						<span class="flex-1 overflow-hidden text-left text-ellipsis">{item.key}</span>
						<Badge variant="secondary" class="ml-2 text-xs opacity-60">{item.type}</Badge>
					</Button>
				</li>
			{/each}
		</ul>

		{#if hasMore}
			<Button variant="secondary" class="w-full" onclick={() => loadKeys(false)} disabled={loading}>
				{loading ? 'Loading...' : 'Load more'}
			</Button>
		{/if}

		{#if sortedKeys.length === 0 && !loading}
			<div class="py-8 text-center text-muted-foreground">No keys found</div>
		{/if}
	</div>
{/if}
