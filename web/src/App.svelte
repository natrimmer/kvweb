<script lang="ts">
	import Logo from '$lib/components/Logo.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import * as Empty from '$lib/components/ui/empty';
	import * as Resizable from '$lib/components/ui/resizable';
	import { Toaster } from '$lib/components/ui/sonner';
	import { CloudOff, Database, Radio } from '@lucide/svelte/icons';
	import { onMount } from 'svelte';
	import { api } from './lib/api';
	import { matchesShortcut } from './lib/keyboard';
	import KeyEditor from './lib/KeyEditor.svelte';
	import KeyList from './lib/KeyList.svelte';
	import { ws } from './lib/ws';

	let selectedKey = $state<string | null>(null);
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

		// Keyboard shortcuts
		function handleKeydown(e: KeyboardEvent) {
			// Escape: Clear selected key (return to home)
			if (matchesShortcut(e, 'Escape')) {
				if (selectedKey) {
					// Only if not focused on an input
					const activeElement = document.activeElement;
					if (activeElement?.tagName !== 'INPUT' && activeElement?.tagName !== 'TEXTAREA') {
						e.preventDefault();
						resetToHome();
					}
				}
				return;
			}
		}

		window.addEventListener('keydown', handleKeydown);

		return () => {
			ws.disconnect();
			if (healthCheckInterval) {
				clearInterval(healthCheckInterval);
			}
			window.removeEventListener('keydown', handleKeydown);
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
			<Logo size={24} class="text-primary transition-colors group-hover:text-primary" />
			kvweb
		</button>

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
						{disableFlush}
					/>
				</div>
			</Resizable.Pane>
			<Resizable.Handle withHandle />
			<Resizable.Pane defaultSize={75}>
				<div class="h-full overflow-auto">
					{#if selectedKey}
						<KeyEditor key={selectedKey} ondeleted={handleKeyDeleted} {readOnly} />
					{:else if dbSize === 0}
						<Empty.Root class="h-full">
							<Empty.Header>
								<Empty.Media variant="default">
									<Logo size={240} class="text-alabaster-grey-100" />
								</Empty.Media>
							</Empty.Header>
						</Empty.Root>
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
