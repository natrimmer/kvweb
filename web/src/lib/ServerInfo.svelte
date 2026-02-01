<script lang="ts">
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import * as Select from '$lib/components/ui/select';
	import { Info, Radio, RotateCcw } from '@lucide/svelte';
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { api } from './api';
	import { toastError } from './utils';
	import { ws, type Stats, type Status } from './ws';
	interface Props {
		readOnly: boolean;
		disableFlush: boolean;
		clearSelectedKey: () => void;
	}

	let { readOnly, disableFlush, clearSelectedKey }: Props = $props();

	let info = $state('');
	let loading = $state(false);
	let section = $state('');
	let notificationsEnabled = $state(false);
	let enablingNotifications = $state(false);
	let flushDialogOpen = $state(false);

	const sections = [
		{ value: '', label: 'All Sections' },
		{ value: 'server', label: 'Server' },
		{ value: 'clients', label: 'Clients' },
		{ value: 'memory', label: 'Memory' },
		{ value: 'stats', label: 'Stats' },
		{ value: 'replication', label: 'Replication' },
		{ value: 'cpu', label: 'CPU' },
		{ value: 'keyspace', label: 'Keyspace' }
	];

	const selectedLabel = $derived(
		sections.find((s) => s.value === section)?.label ?? 'All Sections'
	);

	async function loadInfo() {
		loading = true;
		try {
			const result = await api.getInfo(section);
			info = result.info;
		} catch (e) {
			info = 'Failed to load server info';
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		loadInfo();

		// Load initial notifications status
		api
			.getNotifications()
			.then((result) => {
				notificationsEnabled = result.enabled;
			})
			.catch(() => {
				// Ignore if endpoint not available
			});

		// Subscribe to status updates for immediate notification changes
		const unsubStatus = ws.onStatus((status: Status) => {
			notificationsEnabled = status.live;
		});

		// Subscribe to stats updates for live notifications status
		const unsubStats = ws.onStats((stats: Stats) => {
			notificationsEnabled = stats.notificationsOn;
		});

		return () => {
			unsubStatus();
			unsubStats();
		};
	});

	function handleValueChange(value: string | undefined) {
		if (value !== undefined) {
			section = value;
			loadInfo();
		}
	}

	async function flushDb() {
		try {
			await api.flushDb();
			toast.success('Database flushed');
		} catch (e) {
			toastError(e, 'Failed to flush database');
		} finally {
			clearSelectedKey();
			flushDialogOpen = false;
		}
	}

	async function toggleNotifications() {
		enablingNotifications = true;
		try {
			const result = await api.setNotifications(!notificationsEnabled);
			notificationsEnabled = result.enabled;
			if (result.enabled) {
				toast.success('Live updates enabled');
			} else {
				toast.success('Live updates disabled');
			}
		} catch (e) {
			toastError(e, `Failed to ${notificationsEnabled ? 'disable' : 'enable'} notifications`);
		} finally {
			enablingNotifications = false;
		}
	}
</script>

<div class="flex h-full flex-col gap-4 p-6">
	<div class="flex items-center justify-between">
		<div class="flex gap-2">
			<Select.Root type="single" bind:value={section} onValueChange={handleValueChange}>
				<Select.Trigger class="w-50" size="sm">
					{selectedLabel}
				</Select.Trigger>
				<Select.Content>
					{#each sections as sect}
						<Select.Item value={sect.value}>{sect.label}</Select.Item>
					{/each}
				</Select.Content>
			</Select.Root>
			<Button
				variant="secondary"
				size="sm"
				onclick={loadInfo}
				disabled={loading}
				class="cursor-pointer"
				title="Refresh server info"
				aria-label="Refresh server info"
			>
				<RotateCcw class="size-4" />
			</Button>
			{#if !readOnly && !disableFlush}
				<AlertDialog.Root bind:open={flushDialogOpen}>
					<AlertDialog.Trigger>
						{#snippet child({ props })}
							<Button
								variant="destructive"
								size="sm"
								{...props}
								class="cursor-pointer"
								title="Delete all keys in database"
							>
								Flush Database
							</Button>
						{/snippet}
					</AlertDialog.Trigger>
					<AlertDialog.Content>
						<AlertDialog.Header>
							<AlertDialog.Title>Flush Database</AlertDialog.Title>
							<AlertDialog.Description>
								This will delete ALL keys in the current database. This action cannot be undone.
							</AlertDialog.Description>
						</AlertDialog.Header>
						<AlertDialog.Footer>
							<AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
							<AlertDialog.Action onclick={flushDb}>Flush Database</AlertDialog.Action>
						</AlertDialog.Footer>
					</AlertDialog.Content>
				</AlertDialog.Root>
			{/if}
		</div>
		{#if !readOnly}
			<Button
				variant="outline"
				size="sm"
				onclick={toggleNotifications}
				disabled={enablingNotifications}
				class="cursor-pointer hover:bg-accent"
				title={notificationsEnabled
					? 'Disable Valkey keyspace notifications (stops real-time updates)'
					: 'Enable Valkey keyspace notifications at runtime (enables real-time key change updates)'}
				aria-label={notificationsEnabled ? 'Disable live updates' : 'Enable live updates'}
			>
				<Radio />
				{enablingNotifications
					? notificationsEnabled
						? 'Disabling...'
						: 'Enabling...'
					: notificationsEnabled
						? 'Disable Live Updates'
						: 'Enable Live Updates'}
			</Button>
		{/if}
	</div>

	<pre
		class="flex-1 overflow-auto rounded bg-muted p-4 font-mono text-sm break-all whitespace-pre-wrap">{loading
			? 'Loading...'
			: info}
	</pre>

	<a
		href="/kvweb"
		class="flex items-center gap-2 text-sm text-muted-foreground hover:underline"
		title="Learn more about kvweb"
		aria-label="Learn more about kvweb"
	>
		<span>learn more about kvweb</span>
		<Info size={20} />
	</a>
</div>
