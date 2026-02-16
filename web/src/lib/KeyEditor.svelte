<script lang="ts">
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { toast } from 'svelte-sonner';
	import {
		api,
		type HashPair,
		type HLLData,
		type KeyInfo,
		type StreamEntry,
		type ZSetMember
	} from './api';
	import {
		HashEditor,
		HLLEditor,
		ListEditor,
		SetEditor,
		StreamEditor,
		StringEditor,
		ZSetEditor
	} from './editors';
	import KeyHeader from './KeyHeader.svelte';
	import { copyToClipboard, deleteOps, getErrorMessage, modifyOps, toastError } from './utils';
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
	let updatingTtl = $state(false);
	let error = $state('');
	let liveTtl = $state<number | null>(null);
	let ttlInterval: ReturnType<typeof setInterval> | null = null;
	let expiresAt: number | null = null;

	// Pagination state
	let currentPage = $state(1);
	let pageSize = $state(100);

	// Cursor-based pagination for sets and hashes
	let cursorStack = $state<number[]>([0]); // history of cursors visited
	let cursorIndex = $state(0); // current position in stack
	let nextCursor = $state<number | undefined>(undefined);

	function isCursorBased(type?: string): boolean {
		return type === 'set' || type === 'hash';
	}

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
	function asHLL(): HLLData {
		return (keyInfo?.value as HLLData) ?? { count: 0 };
	}

	let previousKey = $state<string | null>(null);

	$effect(() => {
		// Reset to page 1 only when key changes (not on pagination)
		if (previousKey !== key) {
			currentPage = 1;
			previousKey = key;
			// Reset cursor state
			cursorStack = [0];
			cursorIndex = 0;
			nextCursor = undefined;
			// Reset editor-specific state
			zsetGetCopyValue = undefined;
			zsetGeoViewActive = false;
		}
		loadKey(key);
		return () => stopTtlCountdown();
	});

	// Subscribe to WebSocket key events for live auto-reload
	$effect(() => {
		if (!key) return;

		const unsubscribe = ws.onKeyEvent((event) => {
			if (event.key !== key) return;

			if (deleteOps.has(event.op)) {
				// Key was deleted externally, close the editor
				ondeleted();
			} else if (modifyOps.has(event.op)) {
				// Key was modified externally, auto-reload
				loadKey(key);
			}
		});

		return unsubscribe;
	});

	// Keyboard shortcuts
	$effect(() => {
		function handleKeydown(e: KeyboardEvent) {
			// Delete: Delete key with confirmation
			if (e.key === 'Delete' && !readOnly && keyInfo) {
				// Only if not focused on an input
				const activeElement = document.activeElement;
				if (activeElement?.tagName !== 'INPUT' && activeElement?.tagName !== 'TEXTAREA') {
					e.preventDefault();
					openDeleteDialog();
				}
				return;
			}
		}

		window.addEventListener('keydown', handleKeydown);
		return () => window.removeEventListener('keydown', handleKeydown);
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
			// Pass cursor for set/hash cursor-based pagination (harmless no-op for other types)
			const cursor = cursorStack[cursorIndex] || undefined;
			keyInfo = await api.getKey(k, currentPage, pageSize, cursor);
			startTtlCountdown(keyInfo.ttl);
			// Store nextCursor from response
			if (keyInfo.pagination?.nextCursor !== undefined) {
				nextCursor = keyInfo.pagination.nextCursor;
			} else {
				nextCursor = undefined;
			}
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

	// Cursor-based navigation for sets and hashes
	function cursorNext() {
		if (nextCursor === undefined || nextCursor === 0) return;
		// Push nextCursor onto stack and advance index
		cursorStack = [...cursorStack.slice(0, cursorIndex + 1), nextCursor];
		cursorIndex = cursorStack.length - 1;
		currentPage = cursorIndex + 1;
		loadKey(key);
	}

	function cursorPrev() {
		if (cursorIndex <= 0) return;
		cursorIndex--;
		currentPage = cursorIndex + 1;
		loadKey(key);
	}

	function cursorFirst() {
		cursorStack = [0];
		cursorIndex = 0;
		currentPage = 1;
		nextCursor = undefined;
		loadKey(key);
	}

	function handleCursorPageChange(page: number) {
		if (page > currentPage) {
			cursorNext();
		} else if (page < currentPage && page === 1) {
			cursorFirst();
		} else if (page < currentPage) {
			cursorPrev();
		}
	}

	function changePageSize(newSize: number) {
		pageSize = newSize;
		currentPage = 1;
		// Reset cursor state on page size change
		cursorStack = [0];
		cursorIndex = 0;
		nextCursor = undefined;
		loadKey(key);
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

<div class="flex h-full flex-col overflow-hidden p-6">
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
			{updatingTtl}
			{renamingKey}
			{loading}
			{typeHeaderExpanded}
			geoViewActive={keyInfo.type === 'zset' && zsetGeoViewActive}
			onToggleTypeHeader={toggleTypeHeader}
			onDelete={openDeleteDialog}
			onTtlChange={updateTtl}
			onCopyValue={copyValue}
			onRename={renameKey}
			onRefresh={() => loadKey(key)}
		/>

		{#if keyInfo.type === 'string'}
			<StringEditor
				keyName={key}
				value={keyInfo.value as string}
				{readOnly}
				{typeHeaderExpanded}
				onDataChange={handleDataChange}
			/>
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
				cursorBased={true}
				hasMore={nextCursor !== undefined && nextCursor !== 0}
				onPageChange={handleCursorPageChange}
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
				cursorBased={true}
				hasMore={nextCursor !== undefined && nextCursor !== 0}
				onPageChange={handleCursorPageChange}
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
		{:else if keyInfo.type === 'hyperloglog'}
			<HLLEditor
				keyName={key}
				data={asHLL()}
				{readOnly}
				{typeHeaderExpanded}
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
		color: var(--json-string);
	}
	:global(.json-number) {
		color: var(--json-number);
	}
	:global(.json-keyword) {
		color: var(--json-keyword);
	}
</style>
