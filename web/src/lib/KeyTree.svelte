<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Dot, Folder, House, List, MoveLeft } from '@lucide/svelte';
	import { api, type PrefixEntry } from './api';
	import { getErrorMessage } from './utils';

	interface Props {
		selected: string | null;
		onselect: (key: string) => void;
		onclose: () => void;
	}

	let { selected, onselect, onclose }: Props = $props();

	let entries = $state<PrefixEntry[]>([]);
	let loading = $state(false);
	let currentPrefix = $state('');
	let prefixStack = $state<string[]>([]);
	let error = $state('');

	async function loadPrefixes(prefix: string) {
		loading = true;
		error = '';
		try {
			const result = await api.getPrefixes(prefix);
			entries = result.entries;
			currentPrefix = prefix;
		} catch (e) {
			error = getErrorMessage(e, 'Failed to load');
			entries = [];
		} finally {
			loading = false;
		}
	}

	function navigateTo(prefix: string) {
		prefixStack = [...prefixStack, currentPrefix];
		loadPrefixes(prefix);
	}

	function navigateBack() {
		const prev = prefixStack.pop();
		prefixStack = prefixStack;
		loadPrefixes(prev ?? '');
	}

	function navigateToRoot() {
		prefixStack = [];
		loadPrefixes('');
	}

	function handleClick(entry: PrefixEntry) {
		if (entry.isLeaf && entry.fullKey) {
			onselect(entry.fullKey);
		} else {
			navigateTo(entry.prefix);
		}
	}

	function displayName(entry: PrefixEntry): string {
		// Show just the last segment
		const name = entry.prefix.slice(currentPrefix.length);
		return name || entry.prefix;
	}

	// Load root on mount
	loadPrefixes('');
</script>

<div class="flex h-full flex-col gap-3 p-4">
	<div class="flex items-center gap-2">
		{#if currentPrefix}
			<Button
				variant="ghost"
				size="sm"
				onclick={navigateBack}
				class="px-2"
				title="Go back"
				aria-label="Go back"
			>
				<MoveLeft size={16} />
			</Button>
			<Button
				variant="ghost"
				size="sm"
				onclick={navigateToRoot}
				class="px-2"
				title="Go to root"
				aria-label="Go to root"
			>
				<House size={16} />
			</Button>
			<span class="flex-1 truncate font-mono text-sm text-muted-foreground">{currentPrefix}</span>
		{:else}
			<span class="flex-1 text-sm text-muted-foreground">Tree View</span>
		{/if}
		<Button
			size="sm"
			onclick={onclose}
			title="Switch to list view"
			aria-label="Switch to list view"
		>
			<List size={18} />
		</Button>
	</div>

	{#if loading}
		<div class="flex flex-1 items-center justify-center text-muted-foreground">Loading...</div>
	{:else if error}
		<div class="flex flex-1 items-center justify-center text-destructive">{error}</div>
	{:else}
		<ul class="flex-1 list-none overflow-y-auto">
			{#each entries as entry (entry.prefix)}
				<li>
					<Button
						variant="ghost"
						class="w-full justify-start rounded p-2 font-mono text-sm text-foreground hover:bg-primary/10 {entry.isLeaf &&
						entry.fullKey === selected
							? 'bg-primary/20 hover:bg-primary/20'
							: ''}"
						onclick={() => handleClick(entry)}
						title={entry.isLeaf ? `View key: ${entry.fullKey}` : `Navigate to: ${entry.prefix}`}
						aria-label={entry.isLeaf
							? `View key: ${entry.fullKey}`
							: `Navigate to: ${entry.prefix}`}
					>
						<span class="text-muted-foreground">
							{#if entry.isLeaf}
								<Dot size={16} />
							{:else if !entry.isLeaf}
								<Folder size={16} />
							{/if}
						</span>
						<span class="flex-1 overflow-hidden text-left text-ellipsis">{displayName(entry)}</span>
						{#if entry.isLeaf && entry.type}
							<Badge variant="secondary" class="ml-2 text-xs opacity-60">{entry.type}</Badge>
						{:else if !entry.isLeaf}
							<span class="ml-2 text-xs text-muted-foreground">({entry.count})</span>
						{/if}
					</Button>
				</li>
			{/each}
		</ul>

		{#if entries.length === 0}
			<div class="py-8 text-center text-muted-foreground">No keys found</div>
		{/if}
	{/if}
</div>
