<script lang="ts">
	import { api, type PaginationInfo, type StreamEntry } from '$lib/api';
	import ActionsToggle from '$lib/components/ActionsToggle.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as ButtonGroup from '$lib/components/ui/button-group';
	import { Input } from '$lib/components/ui/input';
	import DeleteItemDialog from '$lib/dialogs/DeleteItemDialog.svelte';
	import ExpandedItemDialog from '$lib/dialogs/ExpandedItemDialog.svelte';
	import LargeValueWarningDialog from '$lib/dialogs/LargeValueWarningDialog.svelte';
	import PaginationControls from '$lib/PaginationControls.svelte';
	import TypeHeader from '$lib/TypeHeader.svelte';
	import {
		highlightJson,
		isLargeValue,
		isNonEmpty,
		showPaginationControls,
		toastError
	} from '$lib/utils';
	import {
		Braces,
		ChevronsLeftRight,
		LayoutList,
		Plus,
		RemoveFormatting,
		Trash2,
		X
	} from '@lucide/svelte/icons';
	import { toast } from 'svelte-sonner';

	interface Props {
		keyName: string;
		entries: StreamEntry[];
		pagination: PaginationInfo | undefined;
		currentPage: number;
		pageSize: number;
		readOnly: boolean;
		typeHeaderExpanded: boolean;
		onPageChange: (page: number) => void;
		onPageSizeChange: (size: number) => void;
		onDataChange: () => void;
	}

	let {
		keyName,
		entries,
		pagination,
		currentPage,
		pageSize,
		readOnly,
		typeHeaderExpanded,
		onPageChange,
		onPageSizeChange,
		onDataChange
	}: Props = $props();

	// View state
	let rawView = $state(false);
	let showActions = $state(true);
	let prettyPrint = $state(false);

	// Add form state
	let showAddForm = $state(false);
	let streamFields = $state<{ key: string; value: string }[]>([{ key: '', value: '' }]);
	let adding = $state(false);

	// Delete state
	let deleteDialogOpen = $state(false);
	let deleteTarget = $state<{ id: string; display: string } | null>(null);

	// Large value warning
	let largeValueWarningOpen = $state(false);
	let largeValueSize = $state(0);
	let pendingAddFields: Record<string, string> | null = null;

	// Expanded view state
	let expandedDialogOpen = $state(false);
	let expandedValue = $state<string>('');
	let expandedTitle = $state<string>('');

	let rawJsonHtml = $derived(rawView ? highlightJson(JSON.stringify(entries, null, 2), true) : '');

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

	// JSON highlighting for stream field values
	let fieldHighlights = $derived.by(() => {
		const highlights: Record<string, Record<string, string>> = {};
		for (const entry of entries) {
			highlights[entry.id] = {};
			for (const [field, value] of Object.entries(entry.fields)) {
				if (isJson(value)) {
					highlights[entry.id][field] = highlightJson(value, prettyPrint);
				}
			}
		}
		return highlights;
	});

	function addField() {
		streamFields = [...streamFields, { key: '', value: '' }];
	}

	function removeField(index: number) {
		if (streamFields.length > 1) {
			streamFields = streamFields.filter((_, i) => i !== index);
		}
	}

	function resetForm() {
		streamFields = [{ key: '', value: '' }];
	}

	async function addItem() {
		const fields: Record<string, string> = {};
		for (const f of streamFields) {
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

		// Check if any field value is large and needs confirmation
		const fieldsString = JSON.stringify(fields);
		if (isLargeValue(fieldsString) && pendingAddFields !== fields) {
			// Find the largest value to report accurate size
			let maxSize = 0;
			for (const value of Object.values(fields)) {
				const size = new Blob([value]).size;
				if (size > maxSize) maxSize = size;
			}
			largeValueSize = maxSize;
			pendingAddFields = fields;
			largeValueWarningOpen = true;
			return;
		}

		adding = true;
		try {
			const result = await api.streamAdd(keyName, fields);
			toast.success(`Entry added: ${result.id}`);
			resetForm();
			onDataChange();
		} catch (e) {
			toastError(e, 'Failed to add entry');
		} finally {
			adding = false;
			pendingAddFields = null;
		}
	}

	function confirmLargeValue() {
		largeValueWarningOpen = false;
		if (pendingAddFields !== null) {
			addItem();
		}
	}

	function cancelLargeValue() {
		largeValueWarningOpen = false;
		pendingAddFields = null;
	}

	function openDeleteDialog(id: string) {
		deleteTarget = {
			id,
			display: id.length > 30 ? id.slice(0, 30) + '...' : id
		};
		deleteDialogOpen = true;
	}

	async function confirmDelete() {
		if (!deleteTarget) return;
		try {
			await api.streamRemove(keyName, deleteTarget.id);
			toast.success('Entry deleted');
			onDataChange();
		} catch (e) {
			toastError(e, 'Failed to delete entry');
		} finally {
			deleteDialogOpen = false;
			deleteTarget = null;
		}
	}

	function openExpandedView(title: string, value: string) {
		expandedTitle = title;
		expandedValue = value;
		expandedDialogOpen = true;
	}

	function closeExpandedView() {
		expandedDialogOpen = false;
		expandedValue = '';
		expandedTitle = '';
	}
</script>

<div class="flex min-h-0 flex-1 flex-col">
	<TypeHeader expanded={typeHeaderExpanded}>
		<div class="flex items-center justify-between">
			<div class="flex-1">
				{#if pagination && !showPaginationControls(pagination.total)}
					<span class="text-sm text-muted-foreground">
						{pagination.total} entr{pagination.total === 1 ? 'y' : 'ies'} total
					</span>
				{/if}
			</div>
			<div class="flex items-center gap-2">
				{#if !readOnly}
					<Button
						size="sm"
						variant="outline"
						onclick={() => (showAddForm = true)}
						class="cursor-pointer"
						title="Add entry to stream"
						aria-label="Add entry to stream"
					>
						<Plus class="mr-1 h-4 w-4" />
						Add Entry
					</Button>
				{/if}
				<ButtonGroup.Root>
					<Button
						size="sm"
						variant="outline"
						onclick={() => (prettyPrint = false)}
						disabled={rawView}
						class="cursor-pointer {!prettyPrint ? 'bg-accent' : ''}"
						title="Show compact JSON"
						aria-label="Show compact JSON"
					>
						<RemoveFormatting class="h-4 w-4" />
					</Button>
					<Button
						size="sm"
						variant="outline"
						onclick={() => (prettyPrint = true)}
						disabled={rawView}
						class="cursor-pointer {prettyPrint ? 'bg-accent' : ''}"
						title="Show formatted JSON"
						aria-label="Show formatted JSON"
					>
						<Braces class="h-4 w-4" />
					</Button>
				</ButtonGroup.Root>
				<ButtonGroup.Root>
					<Button
						size="sm"
						variant="outline"
						onclick={() => (rawView = false)}
						class="cursor-pointer {!rawView ? 'bg-accent' : ''}"
						title="Show as Cards"
						aria-label="Show as Cards"
					>
						<LayoutList class="h-4 w-4" />
					</Button>
					<Button
						size="sm"
						variant="outline"
						onclick={() => (rawView = true)}
						class="cursor-pointer {rawView ? 'bg-accent' : ''}"
						title="Show as Raw JSON"
						aria-label="Show as Raw JSON"
					>
						{'{ }'}
					</Button>
				</ButtonGroup.Root>
				{#if !readOnly}
					<ActionsToggle {showActions} onToggle={(sa) => (showActions = sa)} disabled={rawView} />
				{/if}
			</div>
		</div>

		{#if showAddForm}
			<div class="mt-3 flex flex-col gap-2 rounded border border-border bg-muted/50 p-3">
				<div class="text-sm text-muted-foreground">Add stream entry (append-only)</div>
				{#each streamFields as field, i}
					<div class="flex items-center gap-2">
						<Input bind:value={field.key} placeholder="Field name" class="w-48" />
						<Input bind:value={field.value} placeholder="Value" class="flex-1" />
						{#if streamFields.length > 1}
							<Button
								size="sm"
								variant="ghost"
								onclick={() => removeField(i)}
								title="Remove field"
								aria-label="Remove field"
								class="h-8 w-8 cursor-pointer p-0"
							>
								<X class="h-4 w-4" />
							</Button>
						{/if}
					</div>
				{/each}
				<div class="flex items-center gap-2">
					<Button
						size="sm"
						variant="outline"
						onclick={addField}
						class="cursor-pointer"
						title="Add another field"
						aria-label="Add another field"
					>
						<Plus class="mr-1 h-4 w-4" />
						Add Field
					</Button>
					<div class="flex-1"></div>
					<Button
						size="sm"
						onclick={addItem}
						disabled={adding}
						class="cursor-pointer"
						title="Add entry"
						aria-label="Add entry"
					>
						{adding ? 'Adding...' : 'Add Entry'}
					</Button>
					<Button
						size="sm"
						variant="ghost"
						onclick={() => {
							showAddForm = false;
							resetForm();
						}}
						title="Cancel"
						aria-label="Cancel"
						class="cursor-pointer"
					>
						<X class="h-4 w-4" />
					</Button>
				</div>
			</div>
		{/if}

		{#if pagination && showPaginationControls(pagination.total)}
			<div class="pt-3">
				<PaginationControls
					page={currentPage}
					{pageSize}
					total={pagination.total}
					itemLabel="entries"
					{onPageChange}
					{onPageSizeChange}
				/>
			</div>
		{/if}
	</TypeHeader>

	<div class="-mx-6 min-h-0 flex-1 overflow-auto border-t border-border px-6 pt-6">
		{#if rawView && rawJsonHtml}
			<div
				class="rounded border border-border [&>pre]:m-0 [&>pre]:min-h-full [&>pre]:p-4 [&>pre]:text-sm"
			>
				{@html rawJsonHtml}
			</div>
		{:else}
			<div class="flex flex-col gap-2">
				{#each entries as entry}
					<div class="flex gap-2 rounded border border-border p-3">
						<div class="flex items-start pt-0.5">
							<Button
								size="sm"
								variant="outline"
								onclick={() =>
									openExpandedView(`Entry ${entry.id}`, JSON.stringify(entry.fields, null, 2))}
								class="h-6 w-6 shrink-0 cursor-pointer p-0"
								title="Expand to full view"
								aria-label="Expand to full view"
							>
								<ChevronsLeftRight class="h-3 w-3" />
							</Button>
						</div>
						<div class="flex-1">
							<div class="mb-2 flex items-center justify-between gap-2">
								<div class="font-mono text-xs text-muted-foreground">{entry.id}</div>
								{#if !readOnly && showActions}
									<Button
										size="sm"
										variant="ghost"
										onclick={() => openDeleteDialog(entry.id)}
										title="Delete entry"
										aria-label="Delete entry"
										class="h-6 w-6 cursor-pointer p-0 text-destructive hover:text-destructive"
									>
										<Trash2 class="h-4 w-4" />
									</Button>
								{/if}
							</div>
							<div class="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm">
								{#each Object.entries(entry.fields) as [field, val]}
									<span class="font-mono text-muted-foreground">{field}</span>
									<span class="font-mono">
										<div class="flex items-center gap-1">
											{#if fieldHighlights[entry.id]?.[field]}
												<!-- JSON value with highlighting -->
												<div
													class="[&>pre]:m-0 [&>pre]:overflow-hidden [&>pre]:bg-transparent [&>pre]:p-0 [&>pre]:text-sm [&>pre]:text-ellipsis [&>pre]:whitespace-nowrap"
												>
													{@html fieldHighlights[entry.id][field]}
												</div>
											{:else}
												<!-- Plain text value -->
												<span class="break-all">
													{val.length > 100 ? val.slice(0, 100) + '…' : val}
												</span>
											{/if}
										</div>
									</span>
								{/each}
							</div>
						</div>
					</div>
				{/each}
			</div>
		{/if}
	</div>
</div>

<DeleteItemDialog
	bind:open={deleteDialogOpen}
	itemType="stream entry"
	itemDisplay={deleteTarget?.display ?? ''}
	onConfirm={confirmDelete}
	onCancel={() => (deleteDialogOpen = false)}
/>

<LargeValueWarningDialog
	bind:open={largeValueWarningOpen}
	valueSize={largeValueSize}
	onConfirm={confirmLargeValue}
	onCancel={cancelLargeValue}
/>

<ExpandedItemDialog
	bind:open={expandedDialogOpen}
	title={expandedTitle}
	value={expandedValue}
	readOnly={true}
	onCancel={closeExpandedView}
/>
