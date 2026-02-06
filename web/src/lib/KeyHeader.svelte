<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as ButtonGroup from '$lib/components/ui/button-group';
	import { Input } from '$lib/components/ui/input';
	import { Check, ChevronDown, ChevronUp, Copy, Pencil, RefreshCw, X } from '@lucide/svelte/icons';
	import type { KeyType } from './api';
	import { formatShortcut } from './keyboard';
	import { copyToClipboard, formatTtl } from './utils';

	interface Props {
		keyName: string;
		keyType: KeyType;
		liveTtl: number | null;
		readOnly: boolean;
		updatingTtl: boolean;
		renamingKey: boolean;
		loading: boolean;
		typeHeaderExpanded: boolean;
		geoViewActive?: boolean;
		onToggleTypeHeader: () => void;
		onDelete: () => void;
		onTtlChange: (ttl: number) => void;
		onCopyValue: () => void;
		onRename: (newKey: string) => void;
		onRefresh: () => void;
	}

	let {
		keyName,
		keyType,
		liveTtl,
		readOnly,
		updatingTtl,
		renamingKey,
		loading,
		typeHeaderExpanded,
		geoViewActive = false,
		onToggleTypeHeader,
		onDelete,
		onTtlChange,
		onCopyValue,
		onRename,
		onRefresh
	}: Props = $props();

	let editTtl = $state('');
	let editKeyName = $state('');
	let copiedKey = $state(false);
	let copiedValue = $state(false);
	let lastKeyName = $state('');
	let editingTtl = $state(false);
	let editingKeyName = $state(false);

	// Sync editTtl only when key changes (not on every liveTtl countdown tick)
	$effect(() => {
		if (keyName !== lastKeyName) {
			lastKeyName = keyName;
			if (liveTtl !== null && liveTtl > 0) {
				editTtl = String(liveTtl);
			} else {
				editTtl = '';
			}
		}
	});

	async function copyKeyName() {
		await copyToClipboard(keyName, (v) => (copiedKey = v));
	}

	async function handleCopyValue() {
		copiedValue = true;
		onCopyValue();
		setTimeout(() => (copiedValue = false), 2000);
	}

	function handleTtlUpdate() {
		const ttl = editTtl ? parseInt(editTtl, 10) : 0;
		onTtlChange(ttl);
		editingTtl = false;
	}

	function startEditingTtl() {
		if (liveTtl !== null && liveTtl > 0) {
			editTtl = String(liveTtl);
		} else {
			editTtl = '';
		}
		editingTtl = true;
	}

	function cancelEditingTtl() {
		editingTtl = false;
	}

	function handleRenameClick() {
		editingKeyName = !editingKeyName;
		if (editingKeyName) {
			editKeyName = keyName;
		}
	}

	function cancelEditingKeyName() {
		editingKeyName = false;
	}

	function handleKeyRename() {
		const newKey = editKeyName.trim();
		if (newKey && newKey !== keyName) {
			onRename(newKey);
		}
		editingKeyName = false;
	}
</script>

<div class="flex flex-col pb-6">
	<!-- Row 1: Key name, type badge, TTL, copy buttons, delete -->
	<div class="flex items-center gap-3">
		<div class="flex min-w-0 flex-1 items-center gap-2">
			{#if editingKeyName}
				<span class="flex items-center gap-1">
					<Input
						type="text"
						bind:value={editKeyName}
						placeholder="Key name"
						class="h-8 w-48 text-sm"
						onkeydown={(e) => {
							if (e.key === 'Enter') handleKeyRename();
							if (e.key === 'Escape') cancelEditingKeyName();
						}}
						title="Key name"
						aria-label="Key name"
					/>
					<Button
						variant="default"
						size="sm"
						onclick={handleKeyRename}
						disabled={renamingKey}
						title="Save key name"
						aria-label="Save key name"
						class="h-8 cursor-pointer"
					>
						{#if renamingKey}
							...
						{:else}
							<Check class="size-4" />
						{/if}
					</Button>
					<Button
						variant="destructive"
						size="sm"
						onclick={cancelEditingKeyName}
						disabled={renamingKey}
						title="Cancel rename"
						aria-label="Cancel rename"
						class="h-8 cursor-pointer"
					>
						<X class="size-4" />
					</Button>
				</span>
			{:else}
				<h2 class="min-w-0 font-mono text-lg leading-none break-all">{keyName}</h2>
			{/if}
			<Badge variant="secondary" class="shrink-0 uppercase"
				>{keyType}{#if geoViewActive}(geo){/if}</Badge
			>
			{#if !editingTtl}
				<span class="shrink-0 text-sm text-muted-foreground">
					TTL: {formatTtl(liveTtl ?? -1)}
					{#if !readOnly}
						<button
							type="button"
							onclick={startEditingTtl}
							title="Edit TTL"
							aria-label="Edit TTL"
							class="ml-1 cursor-pointer text-muted-foreground hover:text-foreground"
						>
							<Pencil class="inline h-3 w-3" />
						</button>
					{/if}
				</span>
			{:else}
				<span class="flex shrink-0 items-center gap-1 text-sm">
					<span class="text-muted-foreground">TTL:</span>
					<Input
						type="number"
						bind:value={editTtl}
						placeholder="seconds"
						class="h-8 w-32 text-sm"
						onkeydown={(e) => {
							if (e.key === 'Enter') handleTtlUpdate();
							if (e.key === 'Escape') cancelEditingTtl();
						}}
						title="TTL in seconds"
						aria-label="TTL in seconds"
					/>
					<Button
						variant="default"
						size="sm"
						onclick={handleTtlUpdate}
						disabled={updatingTtl}
						title="Save TTL"
						aria-label="Save TTL"
						class="h-8 cursor-pointer"
					>
						{#if updatingTtl}
							...
						{:else}
							<Check class="size-4" />
						{/if}
					</Button>
					<Button
						variant="destructive"
						size="sm"
						onclick={cancelEditingTtl}
						title="Cancel TTL edit"
						aria-label="Cancel TTL edit"
						class="h-8 cursor-pointer"
					>
						<X class="size-4" />
					</Button>
				</span>
			{/if}
		</div>
		<Button
			variant="outline"
			size="sm"
			onclick={onRefresh}
			disabled={loading}
			title="Refresh key data"
			aria-label="Refresh key data"
			class="cursor-pointer"
		>
			<RefreshCw class={loading ? 'h-4 w-4 animate-spin' : 'h-4 w-4'} />
		</Button>
		<ButtonGroup.Root class="shrink-0">
			<Button
				variant="outline"
				size="sm"
				onclick={copyKeyName}
				title="Copy key name to clipboard"
				aria-label="Copy key name to clipboard"
				class="cursor-pointer"
			>
				{#if copiedKey}
					<Check class="h-4 w-4 text-primary" />
				{:else}
					<Copy class="h-4 w-4" />
				{/if}
				Key
			</Button>
			<Button
				variant="outline"
				size="sm"
				onclick={handleCopyValue}
				title="Copy value to clipboard"
				aria-label="Copy value to clipboard"
				class="cursor-pointer"
			>
				{#if copiedValue}
					<Check class="h-4 w-4 text-primary" />
				{:else}
					<Copy class="h-4 w-4" />
				{/if}
				Value
			</Button>
		</ButtonGroup.Root>
		{#if !readOnly}
			<ButtonGroup.Root class="shrink-0">
				<Button
					variant="outline"
					size="sm"
					onclick={handleRenameClick}
					title="Rename this key"
					aria-label="Rename this key"
					class="shrink-0 cursor-pointer"
				>
					Rename
				</Button>
				<Button
					variant="destructive"
					size="sm"
					onclick={onDelete}
					class="shrink-0 cursor-pointer"
					title={`Delete this key (${formatShortcut('Delete')})`}
					aria-label="Delete this key"
				>
					Delete
				</Button>
			</ButtonGroup.Root>
		{/if}
		<Button
			variant="outline"
			size="sm"
			onclick={onToggleTypeHeader}
			aria-label={typeHeaderExpanded ? 'Collapse type controls' : 'Expand type controls'}
			title={typeHeaderExpanded ? 'Collapse type controls' : 'Expand type controls'}
			class="shrink-0 cursor-pointer text-muted-foreground hover:text-foreground"
		>
			{#if typeHeaderExpanded}
				<ChevronUp class="h-5 w-5" />
			{:else}
				<ChevronDown class="h-5 w-5" />
			{/if}
		</Button>
	</div>
</div>
