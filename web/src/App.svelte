<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import * as Empty from '$lib/components/ui/empty';
	import * as Resizable from '$lib/components/ui/resizable';
	import { Toaster } from '$lib/components/ui/sonner';
	import { CloudOff, Database, Radio } from '@lucide/svelte/icons';
	import { onMount } from 'svelte';
	import { api } from './lib/api';
	import KeyEditor from './lib/KeyEditor.svelte';
	import KeyList from './lib/KeyList.svelte';
	import ServerInfo from './lib/ServerInfo.svelte';
	import { ws } from './lib/ws';

	let selectedKey = $state<string | null>(null);
	let view = $state<'keys' | 'info'>('keys');
	let dbSize = $state(0);
	let apiConnected = $state<boolean | null>(null); // null = not checked yet
	let dbConnected = $state<boolean | null>(null); // null = not checked yet
	let wsConnected = $state(false);
	let readOnly = $state(false);
	let prefix = $state('');
	let disableFlush = $state(false);
	let liveUpdates = $state(false);
	let healthCheckInterval: ReturnType<typeof setInterval> | null = null;

	function resetToHome() {
		selectedKey = null;
		view = 'keys';
	}

	async function checkHealth() {
		try {
			const health = await api.getHealth();
			apiConnected = true;
			dbConnected = health.database;
		} catch {
			apiConnected = false;
			dbConnected = false;
		}
		wsConnected = ws.isConnected();
	}

	onMount(() => {
		// Load initial data
		Promise.all([api.getInfo(), api.getConfig()])
			.then(([info, config]) => {
				dbSize = info.dbSize;
				readOnly = config.readOnly;
				prefix = config.prefix;
				disableFlush = config.disableFlush;
				apiConnected = true;
			})
			.catch(() => {
				apiConnected = false;
			});

		// Initial health check
		checkHealth();

		// Periodic health check every 5 seconds
		healthCheckInterval = setInterval(checkHealth, 5000);

		// Connect WebSocket
		ws.connect();

		ws.onStatus((status) => {
			liveUpdates = status.live;
			wsConnected = ws.isConnected();
		});

		ws.onStats((stats) => {
			dbSize = stats.dbSize;
			liveUpdates = stats.notificationsOn;
			wsConnected = ws.isConnected();
		});

		return () => {
			ws.disconnect();
			if (healthCheckInterval) {
				clearInterval(healthCheckInterval);
			}
		};
	});

	function handleKeySelect(key: string) {
		selectedKey = key;
	}

	function handleKeyDeleted() {
		selectedKey = null;
		dbSize = Math.max(0, dbSize - 1);
	}

	function handleKeyCreated() {
		dbSize += 1;
	}
</script>

<div class="flex h-screen flex-col">
	<!-- Error notification bar for critical connection issues -->
	{#if apiConnected === false || dbConnected === false}
		{@const message = apiConnected === false ? 'Server is unreachable' : 'Database connection lost'}
		<div
			class="relative flex items-center gap-3 overflow-hidden bg-destructive px-4 py-3 text-white"
		>
			<CloudOff class="z-10 h-6 w-6 shrink-0" />
			<div class="marquee-container flex-1 overflow-hidden">
				<div class="marquee-content flex text-sm font-medium whitespace-nowrap uppercase">
					{#each Array(20) as _}
						<span class="mr-4">{message} •</span>
					{/each}
				</div>
			</div>
		</div>
	{/if}

	<header class="flex items-center gap-6 border-b border-border px-6 py-4">
		<button
			type="button"
			onclick={resetToHome}
			class="group flex cursor-pointer items-center gap-2 text-xl font-semibold text-foreground transition-colors hover:text-primary"
			title="Return to home"
		>
			<svg
				width="24"
				height="24"
				viewBox="0 0 64 64"
				xmlns="http://www.w3.org/2000/svg"
				class="text-primary transition-colors group-hover:text-primary"
			>
				<rect
					x="8"
					y="8"
					width="48"
					height="48"
					rx="6"
					fill="none"
					stroke="currentColor"
					stroke-width="3"
				/>
				<line x1="32" y1="8" x2="32" y2="56" stroke="currentColor" stroke-width="3" />
				<line x1="8" y1="24" x2="56" y2="24" stroke="currentColor" stroke-width="3" />
				<line x1="8" y1="40" x2="56" y2="40" stroke="currentColor" stroke-width="3" />
				<rect x="32" y="8" width="24" height="16" fill="currentColor" />
			</svg>
			kvweb
		</button>

		<nav class="flex gap-1">
			<button
				type="button"
				onclick={() => (view = 'keys')}
				class="cursor-pointer rounded px-4 py-1.5 text-sm transition-colors {view === 'keys'
					? 'bg-primary text-primary-foreground'
					: 'text-muted-foreground hover:bg-muted hover:text-foreground'}"
				title="View and manage keys"
			>
				Keys
			</button>
			<button
				type="button"
				onclick={() => (view = 'info')}
				class="cursor-pointer rounded px-4 py-1.5 text-sm transition-colors {view === 'info'
					? 'bg-primary text-primary-foreground'
					: 'text-muted-foreground hover:bg-muted hover:text-foreground'}"
				title="View server information"
			>
				Info
			</button>
		</nav>

		<div class="ml-auto flex items-center gap-3 text-sm">
			{#if readOnly}
				<Badge variant="default" class="bg-accent text-accent-foreground hover:bg-accent"
					>READ-ONLY</Badge
				>
			{/if}
			{#if liveUpdates}
				{#if wsConnected}
					<div
						class="flex items-center gap-1.5 text-xs text-primary"
						title="Receiving real-time updates from server"
					>
						<Radio class="h-3.5 w-3.5 animate-pulse" />
						<span>Live</span>
					</div>
				{:else}
					<div
						class="flex items-center gap-1.5 text-xs text-yellow-600"
						title="Live updates enabled but WebSocket disconnected (reconnecting...)"
					>
						<Radio class="h-3.5 w-3.5 opacity-50" />
						<span>Live (reconnecting...)</span>
					</div>
				{/if}
			{/if}
		</div>
	</header>

	<main class="flex-1 overflow-hidden">
		{#if view === 'keys'}
			<Resizable.PaneGroup direction="horizontal" class="h-full">
				<Resizable.Pane defaultSize={25} minSize={15} maxSize={50} collapsible={true}>
					<div class="flex h-full flex-col overflow-hidden border-r border-border">
						<KeyList
							onselect={handleKeySelect}
							selected={selectedKey}
							oncreated={handleKeyCreated}
							{readOnly}
							{prefix}
							{dbSize}
						/>
					</div>
				</Resizable.Pane>
				<Resizable.Handle withHandle />
				<Resizable.Pane defaultSize={75}>
					<div class="h-full overflow-auto">
						{#if selectedKey}
							<KeyEditor key={selectedKey} ondeleted={handleKeyDeleted} {readOnly} />
						{:else}
							<Empty.Root class="h-full">
								<Empty.Header>
									<Empty.Media variant="icon">
										<Database />
									</Empty.Media>
									<Empty.Title>No Key Selected</Empty.Title>
									<Empty.Description>
										Select a key from the list to view or edit its value.
									</Empty.Description>
								</Empty.Header>
							</Empty.Root>
						{/if}
					</div>
				</Resizable.Pane>
			</Resizable.PaneGroup>
		{:else}
			<ServerInfo {readOnly} {disableFlush} clearSelectedKey={() => (selectedKey = null)} />
		{/if}
	</main>
</div>

<Toaster />

<style>
	@keyframes marquee {
		from {
			transform: translateX(0);
		}
		to {
			transform: translateX(-50%);
		}
	}

	.marquee-content {
		animation: marquee 30s linear infinite;
	}
</style>
