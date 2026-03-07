<script lang="ts">
	import * as Alert from '$lib/components/ui/alert';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as ButtonGroup from '$lib/components/ui/button-group';
	import * as Empty from '$lib/components/ui/empty';
	import { Input } from '$lib/components/ui/input';
	import * as Kbd from '$lib/components/ui/kbd';
	import * as Select from '$lib/components/ui/select';
	import AboutDialog from '$lib/dialogs/AboutDialog.svelte';
	import AddKeyDialog from '$lib/dialogs/AddKeyDialog.svelte';
	import BulkDeleteDialog from '$lib/dialogs/BulkDeleteDialog.svelte';
	import PaletteDialog from '$lib/dialogs/PaletteDialog.svelte';
	import ServerSettingsDialog from '$lib/dialogs/ServerSettingsDialog.svelte';
	import {
		ArrowUpFromDot,
		CircleAlert,
		CirclePlus,
		CircleX,
		DatabaseZap,
		Dot,
		Folder,
		Funnel,
		House,
		Info,
		ListTree,
		MoveLeft,
		Palette,
		Regex,
		Search,
		Settings,
		SquareTerminal,
		Trash2
	} from '@lucide/svelte';
	import { onMount, untrack } from 'svelte';
	import { api, type KeyMeta } from './api';
	import SearchHistory, { type HistoryEntry } from './SearchHistory.svelte';
	import { deleteOps, formatBytes, getErrorMessage, modifyOps, toastError } from './utils';
	import { ws } from './ws';

	interface Props {
		selected: string | null;
		onselect: (key: string) => void;
		oncreated: () => void;
		readOnly: boolean;
		prefix: string;
		dbSize: number;
		usedMemoryHuman: string;
		disableFlush?: boolean;
		version: string;
		commit: string;
		dirty: boolean;
		consoleVisible?: boolean;
		onToggleConsole?: () => void;
	}

	let {
		selected,
		onselect,
		oncreated,
		readOnly,
		prefix,
		dbSize,
		usedMemoryHuman,
		disableFlush = false,
		version,
		commit,
		dirty,
		consoleVisible = false,
		onToggleConsole
	}: Props = $props();

	let viewMode = $state<'list' | 'tree'>('list');
	let keys = $state<KeyMeta[]>([]);
	let pattern = $state('*');
	let useRegex = $state(false);
	let regexError = $state('');
	let typeFilter = $state('all');
	let sortBy = $state<'key' | 'type' | 'ttl' | 'memory'>('key');
	let sortAsc = $state(true);
	let loading = $state(false);
	let cursor = $state(0);
	let hasMore = $state(false);
	let showAbout = $state(false);
	let showAddDialog = $state(false);
	let showPalette = $state(false);
	let showSettings = $state(false);
	let debounceTimer: ReturnType<typeof setTimeout> | null = null;
	let showHistory = $state(false);
	let searchHistoryRef: SearchHistory | undefined = $state();
	let showFilters = $state(false);
	let inputRef: HTMLInputElement | null = $state(null);
	let selectedKeys = $state<Set<string>>(new Set());
	let lastClickedKey = $state<string | null>(null);
	let lastClickedTree = $state<string | null>(null);
	let showBulkDelete = $state(false);
	let currentPrefix = $state('');
	let prefixStack = $state<string[]>([]);
	let memoryCache = $state<Map<string, number>>(new Map());
	let memoryLoading = $state(false);

	const isMac = /mac/i.test(
		(navigator as Navigator & { userAgentData?: { platform: string } }).userAgentData?.platform ??
			navigator.userAgent
	);

	const keyTypes = [
		{ value: 'all', label: 'All types' },
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
		{ value: 'ttl', label: 'TTL' },
		{ value: 'memory', label: 'Memory' }
	] as const;

	interface TreeEntry {
		name: string;
		fullPrefix: string;
		isFolder: boolean;
		count: number;
		key?: KeyMeta;
	}

	function buildTreeLevel(keys: KeyMeta[], prefix: string, delimiter = ':'): TreeEntry[] {
		const folders = new Map<string, number>();
		const leaves: TreeEntry[] = [];

		for (const k of keys) {
			if (!k.key.startsWith(prefix)) continue;
			const rest = k.key.slice(prefix.length);
			const delimIdx = rest.indexOf(delimiter);
			if (delimIdx === -1) {
				// Leaf node — key is directly at this level
				leaves.push({
					name: rest,
					fullPrefix: k.key,
					isFolder: false,
					count: 0,
					key: k
				});
			} else {
				// Folder — group by next segment
				const segment = rest.slice(0, delimIdx + 1);
				const folderPrefix = prefix + segment;
				folders.set(folderPrefix, (folders.get(folderPrefix) ?? 0) + 1);
			}
		}

		const folderEntries: TreeEntry[] = [...folders.entries()]
			.sort(([a], [b]) => a.localeCompare(b))
			.map(([fp, count]) => ({
				name: fp.slice(prefix.length),
				fullPrefix: fp,
				isFolder: true,
				count
			}));

		return [...folderEntries, ...leaves];
	}

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
		if (value === 'key' || value === 'type' || value === 'ttl' || value === 'memory') {
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
				case 'memory': {
					const aBytes = memoryCache.get(a.key);
					const bBytes = memoryCache.get(b.key);
					// Keys without data sort last
					if (aBytes === undefined && bBytes === undefined) return a.key.localeCompare(b.key);
					if (aBytes === undefined) return 1;
					if (bBytes === undefined) return -1;
					return aBytes - bBytes || a.key.localeCompare(b.key);
				}
				default:
					return 0;
			}
		});
		return sortAsc ? sorted : sorted.reverse();
	}

	let sortedKeys = $derived.by(() => {
		memoryCache; // track for reactivity when memory data arrives
		return sortKeys(keys);
	});
	let treeEntries = $derived(buildTreeLevel(sortedKeys, currentPrefix));
	let selectableKeyNames = $derived(
		viewMode === 'tree'
			? treeEntries.filter((e) => !e.isFolder).map((e) => e.fullPrefix)
			: sortedKeys.map((k) => k.key)
	);

	// Determine empty state type
	let hasActiveFilters = $derived(pattern !== '*' || typeFilter !== 'all');
	let isEmpty = $derived(sortedKeys.length === 0 && !loading);
	let isEmptyDatabase = $derived(isEmpty && dbSize === 0 && !hasActiveFilters);
	let isEmptySearch = $derived(isEmpty && hasActiveFilters);

	function selectHistory(entry: HistoryEntry) {
		pattern = entry.pattern;
		useRegex = entry.regex;
		showHistory = false;
	}

	function clearFilters() {
		pattern = '*';
		typeFilter = 'all';
		inputRef?.focus();
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

	// Lazy-load memory usage when sort mode is "memory"
	$effect(() => {
		if (sortBy !== 'memory') return;
		const currentKeys = keys; // track: re-fire on load more
		if (currentKeys.length === 0) return;

		// Read cache without tracking to avoid infinite loop (we write to it below)
		const cache = untrack(() => memoryCache);
		const uncached = currentKeys.map((k) => k.key).filter((k) => !cache.has(k));
		if (uncached.length === 0) return;

		memoryLoading = true;
		api
			.getKeysMemory(uncached)
			.then((result) => {
				const next = new Map(memoryCache);
				for (const [k, v] of Object.entries(result.memory)) {
					next.set(k, v);
				}
				memoryCache = next;
				memoryLoading = false;
			})
			.catch(() => {
				memoryLoading = false;
			});
	});

	// Debounced search when pattern, type filter, or regex mode changes
	$effect(() => {
		pattern; // track dependency
		typeFilter; // track dependency
		useRegex; // track dependency
		if (debounceTimer) clearTimeout(debounceTimer);
		debounceTimer = setTimeout(() => {
			selectedKeys = new Set();
			currentPrefix = '';
			prefixStack = [];
			loadKeys(true);
			searchHistoryRef?.addToHistory(pattern, useRegex);
		}, 500);
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
			const actualTypeFilter =
				typeFilter === 'all' ? undefined : typeFilter === 'geo' ? 'zset' : typeFilter;
			const result = await api.getKeys(pattern, c, 100, actualTypeFilter, true, useRegex);
			const newKeys = result.keys as KeyMeta[];
			if (reset) {
				keys = newKeys;
				memoryCache = new Map();
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

	function handleKeyCreated(keyName: string) {
		loadKeys(true);
		onselect(keyName);
		oncreated();
	}

	function handleKeyClick(event: MouseEvent, key: string) {
		if (event.metaKey || event.ctrlKey) {
			// Toggle individual key
			const next = new Set(selectedKeys);
			if (next.has(key)) {
				next.delete(key);
			} else {
				next.add(key);
			}
			selectedKeys = next;
			lastClickedKey = key;
		} else if (event.shiftKey && lastClickedKey) {
			// Range select
			const keyNames = selectableKeyNames;
			const fromIdx = keyNames.indexOf(lastClickedKey);
			const toIdx = keyNames.indexOf(key);
			if (fromIdx !== -1 && toIdx !== -1) {
				const start = Math.min(fromIdx, toIdx);
				const end = Math.max(fromIdx, toIdx);
				const next = new Set(selectedKeys);
				for (let i = start; i <= end; i++) {
					next.add(keyNames[i]);
				}
				selectedKeys = next;
			}
		} else {
			// Normal click — clear multi-select, open editor
			selectedKeys = new Set();
			lastClickedKey = key;
			onselect(key);
		}
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape' && selectedKeys.size > 0) {
			selectedKeys = new Set();
			event.preventDefault();
		}
		if ((event.metaKey || event.ctrlKey) && event.key === 'a' && selectableKeyNames.length > 0) {
			event.preventDefault();
			selectedKeys = new Set(selectableKeyNames);
		}
	}

	function handleBulkDeleted(count: number) {
		selectedKeys = new Set();
		loadKeys(true);
	}

	function keysUnderPrefix(prefix: string): string[] {
		return sortedKeys.filter((k) => k.key.startsWith(prefix)).map((k) => k.key);
	}

	function entryKeys(entry: TreeEntry): string[] {
		return entry.isFolder ? keysUnderPrefix(entry.fullPrefix) : [entry.fullPrefix];
	}

	function handleTreeClick(event: MouseEvent, entry: TreeEntry) {
		if (event.metaKey || event.ctrlKey) {
			const keys = entryKeys(entry);
			const allSelected = keys.every((k) => selectedKeys.has(k));
			const next = new Set(selectedKeys);
			for (const k of keys) {
				if (allSelected) next.delete(k);
				else next.add(k);
			}
			selectedKeys = next;
			lastClickedTree = entry.fullPrefix;
		} else if (event.shiftKey && lastClickedTree) {
			const fromIdx = treeEntries.findIndex((e) => e.fullPrefix === lastClickedTree);
			const toIdx = treeEntries.findIndex((e) => e.fullPrefix === entry.fullPrefix);
			if (fromIdx !== -1 && toIdx !== -1) {
				const start = Math.min(fromIdx, toIdx);
				const end = Math.max(fromIdx, toIdx);
				const next = new Set(selectedKeys);
				for (let i = start; i <= end; i++) {
					for (const k of entryKeys(treeEntries[i])) next.add(k);
				}
				selectedKeys = next;
			}
		} else if (entry.isFolder) {
			navigateTo(entry.fullPrefix);
		} else {
			selectedKeys = new Set();
			lastClickedTree = entry.fullPrefix;
			lastClickedKey = entry.fullPrefix;
			onselect(entry.fullPrefix);
		}
	}

	function navigateTo(prefix: string) {
		prefixStack = [...prefixStack, currentPrefix];
		currentPrefix = prefix;
	}

	function navigateBack() {
		const prev = prefixStack[prefixStack.length - 1] ?? '';
		prefixStack = prefixStack.slice(0, -1);
		currentPrefix = prev;
	}

	function navigateToRoot() {
		prefixStack = [];
		currentPrefix = '';
	}
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="flex h-full flex-col p-4" onkeydown={handleKeydown}>
	<div class="mb-3 flex gap-2">
		<div class="relative flex-1">
			<Input
				bind:ref={inputRef}
				type="text"
				bind:value={pattern}
				placeholder={useRegex ? 'Regex (e.g., ^user:\\d+)' : 'Pattern (e.g., user:*)'}
				onfocus={() => (showHistory = true)}
				onblur={() => setTimeout(() => (showHistory = false), 150)}
				onkeydown={(e) => {
					if (e.key === 'Escape' && showHistory) {
						showHistory = false;
						e.preventDefault();
						e.stopPropagation();
					}
				}}
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
				class={useRegex ? 'bg-accent  dark:bg-accent/75' : ''}
				title={useRegex ? 'Regex mode (click for glob)' : 'Glob mode (click for regex)'}
				aria-label={useRegex ? 'Regex mode (click for glob)' : 'Glob mode (click for regex)'}
			>
				<Regex size={18} />
			</Button>
			<Button
				variant="outline"
				onclick={() => (showFilters = !showFilters)}
				class={showFilters ? 'bg-accent' : ''}
				title="Toggle filters"
				aria-label="Toggle filters"
			>
				<Funnel size={18} />
			</Button>
			<Button
				variant="outline"
				onclick={() => (viewMode = viewMode === 'list' ? 'tree' : 'list')}
				title={viewMode === 'list' ? 'Switch to tree view' : 'Switch to list view'}
				aria-label={viewMode === 'list' ? 'Switch to tree view' : 'Switch to list view'}
				class={viewMode === 'tree' ? 'bg-accent' : ''}
			>
				<ListTree size={18} />
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
		{#if selectedKeys.size > 0}
			<div class="flex items-center gap-1.5">
				<Button
					variant="ghost"
					size="icon"
					class="h-6 w-6 text-muted-foreground"
					onclick={() => (selectedKeys = new Set())}
					title="Clear selection"
					aria-label="Clear selection"
				>
					<CircleX size={14} />
				</Button>
				<span class="text-sm font-medium">{selectedKeys.size} selected</span>
			</div>
			<div class="flex items-center gap-1.5">
				<Button
					variant="ghost"
					size="sm"
					onclick={() => {
						if (selectedKeys.size === selectableKeyNames.length) {
							selectedKeys = new Set();
						} else {
							selectedKeys = new Set(selectableKeyNames);
						}
					}}
				>
					{selectedKeys.size === selectableKeyNames.length ? 'Deselect All' : 'Select All'}
				</Button>
				{#if !readOnly}
					<Button variant="destructive" size="sm" onclick={() => (showBulkDelete = true)}>
						<Trash2 size={14} />
					</Button>
				{/if}
			</div>
		{:else}
			<div class="flex items-center gap-2 text-sm text-muted-foreground">
				<span>
					{#if pattern !== '*' || typeFilter !== 'all'}
						{sortedKeys.length} of {dbSize} key{dbSize === 1 ? '' : 's'}
					{:else}
						{dbSize} total key{dbSize === 1 ? '' : 's'}
					{/if}
				</span>
				<span class="text-xs opacity-40">
					<Kbd.Root>{isMac ? '⌘' : 'Ctrl'}</Kbd.Root> Click to select
				</span>
			</div>
			<span>
				{#if !readOnly}
					<Button
						variant="outline"
						size="sm"
						class="hover:bg-accent"
						onclick={() => (showAddDialog = true)}
					>
						<CirclePlus /> New Key
					</Button>
				{/if}
			</span>
		{/if}
	</div>

	{#if typeFilter === 'geo'}
		<div class="mb-3 text-xs text-muted-foreground">
			Geo data is stored as sorted sets. Use "View as Geo" toggle in the editor to see coordinates.
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

	<div class="flex-1 overflow-y-auto border-t border-muted">
		{#if sortedKeys.length > 0}
			{#if viewMode === 'tree'}
				<div class="flex h-full flex-col gap-3 py-3">
					<div class="flex items-center gap-3">
						<ButtonGroup.Root>
							<Button
								variant="outline"
								size="sm"
								onclick={navigateToRoot}
								class="px-2"
								title="Go to root"
								aria-label="Go to root"
								disabled={currentPrefix === ''}
							>
								<House size={16} />
							</Button>
							<Button
								variant="outline"
								size="sm"
								onclick={navigateBack}
								class="px-2"
								title="Go back"
								aria-label="Go back"
								disabled={prefixStack.length === 0}
							>
								<MoveLeft size={16} />
							</Button>
						</ButtonGroup.Root>
						<span class="flex-1 truncate font-mono text-sm text-muted-foreground"
							>{currentPrefix || '*'}</span
						>
					</div>

					<ul class="flex-1 list-none overflow-y-auto">
						{#each treeEntries as entry, i (entry.fullPrefix)}
							{@const isSelected = entry.isFolder
								? keysUnderPrefix(entry.fullPrefix).every((k) => selectedKeys.has(k)) &&
									selectedKeys.size > 0
								: selectedKeys.has(entry.fullPrefix)}
							{@const prevEntry = i > 0 ? treeEntries[i - 1] : null}
							{@const nextEntry = i < treeEntries.length - 1 ? treeEntries[i + 1] : null}
							{@const prevSelected =
								isSelected &&
								prevEntry != null &&
								(prevEntry.isFolder
									? keysUnderPrefix(prevEntry.fullPrefix).every((k) => selectedKeys.has(k)) &&
										selectedKeys.size > 0
									: selectedKeys.has(prevEntry.fullPrefix))}
							{@const nextSelected =
								isSelected &&
								nextEntry != null &&
								(nextEntry.isFolder
									? keysUnderPrefix(nextEntry.fullPrefix).every((k) => selectedKeys.has(k)) &&
										selectedKeys.size > 0
									: selectedKeys.has(nextEntry.fullPrefix))}
							{@const rounding = isSelected
								? prevSelected && nextSelected
									? 'rounded-none'
									: prevSelected
										? 'rounded-t-none rounded-b'
										: nextSelected
											? 'rounded-t rounded-b-none'
											: 'rounded'
								: 'rounded'}
							<li>
								{#if entry.isFolder}
									<Button
										variant="ghost"
										class="w-full justify-start p-2 font-mono text-sm text-foreground hover:bg-primary/10 {isSelected
											? 'bg-primary/20 hover:bg-primary/20'
											: ''} {rounding}"
										onclick={(e: MouseEvent) => handleTreeClick(e, entry)}
										title={`Navigate to: ${entry.fullPrefix}`}
										aria-label={`Navigate to: ${entry.fullPrefix}`}
									>
										<span class="text-muted-foreground">
											<Folder size={16} />
										</span>
										<span class="flex-1 overflow-hidden text-left text-ellipsis">{entry.name}</span>
										<span class="ml-2 text-xs text-muted-foreground">({entry.count})</span>
									</Button>
								{:else}
									<Button
										variant="ghost"
										class="w-full justify-start p-2 font-mono text-sm text-foreground hover:bg-primary/10 {isSelected
											? 'bg-primary/20 hover:bg-primary/20'
											: entry.fullPrefix === selected && selectedKeys.size === 0
												? 'bg-primary/20 hover:bg-primary/20'
												: ''} {rounding}"
										onclick={(e: MouseEvent) => handleTreeClick(e, entry)}
										title={`View key: ${entry.fullPrefix}`}
										aria-label={`View key: ${entry.fullPrefix}`}
									>
										<span class="text-muted-foreground">
											<Dot size={16} />
										</span>
										<span class="flex-1 overflow-hidden text-left text-ellipsis">{entry.name}</span>
										{#if entry.key}
											<Badge variant="secondary" class="ml-2 text-xs opacity-60"
												>{entry.key.type}</Badge
											>
											{#if memoryCache.has(entry.fullPrefix)}
												<span class="ml-1 text-xs text-muted-foreground opacity-60"
													>{formatBytes(memoryCache.get(entry.fullPrefix)!)}</span
												>
											{/if}
										{/if}
									</Button>
								{/if}
							</li>
						{/each}
					</ul>

					{#if treeEntries.length === 0}
						<div class="py-8 text-center text-muted-foreground">No keys at this level</div>
					{/if}
				</div>
			{:else}
				<ul class="list-none py-1">
					{#each sortedKeys as item, i (item.key)}
						{@const hasTtlBoundary =
							sortBy === 'ttl' &&
							i < sortedKeys.length - 1 &&
							item.ttl >= 0 !== sortedKeys[i + 1].ttl >= 0}
						{@const isSelected = selectedKeys.has(item.key)}
						{@const prevSelected = isSelected && i > 0 && selectedKeys.has(sortedKeys[i - 1].key)}
						{@const nextSelected =
							isSelected && i < sortedKeys.length - 1 && selectedKeys.has(sortedKeys[i + 1].key)}
						<li class={hasTtlBoundary ? 'mb-1 border-b border-border pb-1' : ''}>
							<Button
								variant="ghost"
								class="w-full justify-start overflow-hidden p-2 font-mono text-sm text-ellipsis whitespace-nowrap text-foreground hover:bg-primary/10 {isSelected
									? 'bg-primary/20 hover:bg-primary/20'
									: item.key === selected && selectedKeys.size === 0
										? 'bg-primary/20 hover:bg-primary/20'
										: ''} {isSelected
									? prevSelected && nextSelected
										? 'rounded-none'
										: prevSelected
											? 'rounded-t-none rounded-b'
											: nextSelected
												? 'rounded-t rounded-b-none'
												: 'rounded'
									: 'rounded'}"
								onclick={(e: MouseEvent) => handleKeyClick(e, item.key)}
								title={`View key: ${item.key}`}
								aria-label={`View key: ${item.key}`}
							>
								<span class="flex-1 overflow-hidden text-left text-ellipsis">{item.key}</span>
								<Badge variant="secondary" class="ml-2 text-xs opacity-60">{item.type}</Badge>
								{#if memoryCache.has(item.key)}
									<span class="ml-1 text-xs text-muted-foreground opacity-60"
										>{formatBytes(memoryCache.get(item.key)!)}</span
									>
								{/if}
							</Button>
						</li>
					{/each}
				</ul>
			{/if}

			{#if hasMore}
				<div class="border-t border-muted px-1 py-1">
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
				</div>
			{/if}

			{#if memoryLoading}
				<div class="px-2 py-1 text-xs text-muted-foreground">Loading memory usage data...</div>
			{/if}
		{:else if loading}
			<Empty.Root>
				<Empty.Header>
					<Empty.Media variant="icon">
						<Search class="animate-pulse" />
					</Empty.Media>
					<Empty.Title>Loading Keys...</Empty.Title>
					<Empty.Description>Searching database for matching keys</Empty.Description>
				</Empty.Header>
			</Empty.Root>
		{:else if isEmptyDatabase}
			<Empty.Root>
				<Empty.Header>
					<Empty.Media variant="icon">
						<DatabaseZap />
					</Empty.Media>
					<Empty.Title>No Keys in Database</Empty.Title>
					<Empty.Description>
						{!readOnly ? 'Create your first key to get started.' : 'No keys available to view.'}
					</Empty.Description>
				</Empty.Header>
				{#if !readOnly}
					<Empty.Content>
						<Button onclick={() => (showAddDialog = true)} class="w-full">
							<CirclePlus />
							Create First Key
						</Button>
					</Empty.Content>
				{/if}
			</Empty.Root>
		{:else if isEmptySearch}
			<Empty.Root>
				<Empty.Header>
					<Empty.Media variant="icon">
						<Search />
					</Empty.Media>
					<Empty.Title>No Keys Found</Empty.Title>
					<Empty.Description>
						No keys match your current search pattern
						{typeFilter !== 'all' ? `and type filter "${typeFilterLabel}"` : ''}.
					</Empty.Description>
				</Empty.Header>
				<Empty.Content>
					<Button variant="outline" onclick={clearFilters} class="w-full">
						<CircleX />
						Clear Filters
					</Button>
				</Empty.Content>
			</Empty.Root>
		{/if}
	</div>

	<!-- Footer with about and settings -->
	<span class="mt-0 flex items-center justify-between border-t border-border pt-3 text-xs">
		<span>
			{#if usedMemoryHuman}
				<span class="text-muted-foreground">{usedMemoryHuman} memory usage</span>
			{/if}
		</span>
		<div class="flex items-center">
			<Button
				variant="ghost"
				size="sm"
				class="h-7 {showPalette ? 'text-primary' : 'text-muted-foreground'} hover:text-foreground"
				onclick={() => (showPalette = true)}
				title="Color palette"
				aria-label="Color palette"
			>
				<Palette size={14} />
			</Button>
			<Button
				variant="ghost"
				size="sm"
				class="h-7 {showSettings ? 'text-primary' : 'text-muted-foreground'} hover:text-foreground"
				onclick={() => (showSettings = true)}
				title="Settings and server info"
				aria-label="Settings and server info"
			>
				<Settings size={14} />
			</Button>
			<Button
				variant="ghost"
				size="sm"
				class="h-7 {consoleVisible
					? 'text-primary'
					: 'text-muted-foreground'} hover:text-foreground"
				onclick={onToggleConsole}
				title="Toggle console"
				aria-label="Toggle console"
			>
				<SquareTerminal size={14} />
			</Button>
			<Button
				variant="ghost"
				size="sm"
				onclick={() => (showAbout = true)}
				class="h-7 {showAbout ? 'text-primary' : 'text-muted-foreground'} hover:text-foreground"
				title="About kvweb"
				aria-label="About kvweb"
			>
				<Info size={14} />
			</Button>
		</div>
	</span>
</div>

<ServerSettingsDialog bind:open={showSettings} {readOnly} {disableFlush} />

<AboutDialog bind:open={showAbout} {version} {commit} {dirty} />

<PaletteDialog bind:open={showPalette} />

<AddKeyDialog
	bind:open={showAddDialog}
	{prefix}
	onCreated={handleKeyCreated}
	onCancel={() => (showAddDialog = false)}
/>

<BulkDeleteDialog
	bind:open={showBulkDelete}
	keys={selectedKeys}
	onDeleted={handleBulkDeleted}
	onCancel={() => (showBulkDelete = false)}
/>
