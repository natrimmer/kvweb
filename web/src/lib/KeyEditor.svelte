<script lang="ts">
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import { Textarea } from '$lib/components/ui/textarea';
	import { toast } from 'svelte-sonner';
	import { api, type HashPair, type KeyInfo, type StreamEntry, type ZSetMember } from './api';
	import { HashEditor, ListEditor, SetEditor, StreamEditor, ZSetEditor } from './editors';
	import KeyHeader from './KeyHeader.svelte';
	import TypeHeader from './TypeHeader.svelte';
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

	// Type header expand/collapse state
	let typeHeaderExpanded = $state(true);

	function openDeleteDialog() {
		deleteDialogOpen = true;
	}

	function toggleTypeHeader() {
		typeHeaderExpanded = !typeHeaderExpanded;
	}

	// Optional callback from child editors to provide custom copy value
	let zsetGetCopyValue: (() => string) | undefined = $state(undefined);
	let zsetGeoViewActive = $state(false);

	async function copyValue() {
		if (!keyInfo) return;
		let text: string;
		if (zsetGetCopyValue) {
			text = zsetGetCopyValue();
		} else {
			text =
				typeof keyInfo.value === 'string' ? keyInfo.value : JSON.stringify(keyInfo.value, null, 2);
		}
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
			// Reset editor-specific state
			zsetGetCopyValue = undefined;
			zsetGeoViewActive = false;
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

	let renamingKey = $state(false);

	function renameKey(newKey: string) {
		renamingKey = true;
		api
			.renameKey(key, newKey)
			.then(() => {
				toast.success('Key renamed');
				key = newKey;
				loadKey(key);
			})
			.catch((e) => {
				toastError(e, 'Failed to rename key');
			})
			.finally(() => {
				renamingKey = false;
			});
	}
</script>

<div class="flex h-full flex-col gap-4 overflow-hidden p-6">
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
			{renamingKey}
			{typeHeaderExpanded}
			geoViewActive={keyInfo.type === 'zset' && zsetGeoViewActive}
			onToggleTypeHeader={toggleTypeHeader}
			onDelete={openDeleteDialog}
			onReload={() => {
				loadKey(key);
				externallyModified = false;
			}}
			onClose={ondeleted}
			onTtlChange={updateTtl}
			onCopyValue={copyValue}
			onRename={renameKey}
		/>

		{#if keyInfo.type === 'string'}
			<div class="flex min-h-0 flex-1 flex-col gap-2">
				<TypeHeader expanded={typeHeaderExpanded}>
					<div class="flex items-center justify-between">
						<label for="value-textarea">Value:</label>
						<div class="flex items-center gap-2">
							{#if isJsonValue}
								<Button
									size="sm"
									variant="outline"
									onclick={() => (prettyPrint = !prettyPrint)}
									class="cursor-pointer"
									title={prettyPrint ? 'Compact JSON formatting' : 'Pretty-print JSON formatting'}
									aria-label={prettyPrint
										? 'Compact JSON formatting'
										: 'Pretty-print JSON formatting'}
								>
									{prettyPrint ? 'Compact JSON' : 'Format JSON'}
								</Button>
							{/if}
							{#if !readOnly && hasChanges}
								<Button
									size="sm"
									onclick={saveValue}
									disabled={saving}
									class="cursor-pointer"
									title="Save changes"
									aria-label="Save changes"
								>
									{saving ? 'Saving...' : 'Save'}
								</Button>
							{/if}
						</div>
					</div>
				</TypeHeader>

				<div class="-mx-6 min-h-0 flex-1 overflow-auto border-t border-border px-6 pt-2">
					{#if isJsonValue && highlightedHtml}
						<div
							class="rounded border border-border [&>pre]:m-0 [&>pre]:min-h-full [&>pre]:p-4 [&>pre]:text-sm"
						>
							{@html highlightedHtml}
						</div>
					{:else}
						<Textarea
							id="value-textarea"
							bind:value={editValue}
							readonly={readOnly}
							title="Key value"
							aria-label="Key value"
							class="min-h-75 flex-1 resize-none text-sm"
						/>
					{/if}
				</div>
			</div>
		{:else if keyInfo.type === 'list'}
			<ListEditor
				keyName={key}
				items={asArray()}
				pagination={keyInfo.pagination}
				{currentPage}
				{pageSize}
				{readOnly}
				{typeHeaderExpanded}
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
				{typeHeaderExpanded}
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
				{typeHeaderExpanded}
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
				{typeHeaderExpanded}
				onPageChange={goToPage}
				onPageSizeChange={changePageSize}
				onDataChange={handleDataChange}
				bind:getCopyValue={zsetGetCopyValue}
				bind:geoViewActive={zsetGeoViewActive}
			/>
		{:else if keyInfo.type === 'stream'}
			<StreamEditor
				keyName={key}
				entries={asStream()}
				pagination={keyInfo.pagination}
				{currentPage}
				{pageSize}
				{readOnly}
				{typeHeaderExpanded}
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
					<AlertDialog.Cancel title="Cancel" aria-label="Cancel">Cancel</AlertDialog.Cancel>
					<AlertDialog.Action onclick={deleteKey} title="Delete" aria-label="Delete">
						Delete
					</AlertDialog.Action>
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
