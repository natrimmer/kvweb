<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as Table from '$lib/components/ui/table';
	import { RotateCcw } from '@lucide/svelte';
	import { onMount } from 'svelte';
	import { api, type SlowLogEntry } from '$lib/api';
	import { toastError } from '$lib/utils';

	interface Props {
		open: boolean;
	}

	let { open = $bindable() }: Props = $props();

	let entries = $state<SlowLogEntry[]>([]);
	let totalLength = $state(0);
	let showLoading = $state(false);

	function formatDuration(us: number): string {
		if (us < 1000) return `${us}\u00b5s`;
		if (us < 1_000_000) return `${(us / 1000).toFixed(1)}ms`;
		return `${(us / 1_000_000).toFixed(2)}s`;
	}

	function formatCommand(args: string[]): string {
		if (!args || args.length === 0) return '';
		return args.join(' ');
	}

	function formatTime(timestamp: number): string {
		return new Date(timestamp * 1000).toLocaleString();
	}

	async function load() {
		const timer = setTimeout(() => (showLoading = true), 300);
		try {
			const result = await api.getSlowLog(128);
			entries = result.entries;
			totalLength = result.length;
		} catch (e) {
			toastError(e, 'Failed to load slow log');
		} finally {
			clearTimeout(timer);
			showLoading = false;
		}
	}

	onMount(() => {
		load();
	});
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="flex max-h-[80vh] min-w-3xl flex-col">
		<Dialog.Header>
			<Dialog.Title>Slow Log</Dialog.Title>
			<Dialog.Description>
				Commands that exceeded the <code class="text-xs">slowlog-log-slower-than</code> threshold
			</Dialog.Description>
		</Dialog.Header>
		<div class="flex min-h-0 flex-1 flex-col gap-3 overflow-hidden">
			<div class="flex shrink-0 items-center justify-between">
				<span class="text-sm text-muted-foreground">
					{entries.length} of {totalLength} entries
				</span>
				<Button
					variant="secondary"
					size="sm"
					onclick={load}
					disabled={showLoading}
					title="Refresh slow log"
					aria-label="Refresh slow log"
				>
					<RotateCcw class="size-4" />
				</Button>
			</div>

			<div class="min-h-0 flex-1 overflow-auto">
				{#if entries.length > 0}
					<Table.Root>
						<Table.Header>
							<Table.Row>
								<Table.Head class="w-44">Time</Table.Head>
								<Table.Head class="w-24 text-right">Duration</Table.Head>
								<Table.Head>Command</Table.Head>
								<Table.Head class="w-40">Client</Table.Head>
							</Table.Row>
						</Table.Header>
						<Table.Body>
							{#each entries as entry (entry.id)}
								<Table.Row>
									<Table.Cell class="text-xs text-muted-foreground">
										{formatTime(entry.timestamp)}
									</Table.Cell>
									<Table.Cell class="text-right font-mono text-xs">
										{formatDuration(entry.duration)}
									</Table.Cell>
									<Table.Cell>
										<span
											class="block max-w-md truncate font-mono text-xs"
											title={formatCommand(entry.args)}
										>
											{formatCommand(entry.args)}
										</span>
									</Table.Cell>
									<Table.Cell class="text-xs text-muted-foreground">
										{entry.clientAddr}
										{#if entry.clientName}
											<span class="ml-1 opacity-60">({entry.clientName})</span>
										{/if}
									</Table.Cell>
								</Table.Row>
							{/each}
						</Table.Body>
					</Table.Root>
				{:else if !showLoading}
					<div class="py-12 text-center text-muted-foreground">
						<p class="mb-2 font-medium">No slow log entries</p>
						<p class="text-sm">
							Commands slower than the <code class="text-xs">slowlog-log-slower-than</code> threshold
							(default 10ms) will appear here.
						</p>
					</div>
				{:else}
					<div class="py-12 text-center text-muted-foreground">Loading...</div>
				{/if}
			</div>
		</div>
	</Dialog.Content>
</Dialog.Root>
