<script lang="ts">
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as ButtonGroup from '$lib/components/ui/button-group';
	import { Input } from '$lib/components/ui/input';
	import { Textarea } from '$lib/components/ui/textarea';
	import CheckIcon from '@lucide/svelte/icons/check';
	import CopyIcon from '@lucide/svelte/icons/copy';
	import { toast } from 'svelte-sonner';
	import { api, type HashPair, type KeyInfo, type StreamEntry, type ZSetMember } from './api';
	import { HashEditor, ListEditor, SetEditor, StreamEditor, ZSetEditor } from './editors';
	import {
		copyToClipboard,
		deleteOps,
		formatTtl,
		getErrorMessage,
		highlightJson,
		modifyOps,
		toastError
	} from './utils';
	import { ws } from './ws';

	interface Props {
		key: string;
		ondeleted: () => void;
		readOnly: boolean;
	}

	let { key, ondeleted, readOnly }: Props = $props();

	let keyInfo = $state<KeyInfo | null>(null);
	let loading = $state(false);
	let showLoading = $state(false);
	let loadingTimeout: ReturnType<typeof setTimeout> | null = null;
	let saving = $state(false);
	let updatingTtl = $state(false);
	let editValue = $state('');
	let editTtl = $state('');
	let error = $state('');
	let liveTtl = $state<number | null>(null);
	let ttlInterval: ReturnType<typeof setInterval> | null = null;
	let expiresAt: number | null = null;

	// JSON highlighting state for string type
	let prettyPrint = $state(false);
	let highlightedHtml = $state('');

	// Pagination state
	let currentPage = $state(1);
	let pageSize = $state(100);

	// External modification detection
	let externallyModified = $state(false);
	let keyDeleted = $state(false);

	// Copy to clipboard state
	let copiedValue = $state(false);
	let copiedKey = $state(false);

	// Delete confirmation dialog
	let deleteDialogOpen = $state(false);

	function openDeleteDialog() {
		deleteDialogOpen = true;
	}

	async function copyValue() {
		if (!keyInfo) return;
		const text =
			typeof keyInfo.value === 'string' ? keyInfo.value : JSON.stringify(keyInfo.value, null, 2);
		await copyToClipboard(text, (v) => (copiedValue = v));
	}

	async function copyKeyName() {
		await copyToClipboard(key, (v) => (copiedKey = v));
	}

	function startTtlCountdown(ttl: number) {
		stopTtlCountdown();
		if (ttl > 0) {
			expiresAt = Date.now() + ttl * 1000;
			updateLiveTtl();
			ttlInterval = setInterval(updateLiveTtl, 1000);
		} else {
			liveTtl = ttl;
		}
	}

	function updateLiveTtl() {
		if (expiresAt !== null) {
			const remaining = Math.round((expiresAt - Date.now()) / 1000);
			liveTtl = Math.max(0, remaining);
			if (remaining <= 0) {
				stopTtlCountdown();
			}
		}
	}

	function stopTtlCountdown() {
		if (ttlInterval) {
			clearInterval(ttlInterval);
			ttlInterval = null;
		}
		expiresAt = null;
	}

	// Type-safe accessors for complex types
	function asArray(): string[] {
		return Array.isArray(keyInfo?.value) ? (keyInfo.value as string[]) : [];
	}
	function asHash(): HashPair[] {
		return Array.isArray(keyInfo?.value) ? (keyInfo.value as HashPair[]) : [];
	}
	function asZSet(): ZSetMember[] {
		return Array.isArray(keyInfo?.value) ? (keyInfo.value as ZSetMember[]) : [];
	}
	function asStream(): StreamEntry[] {
		return Array.isArray(keyInfo?.value) ? (keyInfo.value as StreamEntry[]) : [];
	}

	function isJson(str: string): boolean {
		if (!str || str.length < 2) return false;
		const trimmed = str.trim();
		if (
			!(
				(trimmed.startsWith('{') && trimmed.endsWith('}')) ||
				(trimmed.startsWith('[') && trimmed.endsWith(']'))
			)
		) {
			return false;
		}
		try {
			JSON.parse(str);
			return true;
		} catch {
			return false;
		}
	}

	// Highlight string value when it changes or prettyPrint toggles
	$effect(() => {
		if (keyInfo?.type === 'string' && isJson(editValue)) {
			highlightedHtml = highlightJson(editValue, prettyPrint);
		} else {
			highlightedHtml = '';
		}
	});

	let isJsonValue = $derived(keyInfo?.type === 'string' && isJson(editValue));

	// Track original value for dirty checking
	let originalValue = $state('');
	let hasChanges = $derived(editValue !== originalValue);

	let previousKey = $state<string | null>(null);

	$effect(() => {
		// Reset to page 1 only when key changes (not on pagination)
		if (previousKey !== key) {
			currentPage = 1;
			previousKey = key;
		}
		loadKey(key);
		// Reset external modification state when key changes
		externallyModified = false;
		keyDeleted = false;
		return () => stopTtlCountdown();
	});

	// Subscribe to WebSocket key events for external modification detection
	$effect(() => {
		if (!key) return;

		const unsubscribe = ws.onKeyEvent((event) => {
			if (event.key !== key) return;

			if (deleteOps.has(event.op)) {
				keyDeleted = true;
				externallyModified = false;
			} else if (modifyOps.has(event.op)) {
				// Only mark as externally modified if we're not currently saving
				if (!saving) {
					externallyModified = true;
				}
			}
		});

		return unsubscribe;
	});

	async function loadKey(k: string) {
		loading = true;
		error = '';
		stopTtlCountdown();
		// Only show loading indicator after 500ms delay to avoid flash
		loadingTimeout = setTimeout(() => {
			if (loading) showLoading = true;
		}, 300);
		try {
			keyInfo = await api.getKey(k, currentPage, pageSize);
			const value =
				typeof keyInfo.value === 'string' ? keyInfo.value : JSON.stringify(keyInfo.value, null, 2);
			editValue = value;
			originalValue = value;
			editTtl = keyInfo.ttl > 0 ? String(keyInfo.ttl) : '';
			startTtlCountdown(keyInfo.ttl);
		} catch (e) {
			error = getErrorMessage(e, 'Failed to load key');
			keyInfo = null;
		} finally {
			loading = false;
			showLoading = false;
			if (loadingTimeout) {
				clearTimeout(loadingTimeout);
				loadingTimeout = null;
			}
		}
	}

	function goToPage(page: number) {
		currentPage = page;
		loadKey(key);
	}

	function changePageSize(newSize: number) {
		pageSize = newSize;
		currentPage = 1;
		loadKey(key);
	}

	async function saveValue() {
		if (!keyInfo) return;
		saving = true;
		error = '';
		try {
			const ttl = editTtl ? parseInt(editTtl, 10) : 0;
			await api.setKey(key, editValue, ttl);
			await loadKey(key);
			toast.success('Value saved');
		} catch (e) {
			toastError(e, 'Failed to save');
		} finally {
			saving = false;
		}
	}

	async function deleteKey() {
		try {
			await api.deleteKey(key);
			toast.success('Key deleted');
			ondeleted();
		} catch (e) {
			toastError(e, 'Failed to delete');
		} finally {
			deleteDialogOpen = false;
		}
	}

	async function updateTtl() {
		if (!keyInfo) return;
		updatingTtl = true;
		try {
			const ttl = editTtl ? parseInt(editTtl, 10) : 0;
			await api.expireKey(key, ttl);
			await loadKey(key);
			toast.success('TTL updated');
		} catch (e) {
			toastError(e, 'Failed to update TTL');
		} finally {
			updatingTtl = false;
		}
	}

	function handleDataChange() {
		loadKey(key);
	}
</script>

<div class="flex h-full flex-col gap-4 p-6">
	{#if showLoading}
		<div class="flex h-full items-center justify-center text-muted-foreground">Loading...</div>
	{:else if error}
		<div class="flex h-full items-center justify-center text-destructive">{error}</div>
	{:else if keyInfo}
		<div class="flex items-center gap-4">
			<h2 class="flex-1 font-mono text-xl break-all">{key}</h2>
			<Badge variant="secondary" class="uppercase">{keyInfo.type}</Badge>
			<ButtonGroup.Root>
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
					onclick={copyValue}
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
		</div>

		{#if keyDeleted}
			<div
				class="flex items-center justify-between rounded bg-destructive/10 p-3 text-sm text-destructive"
			>
				<span>This key was deleted externally</span>
				<Button variant="secondary" size="sm" onclick={ondeleted} class="cursor-pointer">
					Close
				</Button>
			</div>
		{:else if externallyModified}
			<div
				class="flex items-center justify-between rounded bg-accent/10 p-3 text-sm text-accent-foreground"
			>
				<span>Modified externally</span>
				<Button
					variant="secondary"
					size="sm"
					onclick={() => {
						loadKey(key);
						externallyModified = false;
					}}
					class="cursor-pointer"
					title="Reload key data"
				>
					Reload
				</Button>
			</div>
		{/if}

		<div class="flex items-center justify-between gap-4 rounded bg-muted p-3">
			<label class="flex items-center gap-2">
				<span class="text-sm">TTL:</span>
				{#if readOnly}
					<span class="text-sm text-muted-foreground">{formatTtl(liveTtl ?? keyInfo.ttl)}</span>
				{:else}
					<Input type="number" bind:value={editTtl} placeholder="seconds" class="w-25" />
					<Button
						variant="secondary"
						size="sm"
						onclick={updateTtl}
						disabled={updatingTtl}
						class="cursor-pointer"
						title="Update TTL"
					>
						{updatingTtl ? 'Setting...' : 'Set'}
					</Button>
					<span class="text-sm text-muted-foreground">{formatTtl(liveTtl ?? keyInfo.ttl)}</span>
				{/if}
			</label>
			{#if !readOnly}
				<div class="flex gap-2">
					{#if keyInfo.type === 'string' && hasChanges}
						<Button
							size="sm"
							onclick={saveValue}
							disabled={saving}
							class="cursor-pointer"
							title="Save changes"
						>
							{saving ? 'Saving...' : 'Save'}
						</Button>
					{/if}
					<Button
						variant="destructive"
						size="sm"
						onclick={openDeleteDialog}
						class="cursor-pointer"
						title="Delete this key"
					>
						Delete
					</Button>
				</div>
			{/if}
		</div>

		{#if keyInfo.type === 'string'}
			<div class="flex flex-1 flex-col gap-2">
				<div class="flex items-center justify-between">
					<label for="value-textarea">Value:</label>
					{#if isJsonValue}
						<button
							type="button"
							onclick={() => (prettyPrint = !prettyPrint)}
							class="cursor-pointer rounded bg-muted px-2 py-1 text-xs text-foreground hover:bg-secondary"
							title={prettyPrint ? 'Compact JSON formatting' : 'Pretty-print JSON formatting'}
						>
							{prettyPrint ? 'Compact JSON' : 'Format JSON'}
						</button>
					{/if}
				</div>

				{#if isJsonValue && highlightedHtml}
					<div
						class="min-h-75 flex-1 overflow-auto rounded border border-border [&>pre]:m-0 [&>pre]:min-h-full [&>pre]:p-4 [&>pre]:text-sm"
					>
						{@html highlightedHtml}
					</div>
				{:else}
					<Textarea
						id="value-textarea"
						bind:value={editValue}
						readonly={readOnly}
						class="min-h-75 flex-1 resize-none text-sm"
					/>
				{/if}
			</div>
		{:else if keyInfo.type === 'list'}
			<ListEditor
				keyName={key}
				items={asArray()}
				pagination={keyInfo.pagination}
				{currentPage}
				{pageSize}
				{readOnly}
				onPageChange={goToPage}
				onPageSizeChange={changePageSize}
				onDataChange={handleDataChange}
			/>
		{:else if keyInfo.type === 'set'}
			<SetEditor
				keyName={key}
				members={asArray()}
				pagination={keyInfo.pagination}
				{currentPage}
				{pageSize}
				{readOnly}
				onPageChange={goToPage}
				onPageSizeChange={changePageSize}
				onDataChange={handleDataChange}
			/>
		{:else if keyInfo.type === 'hash'}
			<HashEditor
				keyName={key}
				fields={asHash()}
				pagination={keyInfo.pagination}
				{currentPage}
				{pageSize}
				{readOnly}
				onPageChange={goToPage}
				onPageSizeChange={changePageSize}
				onDataChange={handleDataChange}
			/>
		{:else if keyInfo.type === 'zset'}
			<ZSetEditor
				keyName={key}
				members={asZSet()}
				pagination={keyInfo.pagination}
				{currentPage}
				{pageSize}
				{readOnly}
				onPageChange={goToPage}
				onPageSizeChange={changePageSize}
				onDataChange={handleDataChange}
			/>
		{:else if keyInfo.type === 'stream'}
			<StreamEditor
				keyName={key}
				entries={asStream()}
				pagination={keyInfo.pagination}
				{currentPage}
				{pageSize}
				{readOnly}
				onPageChange={goToPage}
				onPageSizeChange={changePageSize}
				onDataChange={handleDataChange}
			/>
		{:else}
			<div class="flex flex-col gap-4">
				<p>Unknown type: {keyInfo.type}</p>
				<pre class="overflow-auto rounded bg-muted p-4 font-mono text-sm">{JSON.stringify(
						keyInfo.value,
						null,
						2
					)}</pre>
			</div>
		{/if}

		<AlertDialog.Root bind:open={deleteDialogOpen}>
			<AlertDialog.Content>
				<AlertDialog.Header>
					<AlertDialog.Title>Delete Key</AlertDialog.Title>
					<AlertDialog.Description>
						Are you sure you want to delete <code class="rounded bg-muted px-1 font-mono"
							>{key}</code
						>? This action cannot be undone.
					</AlertDialog.Description>
				</AlertDialog.Header>
				<AlertDialog.Footer>
					<AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
					<AlertDialog.Action onclick={deleteKey}>Delete</AlertDialog.Action>
				</AlertDialog.Footer>
			</AlertDialog.Content>
		</AlertDialog.Root>
	{/if}
</div>

<style>
	:global(.json-highlight) {
		margin: 0;
		font-family: ui-monospace, monospace;
		font-size: 0.875rem;
		line-height: 1.5;
		white-space: pre-wrap;
		word-break: break-all;
	}
	:global(.json-string) {
		color: #0550ae;
	}
	:global(.json-number) {
		color: #116329;
	}
	:global(.json-keyword) {
		color: #cf222e;
	}
</style>
