<script lang="ts">
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as ButtonGroup from '$lib/components/ui/button-group';
	import { Input } from '$lib/components/ui/input';
	import * as Table from '$lib/components/ui/table';
	import { Textarea } from '$lib/components/ui/textarea';
	import CheckIcon from '@lucide/svelte/icons/check';
	import ChevronLeftIcon from '@lucide/svelte/icons/chevron-left';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import ChevronsLeftIcon from '@lucide/svelte/icons/chevrons-left';
	import ChevronsRightIcon from '@lucide/svelte/icons/chevrons-right';
	import CopyIcon from '@lucide/svelte/icons/copy';
	import PencilIcon from '@lucide/svelte/icons/pencil';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';
	import XIcon from '@lucide/svelte/icons/x';
	import { toast } from 'svelte-sonner';
	import { api, type HashPair, type KeyInfo, type StreamEntry, type ZSetMember } from './api';
	import CollapsibleValue from './CollapsibleValue.svelte';
	import {
		copyToClipboard,
		deleteOps,
		formatTtl,
		getErrorMessage,
		highlightJson,
		isNonEmpty,
		isValidScore,
		modifyOps,
		parseScore,
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
	let saving = $state(false);
	let updatingTtl = $state(false);
	let editValue = $state('');
	let editTtl = $state('');
	let error = $state('');
	let liveTtl = $state<number | null>(null);
	let ttlInterval: ReturnType<typeof setInterval> | null = null;
	let expiresAt: number | null = null;

	// JSON highlighting state
	let prettyPrint = $state(false);
	let highlightedHtml = $state('');
	let listHighlights = $state<Record<number, string>>({});

	// Raw view toggle for complex types
	let rawView = $state(false);

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

	// Item delete confirmation
	let itemDeleteDialogOpen = $state(false);
	let itemToDelete = $state<{ type: string; identifier: string | number; display: string } | null>(
		null
	);

	// Inline editing state
	let editingItem = $state<{ type: string; identifier: string | number } | null>(null);
	let editingValue = $state('');
	let savingItem = $state(false);

	// Add item form state
	let showAddForm = $state(false);
	let addFormData = $state<{
		value?: string;
		position?: 'head' | 'tail';
		member?: string;
		field?: string;
		score?: string | number;
		streamFields?: { key: string; value: string }[];
	}>({});
	let addingItem = $state(false);

	function openDeleteDialog() {
		deleteDialogOpen = true;
	}

	function openItemDeleteDialog(type: string, identifier: string | number, display: string) {
		itemToDelete = { type, identifier, display };
		itemDeleteDialogOpen = true;
	}

	function startEditing(type: string, identifier: string | number, currentValue: string) {
		editingItem = { type, identifier };
		editingValue = currentValue;
	}

	function cancelEditing() {
		editingItem = null;
		editingValue = '';
	}

	function resetAddForm() {
		addFormData = {};
		if (keyInfo?.type === 'list') {
			addFormData.position = 'tail';
		}
		if (keyInfo?.type === 'stream') {
			addFormData.streamFields = [{ key: '', value: '' }];
		}
	}

	function openAddForm() {
		resetAddForm();
		showAddForm = true;
	}

	function closeAddForm() {
		showAddForm = false;
		resetAddForm();
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
		// Backend now returns array of {field, value} pairs for pagination
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

	// Highlight list items containing JSON
	$effect(() => {
		if (keyInfo?.type === 'list') {
			const items = asArray();
			const highlights: Record<number, string> = {};
			for (let i = 0; i < items.length; i++) {
				if (isJson(items[i])) {
					highlights[i] = highlightJson(items[i], prettyPrint);
				}
			}
			listHighlights = highlights;
		} else {
			listHighlights = {};
		}
	});

	let isJsonValue = $derived(keyInfo?.type === 'string' && isJson(editValue));

	// Check if current type is a complex type that supports raw view
	let isComplexType = $derived(
		keyInfo?.type === 'list' ||
			keyInfo?.type === 'set' ||
			keyInfo?.type === 'hash' ||
			keyInfo?.type === 'zset' ||
			keyInfo?.type === 'stream'
	);

	// Generate highlighted raw JSON for complex types
	let rawJsonHtml = $derived.by(() => {
		if (!keyInfo || !isComplexType || !rawView) return '';
		return highlightJson(JSON.stringify(keyInfo.value, null, 2), true);
	});

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
		try {
			// Always fetch with pagination params - backend will add metadata only for complex types
			keyInfo = await api.getKey(k, currentPage, pageSize);
			editValue =
				typeof keyInfo.value === 'string' ? keyInfo.value : JSON.stringify(keyInfo.value, null, 2);
			editTtl = keyInfo.ttl > 0 ? String(keyInfo.ttl) : '';
			startTtlCountdown(keyInfo.ttl);
		} catch (e) {
			error = getErrorMessage(e, 'Failed to load key');
			keyInfo = null;
		} finally {
			loading = false;
		}
	}

	function goToPage(page: number) {
		currentPage = page;
		loadKey(key);
	}

	function changePageSize(newSize: number) {
		pageSize = newSize;
		currentPage = 1; // Reset to first page when changing page size
		loadKey(key);
	}

	// Computed pagination info
	let totalPages = $derived(
		keyInfo?.pagination ? Math.ceil(keyInfo.pagination.total / keyInfo.pagination.pageSize) : 0
	);
	let showingStart = $derived(
		keyInfo?.pagination ? (keyInfo.pagination.page - 1) * keyInfo.pagination.pageSize + 1 : 0
	);
	let showingEnd = $derived(
		keyInfo?.pagination
			? Math.min(keyInfo.pagination.page * keyInfo.pagination.pageSize, keyInfo.pagination.total)
			: 0
	);

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

	// Item CRUD operations

	async function saveItemEdit() {
		if (!editingItem || !keyInfo) return;
		savingItem = true;
		try {
			const { type, identifier } = editingItem;
			if (type === 'list') {
				await api.listSet(key, identifier as number, editingValue);
			} else if (type === 'hash') {
				await api.hashSet(key, identifier as string, editingValue);
			} else if (type === 'zset') {
				if (!isValidScore(editingValue)) {
					toast.error('Invalid score value');
					return;
				}
				await api.zsetAdd(key, identifier as string, parseScore(editingValue));
			}
			toast.success('Item updated');
			cancelEditing();
			await loadKey(key);
		} catch (e) {
			toastError(e, 'Failed to update item');
		} finally {
			savingItem = false;
		}
	}

	async function deleteItem() {
		if (!itemToDelete || !keyInfo) return;
		try {
			const { type, identifier } = itemToDelete;
			if (type === 'list') {
				await api.listRemove(key, identifier as number);
			} else if (type === 'set') {
				await api.setRemove(key, identifier as string);
			} else if (type === 'hash') {
				await api.hashRemove(key, identifier as string);
			} else if (type === 'zset') {
				await api.zsetRemove(key, identifier as string);
			}
			toast.success('Item deleted');
			await loadKey(key);
		} catch (e) {
			toastError(e, 'Failed to delete item');
		} finally {
			itemDeleteDialogOpen = false;
			itemToDelete = null;
		}
	}

	async function addItem() {
		if (!keyInfo) return;
		addingItem = true;
		try {
			if (keyInfo.type === 'list') {
				if (!isNonEmpty(addFormData.value || '')) {
					toast.error('Value cannot be empty');
					return;
				}
				await api.listPush(key, addFormData.value!, addFormData.position || 'tail');
				toast.success('Item added');
			} else if (keyInfo.type === 'set') {
				if (!isNonEmpty(addFormData.member || '')) {
					toast.error('Member cannot be empty');
					return;
				}
				await api.setAdd(key, addFormData.member!);
				toast.success('Member added');
			} else if (keyInfo.type === 'hash') {
				if (!isNonEmpty(addFormData.field || '')) {
					toast.error('Field name cannot be empty');
					return;
				}
				await api.hashSet(key, addFormData.field!, addFormData.value || '');
				toast.success('Field added');
			} else if (keyInfo.type === 'zset') {
				if (!isNonEmpty(addFormData.member || '')) {
					toast.error('Member cannot be empty');
					return;
				}
				if (!isValidScore(addFormData.score || '')) {
					toast.error('Invalid score value');
					return;
				}
				await api.zsetAdd(key, addFormData.member!, parseScore(addFormData.score!));
				toast.success('Member added');
			} else if (keyInfo.type === 'stream') {
				const fields: Record<string, string> = {};
				for (const f of addFormData.streamFields || []) {
					if (!isNonEmpty(f.key)) {
						toast.error('Field name cannot be empty');
						return;
					}
					if (!isNonEmpty(f.value)) {
						toast.error('Field value cannot be empty');
						return;
					}
					fields[f.key] = f.value;
				}
				if (Object.keys(fields).length === 0) {
					toast.error('At least one field is required');
					return;
				}
				const result = await api.streamAdd(key, fields);
				toast.success(`Entry added: ${result.id}`);
			}
			resetAddForm();
			await loadKey(key);
		} catch (e) {
			toastError(e, 'Failed to add item');
		} finally {
			addingItem = false;
		}
	}

	function addStreamField() {
		if (addFormData.streamFields) {
			addFormData.streamFields = [...addFormData.streamFields, { key: '', value: '' }];
		}
	}

	function removeStreamField(index: number) {
		if (addFormData.streamFields && addFormData.streamFields.length > 1) {
			addFormData.streamFields = addFormData.streamFields.filter((_, i) => i !== index);
		}
	}

	function handleEditKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' && !e.shiftKey) {
			e.preventDefault();
			saveItemEdit();
		} else if (e.key === 'Escape') {
			cancelEditing();
		}
	}
</script>

<div class="flex h-full flex-col gap-4 p-6">
	{#if loading}
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
					{#if keyInfo.type === 'string'}
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
						title="Delete this key">Delete</Button
					>
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
			<div class="flex flex-1 flex-col gap-2 overflow-auto">
				{#if keyInfo.pagination}
					<div class="flex items-center justify-between gap-4 border-b border-border pb-2">
						<span class="text-sm text-muted-foreground">
							Showing {showingStart}–{showingEnd} of {keyInfo.pagination.total} items
						</span>
						<div class="flex items-center gap-2">
							<span class="text-xs text-muted-foreground">Page size:</span>
							<select
								bind:value={pageSize}
								onchange={(e) => changePageSize(Number(e.currentTarget.value))}
								class="cursor-pointer rounded border border-border bg-background px-2 py-1 text-xs"
							>
								<option value={50}>50</option>
								<option value={100}>100</option>
								<option value={200}>200</option>
								<option value={500}>500</option>
							</select>
							<div class="flex gap-1">
								<Button
									size="sm"
									variant="outline"
									onclick={() => goToPage(1)}
									disabled={currentPage === 1}
									class="h-8 w-8 cursor-pointer p-0"
									title="First page"
								>
									<ChevronsLeftIcon class="h-4 w-4" />
								</Button>
								<Button
									size="sm"
									variant="outline"
									onclick={() => goToPage(currentPage - 1)}
									disabled={currentPage === 1}
									class="h-8 w-8 cursor-pointer p-0"
									title="Previous page"
								>
									<ChevronLeftIcon class="h-4 w-4" />
								</Button>
								<span class="flex items-center px-3 py-1 text-sm">
									Page {currentPage} of {totalPages}
								</span>
								<Button
									size="sm"
									variant="outline"
									onclick={() => goToPage(currentPage + 1)}
									disabled={currentPage >= totalPages}
									class="h-8 w-8 cursor-pointer p-0"
									title="Next page"
								>
									<ChevronRightIcon class="h-4 w-4" />
								</Button>
								<Button
									size="sm"
									variant="outline"
									onclick={() => goToPage(totalPages)}
									disabled={currentPage >= totalPages}
									class="h-8 w-8 cursor-pointer p-0"
									title="Last page"
								>
									<ChevronsRightIcon class="h-4 w-4" />
								</Button>
							</div>
						</div>
					</div>
				{/if}
				<div class="flex items-center justify-between">
					<span class="text-sm text-muted-foreground">
						{#if keyInfo.pagination}
							{keyInfo.pagination.total} items total
						{:else}
							{keyInfo.length} items
						{/if}
					</span>
					<div class="flex items-center gap-2">
						{#if !readOnly}
							<Button
								size="sm"
								variant="outline"
								onclick={openAddForm}
								class="cursor-pointer"
								title="Add item to list"
							>
								<PlusIcon class="mr-1 h-4 w-4" />
								Add Item
							</Button>
						{/if}
						{#if !rawView && Object.keys(listHighlights).length > 0}
							<button
								type="button"
								onclick={() => (prettyPrint = !prettyPrint)}
								class="cursor-pointer rounded bg-muted px-2 py-1 text-xs text-foreground hover:bg-secondary"
								title={prettyPrint ? 'Compact JSON formatting' : 'Pretty-print JSON formatting'}
							>
								{prettyPrint ? 'Compact JSON' : 'Format JSON'}
							</button>
						{/if}
						<button
							type="button"
							onclick={() => (rawView = !rawView)}
							class="cursor-pointer rounded bg-muted px-2 py-1 text-xs text-foreground hover:bg-secondary"
							title={rawView ? 'View as structured table' : 'View as raw JSON document'}
						>
							{rawView ? 'Show as Table' : 'Show as Raw JSON'}
						</button>
					</div>
				</div>
				{#if showAddForm && keyInfo.type === 'list'}
					<div class="flex items-center gap-2 rounded border border-border bg-muted/50 p-3">
						<Input
							bind:value={addFormData.value}
							placeholder="Value"
							class="flex-1"
							onkeydown={(e) => e.key === 'Enter' && addItem()}
						/>
						<select
							bind:value={addFormData.position}
							class="cursor-pointer rounded border border-border bg-background px-2 py-2 text-sm"
						>
							<option value="tail">Append (tail)</option>
							<option value="head">Prepend (head)</option>
						</select>
						<Button
							size="sm"
							onclick={addItem}
							disabled={addingItem}
							class="cursor-pointer"
							title="Add item"
						>
							{addingItem ? 'Adding...' : 'Add'}
						</Button>
						<Button
							size="sm"
							variant="ghost"
							onclick={closeAddForm}
							class="cursor-pointer"
							title="Cancel"
						>
							<XIcon class="h-4 w-4" />
						</Button>
					</div>
				{/if}
				{#if rawView && rawJsonHtml}
					<div
						class="flex-1 overflow-auto rounded border border-border [&>pre]:m-0 [&>pre]:min-h-full [&>pre]:p-4 [&>pre]:text-sm"
					>
						{@html rawJsonHtml}
					</div>
				{:else}
					<Table.Root>
						<Table.Header>
							<Table.Row>
								<Table.Head class="w-16">Index</Table.Head>
								<Table.Head>Value</Table.Head>
								{#if !readOnly}
									<Table.Head class="w-24">Actions</Table.Head>
								{/if}
							</Table.Row>
						</Table.Header>
						<Table.Body>
							{#each asArray() as item, i}
								{@const realIndex = (currentPage - 1) * pageSize + i}
								<Table.Row>
									<Table.Cell class="align-top font-mono text-muted-foreground"
										>{realIndex}</Table.Cell
									>
									<Table.Cell class="font-mono">
										{#if editingItem?.type === 'list' && editingItem.identifier === realIndex}
											<div class="flex items-center gap-2">
												<Input
													bind:value={editingValue}
													class="flex-1 font-mono text-sm"
													onkeydown={handleEditKeydown}
												/>
												<Button
													size="sm"
													onclick={saveItemEdit}
													disabled={savingItem}
													class="cursor-pointer"
													title="Save"
												>
													<CheckIcon class="h-4 w-4" />
												</Button>
												<Button
													size="sm"
													variant="ghost"
													onclick={cancelEditing}
													class="cursor-pointer"
													title="Cancel"
												>
													<XIcon class="h-4 w-4" />
												</Button>
											</div>
										{:else}
											<CollapsibleValue value={item} highlight={listHighlights[i]} />
										{/if}
									</Table.Cell>
									{#if !readOnly}
										<Table.Cell class="align-top">
											{#if !(editingItem?.type === 'list' && editingItem.identifier === realIndex)}
												<div class="flex gap-1">
													<Button
														size="sm"
														variant="ghost"
														onclick={() => startEditing('list', realIndex, item)}
														class="h-8 w-8 cursor-pointer p-0"
														title="Edit item"
													>
														<PencilIcon class="h-4 w-4" />
													</Button>
													<Button
														size="sm"
														variant="ghost"
														onclick={() =>
															openItemDeleteDialog(
																'list',
																realIndex,
																item.length > 50 ? item.slice(0, 50) + '...' : item
															)}
														class="h-8 w-8 cursor-pointer p-0 text-destructive hover:text-destructive"
														title="Delete item"
													>
														<Trash2Icon class="h-4 w-4" />
													</Button>
												</div>
											{/if}
										</Table.Cell>
									{/if}
								</Table.Row>
							{/each}
						</Table.Body>
					</Table.Root>
				{/if}
			</div>
		{:else if keyInfo.type === 'set'}
			<div class="flex flex-1 flex-col gap-2 overflow-auto">
				{#if keyInfo.pagination}
					<div class="flex items-center justify-between gap-4 border-b border-border pb-2">
						<span class="text-sm text-muted-foreground">
							Showing {showingStart}–{showingEnd} of {keyInfo.pagination.total} members
						</span>
						<div class="flex items-center gap-2">
							<span class="text-xs text-muted-foreground">Page size:</span>
							<select
								bind:value={pageSize}
								onchange={(e) => changePageSize(Number(e.currentTarget.value))}
								class="cursor-pointer rounded border border-border bg-background px-2 py-1 text-xs"
							>
								<option value={50}>50</option>
								<option value={100}>100</option>
								<option value={200}>200</option>
								<option value={500}>500</option>
							</select>
							<div class="flex gap-1">
								<Button
									size="sm"
									variant="outline"
									onclick={() => goToPage(1)}
									disabled={currentPage === 1}
									class="h-8 w-8 cursor-pointer p-0"
									title="First page"
								>
									<ChevronsLeftIcon class="h-4 w-4" />
								</Button>
								<Button
									size="sm"
									variant="outline"
									onclick={() => goToPage(currentPage - 1)}
									disabled={currentPage === 1}
									class="h-8 w-8 cursor-pointer p-0"
									title="Previous page"
								>
									<ChevronLeftIcon class="h-4 w-4" />
								</Button>
								<span class="flex items-center px-3 py-1 text-sm"
									>Page {currentPage} of {totalPages}</span
								>
								<Button
									size="sm"
									variant="outline"
									onclick={() => goToPage(currentPage + 1)}
									disabled={currentPage >= totalPages}
									class="h-8 w-8 cursor-pointer p-0"
									title="Next page"
								>
									<ChevronRightIcon class="h-4 w-4" />
								</Button>
								<Button
									size="sm"
									variant="outline"
									onclick={() => goToPage(totalPages)}
									disabled={currentPage >= totalPages}
									class="h-8 w-8 cursor-pointer p-0"
									title="Last page"
								>
									<ChevronsRightIcon class="h-4 w-4" />
								</Button>
							</div>
						</div>
					</div>
				{/if}
				<div class="flex items-center justify-between">
					<span class="text-sm text-muted-foreground">
						{#if keyInfo.pagination}
							{keyInfo.pagination.total} members total
						{:else}
							{keyInfo.length} members
						{/if}
					</span>
					<div class="flex items-center gap-2">
						{#if !readOnly}
							<Button
								size="sm"
								variant="outline"
								onclick={openAddForm}
								class="cursor-pointer"
								title="Add member to set"
							>
								<PlusIcon class="mr-1 h-4 w-4" />
								Add Member
							</Button>
						{/if}
						<button
							type="button"
							onclick={() => (rawView = !rawView)}
							class="cursor-pointer rounded bg-muted px-2 py-1 text-xs text-foreground hover:bg-secondary"
							title={rawView ? 'View as list of members' : 'View as raw JSON document'}
						>
							{rawView ? 'Show as List' : 'Show as Raw JSON'}
						</button>
					</div>
				</div>
				{#if showAddForm && keyInfo.type === 'set'}
					<div class="flex items-center gap-2 rounded border border-border bg-muted/50 p-3">
						<Input
							bind:value={addFormData.member}
							placeholder="Member"
							class="flex-1"
							onkeydown={(e) => e.key === 'Enter' && addItem()}
						/>
						<Button
							size="sm"
							onclick={addItem}
							disabled={addingItem}
							class="cursor-pointer"
							title="Add member"
						>
							{addingItem ? 'Adding...' : 'Add'}
						</Button>
						<Button
							size="sm"
							variant="ghost"
							onclick={closeAddForm}
							class="cursor-pointer"
							title="Cancel"
						>
							<XIcon class="h-4 w-4" />
						</Button>
					</div>
				{/if}
				{#if rawView && rawJsonHtml}
					<div
						class="flex-1 overflow-auto rounded border border-border [&>pre]:m-0 [&>pre]:min-h-full [&>pre]:p-4 [&>pre]:text-sm"
					>
						{@html rawJsonHtml}
					</div>
				{:else}
					<div class="flex flex-col gap-1">
						{#each asArray() as member}
							<div
								class="flex items-center justify-between rounded bg-muted px-2 py-1 font-mono text-sm"
							>
								<CollapsibleValue value={member} maxLength={100} />
								{#if !readOnly}
									<Button
										size="sm"
										variant="ghost"
										onclick={() =>
											openItemDeleteDialog(
												'set',
												member,
												member.length > 50 ? member.slice(0, 50) + '...' : member
											)}
										class="h-6 w-6 cursor-pointer p-0 text-destructive hover:text-destructive"
										title="Remove member"
									>
										<Trash2Icon class="h-4 w-4" />
									</Button>
								{/if}
							</div>
						{/each}
					</div>
				{/if}
			</div>
		{:else if keyInfo.type === 'hash'}
			<div class="flex flex-1 flex-col gap-2 overflow-auto">
				{#if keyInfo.pagination}
					<div class="flex items-center justify-between gap-4 border-b border-border pb-2">
						<span class="text-sm text-muted-foreground">
							Showing {showingStart}–{showingEnd} of {keyInfo.pagination.total} fields
						</span>
						<div class="flex items-center gap-2">
							<span class="text-xs text-muted-foreground">Page size:</span>
							<select
								bind:value={pageSize}
								onchange={(e) => changePageSize(Number(e.currentTarget.value))}
								class="cursor-pointer rounded border border-border bg-background px-2 py-1 text-xs"
							>
								<option value={50}>50</option>
								<option value={100}>100</option>
								<option value={200}>200</option>
								<option value={500}>500</option>
							</select>
							<div class="flex gap-1">
								<Button
									size="sm"
									variant="outline"
									onclick={() => goToPage(1)}
									disabled={currentPage === 1}
									class="h-8 w-8 cursor-pointer p-0"
									title="First page"
								>
									<ChevronsLeftIcon class="h-4 w-4" />
								</Button>
								<Button
									size="sm"
									variant="outline"
									onclick={() => goToPage(currentPage - 1)}
									disabled={currentPage === 1}
									class="h-8 w-8 cursor-pointer p-0"
									title="Previous page"
								>
									<ChevronLeftIcon class="h-4 w-4" />
								</Button>
								<span class="flex items-center px-3 py-1 text-sm"
									>Page {currentPage} of {totalPages}</span
								>
								<Button
									size="sm"
									variant="outline"
									onclick={() => goToPage(currentPage + 1)}
									disabled={currentPage >= totalPages}
									class="h-8 w-8 cursor-pointer p-0"
									title="Next page"
								>
									<ChevronRightIcon class="h-4 w-4" />
								</Button>
								<Button
									size="sm"
									variant="outline"
									onclick={() => goToPage(totalPages)}
									disabled={currentPage >= totalPages}
									class="h-8 w-8 cursor-pointer p-0"
									title="Last page"
								>
									<ChevronsRightIcon class="h-4 w-4" />
								</Button>
							</div>
						</div>
					</div>
				{/if}
				<div class="flex items-center justify-between">
					<span class="text-sm text-muted-foreground">
						{#if keyInfo.pagination}
							{keyInfo.pagination.total} fields total
						{:else}
							{keyInfo.length} fields
						{/if}
					</span>
					<div class="flex items-center gap-2">
						{#if !readOnly}
							<Button
								size="sm"
								variant="outline"
								onclick={openAddForm}
								class="cursor-pointer"
								title="Add field to hash"
							>
								<PlusIcon class="mr-1 h-4 w-4" />
								Add Field
							</Button>
						{/if}
						<button
							type="button"
							onclick={() => (rawView = !rawView)}
							class="cursor-pointer rounded bg-muted px-2 py-1 text-xs text-foreground hover:bg-secondary"
							title={rawView ? 'View as field/value table' : 'View as raw JSON document'}
						>
							{rawView ? 'Show as Table' : 'Show as Raw JSON'}
						</button>
					</div>
				</div>
				{#if showAddForm && keyInfo.type === 'hash'}
					<div class="flex items-center gap-2 rounded border border-border bg-muted/50 p-3">
						<Input
							bind:value={addFormData.field}
							placeholder="Field name"
							class="w-48"
							onkeydown={(e) => e.key === 'Enter' && addItem()}
						/>
						<Input
							bind:value={addFormData.value}
							placeholder="Value"
							class="flex-1"
							onkeydown={(e) => e.key === 'Enter' && addItem()}
						/>
						<Button
							size="sm"
							onclick={addItem}
							disabled={addingItem}
							class="cursor-pointer"
							title="Add field"
						>
							{addingItem ? 'Adding...' : 'Add'}
						</Button>
						<Button
							size="sm"
							variant="ghost"
							onclick={closeAddForm}
							class="cursor-pointer"
							title="Cancel"
						>
							<XIcon class="h-4 w-4" />
						</Button>
					</div>
				{/if}
				{#if rawView && rawJsonHtml}
					<div
						class="flex-1 overflow-auto rounded border border-border [&>pre]:m-0 [&>pre]:min-h-full [&>pre]:p-4 [&>pre]:text-sm"
					>
						{@html rawJsonHtml}
					</div>
				{:else}
					<Table.Root>
						<Table.Header>
							<Table.Row>
								<Table.Head>Field</Table.Head>
								<Table.Head>Value</Table.Head>
								{#if !readOnly}
									<Table.Head class="w-24">Actions</Table.Head>
								{/if}
							</Table.Row>
						</Table.Header>
						<Table.Body>
							{#each asHash() as { field, value }}
								<Table.Row>
									<Table.Cell class="align-top font-mono text-muted-foreground">{field}</Table.Cell>
									<Table.Cell class="font-mono">
										{#if editingItem?.type === 'hash' && editingItem.identifier === field}
											<div class="flex items-center gap-2">
												<Input
													bind:value={editingValue}
													class="flex-1 font-mono text-sm"
													onkeydown={handleEditKeydown}
												/>
												<Button
													size="sm"
													onclick={saveItemEdit}
													disabled={savingItem}
													class="cursor-pointer"
													title="Save"
												>
													<CheckIcon class="h-4 w-4" />
												</Button>
												<Button
													size="sm"
													variant="ghost"
													onclick={cancelEditing}
													class="cursor-pointer"
													title="Cancel"
												>
													<XIcon class="h-4 w-4" />
												</Button>
											</div>
										{:else}
											<CollapsibleValue
												{value}
												highlight={isJson(value) ? highlightJson(value, false) : undefined}
											/>
										{/if}
									</Table.Cell>
									{#if !readOnly}
										<Table.Cell class="align-top">
											{#if !(editingItem?.type === 'hash' && editingItem.identifier === field)}
												<div class="flex gap-1">
													<Button
														size="sm"
														variant="ghost"
														onclick={() => startEditing('hash', field, value)}
														class="h-8 w-8 cursor-pointer p-0"
														title="Edit value"
													>
														<PencilIcon class="h-4 w-4" />
													</Button>
													<Button
														size="sm"
														variant="ghost"
														onclick={() => openItemDeleteDialog('hash', field, field)}
														class="h-8 w-8 cursor-pointer p-0 text-destructive hover:text-destructive"
														title="Delete field"
													>
														<Trash2Icon class="h-4 w-4" />
													</Button>
												</div>
											{/if}
										</Table.Cell>
									{/if}
								</Table.Row>
							{/each}
						</Table.Body>
					</Table.Root>
				{/if}
			</div>
		{:else if keyInfo.type === 'zset'}
			<div class="flex flex-1 flex-col gap-2 overflow-auto">
				{#if keyInfo.pagination}
					<div class="flex items-center justify-between gap-4 border-b border-border pb-2">
						<span class="text-sm text-muted-foreground">
							Showing {showingStart}–{showingEnd} of {keyInfo.pagination.total} members
						</span>
						<div class="flex items-center gap-2">
							<span class="text-xs text-muted-foreground">Page size:</span>
							<select
								bind:value={pageSize}
								onchange={(e) => changePageSize(Number(e.currentTarget.value))}
								class="cursor-pointer rounded border border-border bg-background px-2 py-1 text-xs"
							>
								<option value={50}>50</option>
								<option value={100}>100</option>
								<option value={200}>200</option>
								<option value={500}>500</option>
							</select>
							<div class="flex gap-1">
								<Button
									size="sm"
									variant="outline"
									onclick={() => goToPage(1)}
									disabled={currentPage === 1}
									class="h-8 w-8 cursor-pointer p-0"
									title="First page"
								>
									<ChevronsLeftIcon class="h-4 w-4" />
								</Button>
								<Button
									size="sm"
									variant="outline"
									onclick={() => goToPage(currentPage - 1)}
									disabled={currentPage === 1}
									class="h-8 w-8 cursor-pointer p-0"
									title="Previous page"
								>
									<ChevronLeftIcon class="h-4 w-4" />
								</Button>
								<span class="flex items-center px-3 py-1 text-sm"
									>Page {currentPage} of {totalPages}</span
								>
								<Button
									size="sm"
									variant="outline"
									onclick={() => goToPage(currentPage + 1)}
									disabled={currentPage >= totalPages}
									class="h-8 w-8 cursor-pointer p-0"
									title="Next page"
								>
									<ChevronRightIcon class="h-4 w-4" />
								</Button>
								<Button
									size="sm"
									variant="outline"
									onclick={() => goToPage(totalPages)}
									disabled={currentPage >= totalPages}
									class="h-8 w-8 cursor-pointer p-0"
									title="Last page"
								>
									<ChevronsRightIcon class="h-4 w-4" />
								</Button>
							</div>
						</div>
					</div>
				{/if}
				<div class="flex items-center justify-between">
					<span class="text-sm text-muted-foreground">
						{#if keyInfo.pagination}
							{keyInfo.pagination.total} members total
						{:else}
							{keyInfo.length} members
						{/if}
					</span>
					<div class="flex items-center gap-2">
						{#if !readOnly}
							<Button
								size="sm"
								variant="outline"
								onclick={openAddForm}
								class="cursor-pointer"
								title="Add member to sorted set"
							>
								<PlusIcon class="mr-1 h-4 w-4" />
								Add Member
							</Button>
						{/if}
						<button
							type="button"
							onclick={() => (rawView = !rawView)}
							class="cursor-pointer rounded bg-muted px-2 py-1 text-xs text-foreground hover:bg-secondary"
							title={rawView ? 'View as member/score table' : 'View as raw JSON document'}
						>
							{rawView ? 'Show as Table' : 'Show as Raw JSON'}
						</button>
					</div>
				</div>
				{#if showAddForm && keyInfo.type === 'zset'}
					<div class="flex items-center gap-2 rounded border border-border bg-muted/50 p-3">
						<Input
							bind:value={addFormData.member}
							placeholder="Member"
							class="flex-1"
							onkeydown={(e) => e.key === 'Enter' && addItem()}
						/>
						<Input
							bind:value={addFormData.score}
							placeholder="Score"
							type="number"
							step="any"
							class="w-32"
							onkeydown={(e) => e.key === 'Enter' && addItem()}
						/>
						<Button
							size="sm"
							onclick={addItem}
							disabled={addingItem}
							class="cursor-pointer"
							title="Add member"
						>
							{addingItem ? 'Adding...' : 'Add'}
						</Button>
						<Button
							size="sm"
							variant="ghost"
							onclick={closeAddForm}
							class="cursor-pointer"
							title="Cancel"
						>
							<XIcon class="h-4 w-4" />
						</Button>
					</div>
				{/if}
				{#if rawView && rawJsonHtml}
					<div
						class="flex-1 overflow-auto rounded border border-border [&>pre]:m-0 [&>pre]:min-h-full [&>pre]:p-4 [&>pre]:text-sm"
					>
						{@html rawJsonHtml}
					</div>
				{:else}
					<Table.Root>
						<Table.Header>
							<Table.Row>
								<Table.Head>Member</Table.Head>
								<Table.Head class="w-32">Score</Table.Head>
								{#if !readOnly}
									<Table.Head class="w-24">Actions</Table.Head>
								{/if}
							</Table.Row>
						</Table.Header>
						<Table.Body>
							{#each asZSet() as zitem}
								<Table.Row>
									<Table.Cell class="font-mono">
										<CollapsibleValue value={zitem.member} />
									</Table.Cell>
									<Table.Cell class="font-mono text-muted-foreground">
										{#if editingItem?.type === 'zset' && editingItem.identifier === zitem.member}
											<div class="flex items-center gap-2">
												<Input
													bind:value={editingValue}
													type="number"
													step="any"
													class="w-24 font-mono text-sm"
													onkeydown={handleEditKeydown}
												/>
												<Button
													size="sm"
													onclick={saveItemEdit}
													disabled={savingItem}
													class="cursor-pointer"
													title="Save"
												>
													<CheckIcon class="h-4 w-4" />
												</Button>
												<Button
													size="sm"
													variant="ghost"
													onclick={cancelEditing}
													class="cursor-pointer"
													title="Cancel"
												>
													<XIcon class="h-4 w-4" />
												</Button>
											</div>
										{:else}
											{zitem.score}
										{/if}
									</Table.Cell>
									{#if !readOnly}
										<Table.Cell class="align-top">
											{#if !(editingItem?.type === 'zset' && editingItem.identifier === zitem.member)}
												<div class="flex gap-1">
													<Button
														size="sm"
														variant="ghost"
														onclick={() => startEditing('zset', zitem.member, String(zitem.score))}
														class="h-8 w-8 cursor-pointer p-0"
														title="Edit score"
													>
														<PencilIcon class="h-4 w-4" />
													</Button>
													<Button
														size="sm"
														variant="ghost"
														onclick={() =>
															openItemDeleteDialog(
																'zset',
																zitem.member,
																zitem.member.length > 50
																	? zitem.member.slice(0, 50) + '...'
																	: zitem.member
															)}
														class="h-8 w-8 cursor-pointer p-0 text-destructive hover:text-destructive"
														title="Delete member"
													>
														<Trash2Icon class="h-4 w-4" />
													</Button>
												</div>
											{/if}
										</Table.Cell>
									{/if}
								</Table.Row>
							{/each}
						</Table.Body>
					</Table.Root>
				{/if}
			</div>
		{:else if keyInfo.type === 'stream'}
			<div class="flex flex-1 flex-col gap-2 overflow-auto">
				{#if keyInfo.pagination}
					<div class="flex items-center justify-between gap-4 border-b border-border pb-2">
						<span class="text-sm text-muted-foreground">
							Showing {showingStart}–{showingEnd} of {keyInfo.pagination.total} entries
						</span>
						<div class="flex items-center gap-2">
							<span class="text-xs text-muted-foreground">Page size:</span>
							<select
								bind:value={pageSize}
								onchange={(e) => changePageSize(Number(e.currentTarget.value))}
								class="cursor-pointer rounded border border-border bg-background px-2 py-1 text-xs"
							>
								<option value={50}>50</option>
								<option value={100}>100</option>
								<option value={200}>200</option>
								<option value={500}>500</option>
							</select>
							<div class="flex gap-1">
								<Button
									size="sm"
									variant="outline"
									onclick={() => goToPage(1)}
									disabled={currentPage === 1}
									class="h-8 w-8 cursor-pointer p-0"
									title="First page"
								>
									<ChevronsLeftIcon class="h-4 w-4" />
								</Button>
								<Button
									size="sm"
									variant="outline"
									onclick={() => goToPage(currentPage - 1)}
									disabled={currentPage === 1}
									class="h-8 w-8 cursor-pointer p-0"
									title="Previous page"
								>
									<ChevronLeftIcon class="h-4 w-4" />
								</Button>
								<span class="flex items-center px-3 py-1 text-sm"
									>Page {currentPage} of {totalPages}</span
								>
								<Button
									size="sm"
									variant="outline"
									onclick={() => goToPage(currentPage + 1)}
									disabled={currentPage >= totalPages}
									class="h-8 w-8 cursor-pointer p-0"
									title="Next page"
								>
									<ChevronRightIcon class="h-4 w-4" />
								</Button>
								<Button
									size="sm"
									variant="outline"
									onclick={() => goToPage(totalPages)}
									disabled={currentPage >= totalPages}
									class="h-8 w-8 cursor-pointer p-0"
									title="Last page"
								>
									<ChevronsRightIcon class="h-4 w-4" />
								</Button>
							</div>
						</div>
					</div>
				{/if}
				<div class="flex items-center justify-between">
					<span class="text-sm text-muted-foreground">
						{#if keyInfo.pagination}
							{keyInfo.pagination.total} entries total
						{:else}
							{keyInfo.length} entries
						{/if}
					</span>
					<div class="flex items-center gap-2">
						{#if !readOnly}
							<Button
								size="sm"
								variant="outline"
								onclick={openAddForm}
								class="cursor-pointer"
								title="Add entry to stream"
							>
								<PlusIcon class="mr-1 h-4 w-4" />
								Add Entry
							</Button>
						{/if}
						<button
							type="button"
							onclick={() => (rawView = !rawView)}
							class="cursor-pointer rounded bg-muted px-2 py-1 text-xs text-foreground hover:bg-secondary"
							title={rawView ? 'View as entry cards' : 'View as raw JSON document'}
						>
							{rawView ? 'Show as Cards' : 'Show as Raw JSON'}
						</button>
					</div>
				</div>
				{#if showAddForm && keyInfo.type === 'stream'}
					<div class="flex flex-col gap-2 rounded border border-border bg-muted/50 p-3">
						<div class="text-sm text-muted-foreground">Add stream entry (append-only)</div>
						{#each addFormData.streamFields || [] as field, i}
							<div class="flex items-center gap-2">
								<Input bind:value={field.key} placeholder="Field name" class="w-48" />
								<Input bind:value={field.value} placeholder="Value" class="flex-1" />
								{#if (addFormData.streamFields?.length || 0) > 1}
									<Button
										size="sm"
										variant="ghost"
										onclick={() => removeStreamField(i)}
										class="h-8 w-8 cursor-pointer p-0"
										title="Remove field"
									>
										<XIcon class="h-4 w-4" />
									</Button>
								{/if}
							</div>
						{/each}
						<div class="flex items-center gap-2">
							<Button
								size="sm"
								variant="outline"
								onclick={addStreamField}
								class="cursor-pointer"
								title="Add another field"
							>
								<PlusIcon class="mr-1 h-4 w-4" />
								Add Field
							</Button>
							<div class="flex-1"></div>
							<Button
								size="sm"
								onclick={addItem}
								disabled={addingItem}
								class="cursor-pointer"
								title="Add entry"
							>
								{addingItem ? 'Adding...' : 'Add Entry'}
							</Button>
							<Button
								size="sm"
								variant="ghost"
								onclick={closeAddForm}
								class="cursor-pointer"
								title="Cancel"
							>
								<XIcon class="h-4 w-4" />
							</Button>
						</div>
					</div>
				{/if}
				{#if rawView && rawJsonHtml}
					<div
						class="flex-1 overflow-auto rounded border border-border [&>pre]:m-0 [&>pre]:min-h-full [&>pre]:p-4 [&>pre]:text-sm"
					>
						{@html rawJsonHtml}
					</div>
				{:else}
					<div class="flex flex-col gap-2">
						{#each asStream() as entry}
							<div class="rounded border border-border p-3">
								<div class="mb-2 font-mono text-xs text-muted-foreground">{entry.id}</div>
								<div class="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm">
									{#each Object.entries(entry.fields) as [field, val]}
										<span class="font-mono text-muted-foreground">{field}</span>
										<span class="font-mono">
											<CollapsibleValue value={val} maxLength={150} />
										</span>
									{/each}
								</div>
							</div>
						{/each}
					</div>
				{/if}
			</div>
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

		<AlertDialog.Root bind:open={itemDeleteDialogOpen}>
			<AlertDialog.Content>
				<AlertDialog.Header>
					<AlertDialog.Title>Delete Item</AlertDialog.Title>
					<AlertDialog.Description>
						{#if itemToDelete}
							Are you sure you want to delete this {itemToDelete.type === 'list'
								? 'list item'
								: itemToDelete.type === 'set'
									? 'set member'
									: itemToDelete.type === 'hash'
										? 'hash field'
										: 'sorted set member'}?
							<div class="mt-2">
								<code class="rounded bg-muted px-1 font-mono break-all">{itemToDelete.display}</code
								>
							</div>
						{/if}
					</AlertDialog.Description>
				</AlertDialog.Header>
				<AlertDialog.Footer>
					<AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
					<AlertDialog.Action onclick={deleteItem}>Delete</AlertDialog.Action>
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
