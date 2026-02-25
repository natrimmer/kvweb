<script lang="ts">
	import { X } from '@lucide/svelte';
	import { onMount } from 'svelte';

	export interface HistoryEntry {
		pattern: string;
		regex: boolean;
	}

	interface Props {
		show: boolean;
		onselect: (entry: HistoryEntry) => void;
	}

	let { show, onselect }: Props = $props();

	const HISTORY_KEY = 'kvweb:search-history';
	const MAX_HISTORY = 20;

	let history = $state<HistoryEntry[]>([]);

	function loadHistory() {
		try {
			const stored = localStorage.getItem(HISTORY_KEY);
			if (!stored) {
				history = [];
				return;
			}
			const parsed = JSON.parse(stored);
			if (!Array.isArray(parsed)) {
				history = [];
				return;
			}
			// Migrate old string[] format to HistoryEntry[]
			if (parsed.length > 0 && typeof parsed[0] === 'string') {
				history = parsed.map((p: string) => ({ pattern: p, regex: false }));
				saveHistory();
			} else {
				// Validate each entry has the expected shape
				history = parsed.filter(
					(e: unknown): e is HistoryEntry =>
						typeof e === 'object' &&
						e !== null &&
						typeof (e as HistoryEntry).pattern === 'string' &&
						typeof (e as HistoryEntry).regex === 'boolean'
				);
			}
		} catch {
			history = [];
		}
	}

	function saveHistory() {
		localStorage.setItem(HISTORY_KEY, JSON.stringify(history));
	}

	export function addToHistory(pattern: string, isRegex: boolean) {
		if (!pattern || pattern === '*') return;
		const entry: HistoryEntry = { pattern, regex: isRegex };
		history = [
			entry,
			...history.filter((h) => !(h.pattern === pattern && h.regex === isRegex))
		].slice(0, MAX_HISTORY);
		saveHistory();
	}

	function removeFromHistory(entry: HistoryEntry) {
		history = history.filter((h) => !(h.pattern === entry.pattern && h.regex === entry.regex));
		saveHistory();
	}

	function clearHistory() {
		history = [];
		saveHistory();
	}

	function selectHistory(entry: HistoryEntry) {
		onselect(entry);
	}

	onMount(() => {
		loadHistory();
	});
</script>

{#if show && history.length > 0}
	<div
		class="absolute top-full right-0 left-0 z-10 mt-1 max-h-60 overflow-auto rounded border border-border bg-popover shadow-lg"
	>
		<div class="flex items-center justify-between border-b border-border px-3 py-2">
			<span class="text-xs text-muted-foreground">Recent searches</span>
			<button
				type="button"
				class="text-xs text-muted-foreground hover:text-destructive"
				onmousedown={() => clearHistory()}
				title="Clear search history"
				aria-label="Clear search history"
			>
				Clear all
			</button>
		</div>
		{#each history as h}
			<div class="group flex items-center hover:bg-muted">
				<button
					type="button"
					class="flex flex-1 items-center gap-2 px-3 py-2 text-left font-mono text-sm"
					onmousedown={() => selectHistory(h)}
					title="Use this search pattern"
					aria-label="Use this search pattern"
				>
					<span class="flex-1 overflow-hidden text-ellipsis">{h.pattern}</span>
					{#if h.regex}
						<span class="text-xs text-primary opacity-70">.*</span>
					{/if}
				</button>
				<button
					type="button"
					class="px-2 py-1 text-muted-foreground opacity-0 group-hover:opacity-100 hover:text-destructive"
					onmousedown={() => removeFromHistory(h)}
					title="Remove from history"
					aria-label="Remove from history"
				>
					<X size={14} />
				</button>
			</div>
		{/each}
	</div>
{/if}
