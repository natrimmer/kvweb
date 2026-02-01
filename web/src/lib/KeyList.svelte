<script lang="ts">
	import * as Alert from '$lib/components/ui/alert';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as ButtonGroup from '$lib/components/ui/button-group';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import {
		ArrowUpFromDot,
		CircleAlert,
		CirclePlus,
		CircleX,
		Funnel,
		Info,
		ListTree,
		Regex,
		Settings
	} from '@lucide/svelte';
	import { onMount } from 'svelte';
	import { api, type KeyMeta } from './api';
	import KeyTree from './KeyTree.svelte';
	import SearchHistory, { type HistoryEntry } from './SearchHistory.svelte';
	import ServerSettings from './ServerSettings.svelte';
	import { deleteOps, getErrorMessage, modifyOps, toastError } from './utils';
	import { ws } from './ws';

	interface Props {
		selected: string | null;
		onselect: (key: string) => void;
		oncreated: () => void;
		readOnly: boolean;
		prefix: string;
		dbSize: number;
		disableFlush?: boolean;
	}

	let {
		selected,
		onselect,
		oncreated,
		readOnly,
		prefix,
		dbSize,
		disableFlush = false
	}: Props = $props();

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
	let showSettings = $state(false);
	let newKeyName = $state('');
	let debounceTimer: ReturnType<typeof setTimeout> | null = null;
	let showHistory = $state(false);
	let searchHistoryRef: SearchHistory | undefined = $state();
	let showFilters = $state(false);
	let inputRef: HTMLInputElement | null = $state(null);

	const keyTypes = [
		{ value: '', label: 'All types' },
		{ value: 'string', label: 'string' },
		{ value: 'hash', label: 'hash' },
		{ value: 'list', label: 'list' },
		{ value: 'set', label: 'set' },
		{ value: 'zset', label: 'zset' },
		{ value: 'geo', label: 'geo (zset)' },
		{ value: 'stream', label: 'stream' },
		{ value: 'hyperloglog', label: 'hyperloglog' }
	] as const;
	const sortOptions = [
		{ value: 'key', label: 'Name' },
		{ value: 'type', label: 'Type' },
		{ value: 'ttl', label: 'TTL' }
	] as const;

	let typeFilterLabel = $derived(
		keyTypes.find((t) => t.value === typeFilter)?.label ?? 'All types'
	);
	let sortByLabel = $derived(sortOptions.find((s) => s.value === sortBy)?.label ?? 'Name');

	function handleTypeFilterChange(value: string | undefined) {
		if (value !== undefined) {
			typeFilter = value;
		}
	}

	function handleSortByChange(value: string | undefined) {
		if (value === 'key' || value === 'type' || value === 'ttl') {
			sortBy = value;
		}
	}

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

	function selectHistory(entry: HistoryEntry) {
		pattern = entry.pattern;
		useRegex = entry.regex;
		showHistory = false;
	}

	function clearInput() {
		pattern = '';
		inputRef?.focus();
	}

	// Subscribe to WebSocket key events for live updates
	onMount(() => {
		return ws.onKeyEvent((event) => {
			if (deleteOps.has(event.op)) {
				// Remove deleted/expired keys from list
				keys = keys.filter((k) => k.key !== event.key);
			} else if (modifyOps.has(event.op) || event.op === 'rename_to') {
				// Key modified or renamed - reload to get updated metadata
				loadKeys(true);
			} else if (event.op === 'rename_from') {
				// Remove the old key name
				keys = keys.filter((k) => k.key !== event.key);
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
			searchHistoryRef?.addToHistory(pattern, useRegex);
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
			// "geo" filter uses "zset" since geo data is stored as zset
			const actualTypeFilter = typeFilter === 'geo' ? 'zset' : typeFilter;
			const result = await api.getKeys(
				pattern,
				c,
				100,
				actualTypeFilter || undefined,
				true,
				useRegex
			);
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
	<div class="flex h-full flex-col p-4">
		<div class="mb-3 flex gap-2">
			<div class="relative flex-1">
				<Input
					bind:ref={inputRef}
					type="text"
					bind:value={pattern}
					placeholder={useRegex ? 'Regex (e.g., ^user:\\d+)' : 'Pattern (e.g., user:*)'}
					onfocus={() => (showHistory = true)}
					onblur={() => setTimeout(() => (showHistory = false), 150)}
					class="pr-9"
					title="Search keys by pattern"
					aria-label="Search keys by pattern"
				/>
				{#if pattern && pattern !== '*'}
					<Button
						variant="ghost"
						size="icon"
						onclick={clearInput}
						aria-label="Clear search input"
						title="Clear search input"
						class="absolute inset-y-0 right-0 rounded-l-none text-muted-foreground hover:bg-transparent focus-visible:ring-ring/50"
					>
						<CircleX size={18} />
					</Button>
				{/if}
				<SearchHistory bind:this={searchHistoryRef} show={showHistory} onselect={selectHistory} />
			</div>
			<ButtonGroup.Root>
				<Button
					variant="outline"
					onclick={() => (useRegex = !useRegex)}
					class="cursor-pointer {useRegex ? 'bg-accent' : ''}"
					title={useRegex ? 'Regex mode (click for glob)' : 'Glob mode (click for regex)'}
					aria-label={useRegex ? 'Regex mode (click for glob)' : 'Glob mode (click for regex)'}
				>
					<Regex size={18} />
				</Button>
				<Button
					variant="outline"
					onclick={() => (viewMode = 'tree')}
					class="cursor-pointer"
					title="Switch to tree view"
					aria-label="Switch to tree view"
				>
					<ListTree size={18} />
				</Button>
				<Button
					variant="outline"
					onclick={() => (showFilters = !showFilters)}
					class="cursor-pointer {showFilters ? 'bg-accent' : ''}"
					title="Toggle filters"
					aria-label="Toggle filters"
				>
					<Funnel size={18} />
				</Button>
			</ButtonGroup.Root>
		</div>

		{#if showFilters}
			<ButtonGroup.Root class="mb-3 w-full">
				<Select.Root type="single" value={typeFilter} onValueChange={handleTypeFilterChange}>
					<Select.Trigger class="flex-1">
						{typeFilterLabel}
					</Select.Trigger>
					<Select.Content>
						{#each keyTypes as t}
							<Select.Item value={t.value}>{t.label}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
				<Select.Root type="single" value={sortBy} onValueChange={handleSortByChange}>
					<Select.Trigger class="w-32">
						Sort: {sortByLabel}
					</Select.Trigger>
					<Select.Content>
						{#each sortOptions as opt}
							<Select.Item value={opt.value}>{opt.label}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
				<Button
					variant="outline"
					onclick={() => (sortAsc = !sortAsc)}
					class="cursor-pointer"
					title={sortAsc
						? 'Sorting ascending (click for descending)'
						: 'Sorting descending (click for ascending)'}
					aria-label={sortAsc
						? 'Sorting ascending (click for descending)'
						: 'Sorting descending (click for ascending)'}
				>
					{#if sortAsc}
						<ArrowUpFromDot size={16} />
					{:else}
						<ArrowUpFromDot size={16} class="rotate-180" />
					{/if}
				</Button>
			</ButtonGroup.Root>
		{/if}

		<div class="mb-3 flex items-center justify-between">
			<span class="text-sm text-muted-foreground">
				{#if pattern !== '*' || typeFilter}
					{sortedKeys.length} of {dbSize} key{dbSize === 1 ? '' : 's'}
				{:else}
					{dbSize} total key{dbSize === 1 ? '' : 's'}
				{/if}
			</span>
			<span>
				{#if !readOnly}
					<Button
						variant="outline"
						size="sm"
						class="cursor-pointer hover:bg-accent"
						onclick={() => (showNewKey = !showNewKey)}
					>
						<CirclePlus /> New Key
					</Button>
				{/if}
			</span>
		</div>

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
					title="New key name"
					aria-label="New key name"
				/>
				<Button
					onclick={createKey}
					disabled={!newKeyName.trim()}
					title="Create new key"
					aria-label="Create new key"
				>
					Create
				</Button>
			</div>
		{/if}

		{#if typeFilter === 'geo'}
			<div class="mb-3 text-xs text-muted-foreground">
				Geo data is stored as sorted sets. Use "View as Geo" toggle in the editor to see
				coordinates.
			</div>
		{/if}

		{#if regexError}
			<Alert.Root variant="destructive" class="mb-3 border-destructive bg-background">
				<CircleAlert />
				<Alert.Title>Regex Error</Alert.Title>
				<Alert.Description>
					{regexError}
				</Alert.Description>
			</Alert.Root>
		{/if}

		<ul class="flex-1 list-none overflow-y-auto border-t border-muted py-1">
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
						aria-label={`View key: ${item.key}`}
					>
						<span class="flex-1 overflow-hidden text-left text-ellipsis">{item.key}</span>
						<Badge variant="secondary" class="ml-2 text-xs opacity-60">{item.type}</Badge>
					</Button>
				</li>
			{/each}
		</ul>

		{#if hasMore}
			<Button
				variant="secondary"
				class="w-full"
				onclick={() => loadKeys(false)}
				disabled={loading}
				title="Load more keys"
				aria-label="Load more keys"
			>
				{loading ? 'Loading...' : 'Load more'}
			</Button>
		{/if}

		{#if sortedKeys.length === 0 && !loading}
			<div class="py-8 text-center text-muted-foreground">No keys found</div>
		{/if}

		<!-- Footer with about and settings -->
		<div class="mt-0 flex items-center justify-between border-t border-border pt-3 text-xs">
			<a
				href="/kvweb"
				class="flex items-center gap-1.5 text-muted-foreground hover:text-foreground hover:underline"
				title="Learn more about kvweb"
				aria-label="Learn more about kvweb"
			>
				<Info size={14} />
				<span>About kvweb</span>
			</a>
			<Button
				variant="ghost"
				size="sm"
				class="h-7 cursor-pointer text-muted-foreground hover:text-foreground"
				onclick={() => (showSettings = true)}
				title="Settings and server info"
				aria-label="Settings and server info"
			>
				<Settings size={14} />
			</Button>
		</div>
	</div>
{/if}

<Dialog.Root bind:open={showSettings}>
	<Dialog.Content class="flex max-h-[80vh] min-w-3xl flex-col">
		<Dialog.Header>
			<Dialog.Title>Server Settings</Dialog.Title>
			<Dialog.Description>View server information and manage database settings</Dialog.Description>
		</Dialog.Header>
		<div class="min-h-0 flex-1">
			<ServerSettings {readOnly} {disableFlush} />
		</div>
	</Dialog.Content>
</Dialog.Root>
