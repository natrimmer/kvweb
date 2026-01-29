<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as ButtonGroup from '$lib/components/ui/button-group';
	import { Input } from '$lib/components/ui/input';
	import CheckIcon from '@lucide/svelte/icons/check';
	import ChevronDownIcon from '@lucide/svelte/icons/chevron-down';
	import ChevronUpIcon from '@lucide/svelte/icons/chevron-up';
	import CopyIcon from '@lucide/svelte/icons/copy';
	import PencilIcon from '@lucide/svelte/icons/pencil';
	import XIcon from '@lucide/svelte/icons/x';
	import type { KeyType } from './api';
	import { copyToClipboard, formatTtl } from './utils';

	interface Props {
		keyName: string;
		keyType: KeyType;
		liveTtl: number | null;
		readOnly: boolean;
		externallyModified: boolean;
		keyDeleted: boolean;
		updatingTtl: boolean;
		renamingKey: boolean;
		typeHeaderExpanded: boolean;
		onToggleTypeHeader: () => void;
		onDelete: () => void;
		onReload: () => void;
		onClose: () => void;
		onTtlChange: (ttl: number) => void;
		onCopyValue: () => void;
		onRename: (newKey: string) => void;
	}

	let {
		keyName,
		keyType,
		liveTtl,
		readOnly,
		externallyModified,
		keyDeleted,
		updatingTtl,
		renamingKey,
		typeHeaderExpanded,
		onToggleTypeHeader,
		onDelete,
		onReload,
		onClose,
		onTtlChange,
		onCopyValue,
		onRename
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

<div class="flex flex-col gap-2">
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
					/>
					<Button
						variant="default"
						size="sm"
						onclick={handleKeyRename}
						class="h-8 cursor-pointer"
						title="Save key name"
					>
						{#if renamingKey}
							...
						{:else}
							<CheckIcon class="size-4" />
						{/if}
					</Button>
					<Button
						variant="destructive"
						size="sm"
						onclick={cancelEditingKeyName}
						class="h-8 cursor-pointer"
						title="Cancel"
						disabled={renamingKey}
					>
						<XIcon class="size-4" />
					</Button>
				</span>
			{:else}
				<h2 class="min-w-0 font-mono text-lg leading-none break-all">{keyName}</h2>
			{/if}
			<Badge variant="secondary" class="shrink-0 uppercase">{keyType}</Badge>
			{#if !editingTtl}
				<span class="shrink-0 text-sm text-muted-foreground">
					TTL: {formatTtl(liveTtl ?? -1)}
					{#if !readOnly}
						<button
							type="button"
							onclick={startEditingTtl}
							class="ml-1 cursor-pointer text-muted-foreground hover:text-foreground"
							title="Edit TTL"
						>
							<PencilIcon class="inline h-3 w-3" />
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
					/>
					<Button
						variant="default"
						size="sm"
						onclick={handleTtlUpdate}
						disabled={updatingTtl}
						class="h-8 cursor-pointer"
						title="Save TTL"
					>
						{#if updatingTtl}
							...
						{:else}
							<CheckIcon class="size-4" />
						{/if}
					</Button>
					<Button
						variant="destructive"
						size="sm"
						onclick={cancelEditingTtl}
						class="h-8 cursor-pointer"
						title="Cancel"
					>
						<XIcon class="size-4" />
					</Button>
				</span>
			{/if}
		</div>
		<ButtonGroup.Root class="shrink-0">
			<Button
				variant="outline"
				size="sm"
				onclick={copyKeyName}
				title="Copy key name to clipboard"
				class="cursor-pointer"
			>
				{#if copiedKey}
					<CheckIcon class="h-4 w-4 text-primary" />
				{:else}
					<CopyIcon class="h-4 w-4" />
				{/if}
				Key
			</Button>
			<Button
				variant="outline"
				size="sm"
				onclick={handleCopyValue}
				title="Copy value to clipboard"
				class="cursor-pointer"
			>
				{#if copiedValue}
					<CheckIcon class="h-4 w-4 text-primary" />
				{:else}
					<CopyIcon class="h-4 w-4" />
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
					class="shrink-0 cursor-pointer"
				>
					Rename
				</Button>
				<Button
					variant="destructive"
					size="sm"
					onclick={onDelete}
					class="shrink-0 cursor-pointer"
					title="Delete this key"
				>
					Delete
				</Button>
			</ButtonGroup.Root>
		{/if}
		<Button
			variant="outline"
			size="sm"
			onclick={onToggleTypeHeader}
			class="shrink-0 cursor-pointer text-muted-foreground hover:text-foreground"
			title={typeHeaderExpanded ? 'Collapse type controls' : 'Expand type controls'}
		>
			{#if typeHeaderExpanded}
				<ChevronUpIcon class="h-5 w-5" />
			{:else}
				<ChevronDownIcon class="h-5 w-5" />
			{/if}
		</Button>
	</div>

	<!-- External modification alerts -->
	{#if keyDeleted}
		<div
			class="flex items-center justify-between rounded bg-destructive/10 px-3 py-2 text-sm text-destructive"
		>
			<span>This key was deleted externally</span>
			<Button variant="secondary" size="sm" onclick={onClose} class="cursor-pointer">Close</Button>
		</div>
	{:else if externallyModified}
		<div
			class="flex items-center justify-between rounded bg-accent/10 px-3 py-2 text-sm text-accent-foreground"
		>
			<span>Modified externally</span>
			<Button variant="secondary" size="sm" onclick={onReload} class="cursor-pointer">
				Reload
			</Button>
		</div>
	{/if}
</div>
