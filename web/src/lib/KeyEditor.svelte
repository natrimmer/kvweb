<script lang="ts">
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import { Textarea } from '$lib/components/ui/textarea';
	import { toast } from 'svelte-sonner';
	import { api, type HashPair, type KeyInfo, type StreamEntry, type ZSetMember } from './api';
	import { HashEditor, ListEditor, SetEditor, StreamEditor, ZSetEditor } from './editors';
	import KeyHeader from './KeyHeader.svelte';
	import {
		copyToClipboard,
		deleteOps,
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

	// Delete confirmation dialog
	let deleteDialogOpen = $state(false);

	function openDeleteDialog() {
		deleteDialogOpen = true;
	}

	async function copyValue() {
		if (!keyInfo) return;
		const text =
			typeof keyInfo.value === 'string' ? keyInfo.value : JSON.stringify(keyInfo.value, null, 2);
		await copyToClipboard(text, () => {});
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
			await api.setKey(key, editValue, liveTtl ?? 0);
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

	async function updateTtl(ttl: number) {
		if (!keyInfo) return;
		updatingTtl = true;
		try {
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
		<KeyHeader
			keyName={key}
			keyType={keyInfo.type}
			{liveTtl}
			{readOnly}
			{externallyModified}
			{keyDeleted}
			{updatingTtl}
			onDelete={openDeleteDialog}
			onReload={() => {
				loadKey(key);
				externallyModified = false;
			}}
			onClose={ondeleted}
			onTtlChange={updateTtl}
			onCopyValue={copyValue}
		/>

		{#if keyInfo.type === 'string'}
			{#if !readOnly && hasChanges}
				<div class="flex justify-end">
					<Button
						size="sm"
						onclick={saveValue}
						disabled={saving}
						class="cursor-pointer"
						title="Save changes"
					>
						{saving ? 'Saving...' : 'Save'}
					</Button>
				</div>
			{/if}
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
