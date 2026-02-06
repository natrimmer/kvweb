<script lang="ts">
	import AddItemForm from '$lib/AddItemForm.svelte';
	import { api, type PaginationInfo } from '$lib/api';
	import CollapsibleValue from '$lib/CollapsibleValue.svelte';
	import ActionsToggle from '$lib/components/ActionsToggle.svelte';
	import TableWidthToggle from '$lib/components/TableWidthToggle.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as ButtonGroup from '$lib/components/ui/button-group';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import * as Table from '$lib/components/ui/table';
	import DeleteItemDialog from '$lib/dialogs/DeleteItemDialog.svelte';
	import LargeValueWarningDialog from '$lib/dialogs/LargeValueWarningDialog.svelte';
	import InlineEditor from '$lib/InlineEditor.svelte';
	import ItemActions from '$lib/ItemActions.svelte';
	import PaginationControls from '$lib/PaginationControls.svelte';
	import TypeHeader from '$lib/TypeHeader.svelte';
	import { highlightJson, isLargeValue, showPaginationControls, toastError } from '$lib/utils';
	import { Plus, TableIcon } from '@lucide/svelte/icons';
	import { toast } from 'svelte-sonner';

	interface Props {
		keyName: string;
		items: string[];
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
		items,
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
	let fullWidth = $state(false);
	let showActions = $state(true);
	let prettyPrint = $state(false);

	// Add form state
	let showAddForm = $state(false);
	let addValue = $state('');
	let addPosition = $state<'head' | 'tail'>('tail');
	let adding = $state(false);

	// Edit state
	let editingIndex = $state<number | null>(null);
	let editingValue = $state('');
	let saving = $state(false);

	// Delete state
	let deleteDialogOpen = $state(false);
	let deleteTarget = $state<{ index: number; display: string } | null>(null);

	// Large value warning
	let largeValueWarningOpen = $state(false);
	let largeValueSize = $state(0);
	let pendingAddValue: string | null = null;
	let pendingEditValue: { index: number; value: string } | null = null;

	const positionOptions = [
		{ value: 'tail', label: 'Append (tail)' },
		{ value: 'head', label: 'Prepend (head)' }
	] as const;

	let positionLabel = $derived(
		positionOptions.find((p) => p.value === addPosition)?.label ?? 'Append (tail)'
	);

	function handlePositionChange(value: string | undefined) {
		if (value === 'head' || value === 'tail') {
			addPosition = value;
		}
	}

	// JSON highlighting
	let listHighlights = $derived.by(() => {
		const highlights: Record<number, string> = {};
		for (let i = 0; i < items.length; i++) {
			if (isJson(items[i])) {
				highlights[i] = highlightJson(items[i], prettyPrint);
			}
		}
		return highlights;
	});

	let rawJsonHtml = $derived(rawView ? highlightJson(JSON.stringify(items, null, 2), true) : '');

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

	function startEditing(index: number, value: string) {
		editingIndex = index;
		editingValue = value;
	}

	function cancelEditing() {
		editingIndex = null;
		editingValue = '';
	}

	async function saveEdit(value: string) {
		if (editingIndex === null) return;

		// Check if value is large and needs confirmation
		if (isLargeValue(value) && (pendingEditValue === null || pendingEditValue.value !== value)) {
			largeValueSize = new Blob([value]).size;
			pendingEditValue = { index: editingIndex, value };
			largeValueWarningOpen = true;
			return;
		}

		saving = true;
		try {
			await api.listSet(keyName, editingIndex, value);
			toast.success('Item updated');
			cancelEditing();
			onDataChange();
		} catch (e) {
			toastError(e, 'Failed to update item');
		} finally {
			saving = false;
			pendingEditValue = null;
		}
	}

	async function addItem() {
		if (!addValue.trim()) {
			toast.error('Value cannot be empty');
			return;
		}

		// Check if value is large and needs confirmation
		if (isLargeValue(addValue) && pendingAddValue !== addValue) {
			largeValueSize = new Blob([addValue]).size;
			pendingAddValue = addValue;
			largeValueWarningOpen = true;
			return;
		}

		adding = true;
		try {
			await api.listPush(keyName, addValue, addPosition);
			toast.success('Item added');
			addValue = '';
			onDataChange();
		} catch (e) {
			toastError(e, 'Failed to add item');
		} finally {
			adding = false;
			pendingAddValue = null;
		}
	}

	function confirmLargeValue() {
		largeValueWarningOpen = false;
		if (pendingAddValue !== null) {
			addItem();
		} else if (pendingEditValue !== null) {
			saveEdit(pendingEditValue.value);
		}
	}

	function cancelLargeValue() {
		largeValueWarningOpen = false;
		pendingAddValue = null;
		pendingEditValue = null;
	}

	function openDeleteDialog(index: number, value: string) {
		deleteTarget = {
			index,
			display: value.length > 50 ? value.slice(0, 50) + '...' : value
		};
		deleteDialogOpen = true;
	}

	async function confirmDelete() {
		if (!deleteTarget) return;
		try {
			await api.listRemove(keyName, deleteTarget.index);
			toast.success('Item deleted');
			onDataChange();
		} catch (e) {
			toastError(e, 'Failed to delete item');
		} finally {
			deleteDialogOpen = false;
			deleteTarget = null;
		}
	}
</script>

<div class="flex min-h-0 flex-1 flex-col">
	<TypeHeader expanded={typeHeaderExpanded}>
		{#if pagination && showPaginationControls(pagination.total)}
			<PaginationControls
				page={currentPage}
				{pageSize}
				total={pagination.total}
				itemLabel="items"
				{onPageChange}
				{onPageSizeChange}
			/>
		{/if}

		<div class="flex items-center justify-between">
			<div class="flex-1">
				{#if pagination && !showPaginationControls(pagination.total)}
					<span class="text-sm text-muted-foreground">
						{pagination.total} item{pagination.total === 1 ? '' : 's'} total
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
						title="Add item to list"
						aria-label="Add item to list"
					>
						<Plus class="mr-1 h-4 w-4" />
						Add Item
					</Button>
				{/if}
				{#if !rawView && Object.keys(listHighlights).length > 0}
					<Button
						size="sm"
						variant="outline"
						onclick={() => (prettyPrint = !prettyPrint)}
						class="cursor-pointer"
						title={prettyPrint ? 'Show compact JSON' : 'Show formatted JSON'}
						aria-label={prettyPrint ? 'Show compact JSON' : 'Show formatted JSON'}
					>
						{prettyPrint ? 'Compact JSON' : 'Format JSON'}
					</Button>
				{/if}
				<ButtonGroup.Root>
					<Button
						size="sm"
						variant="outline"
						onclick={() => (rawView = false)}
						class="cursor-pointer {!rawView ? 'bg-accent' : ''}"
						title="Show as Table"
						aria-label="Show as Table"
					>
						<TableIcon class="h-4 w-4" />
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
				<TableWidthToggle {fullWidth} onToggle={(fw) => (fullWidth = fw)} disabled={rawView} />
				{#if !readOnly}
					<ActionsToggle {showActions} onToggle={(sa) => (showActions = sa)} disabled={rawView} />
				{/if}
			</div>
		</div>

		{#if showAddForm}
			<AddItemForm {adding} onAdd={addItem} onClose={() => (showAddForm = false)}>
				<Input
					bind:value={addValue}
					placeholder="Value"
					class="flex-1"
					onkeydown={(e) => e.key === 'Enter' && addItem()}
					title="Value"
					aria-label="Value"
				/>
				<Select.Root type="single" value={addPosition} onValueChange={handlePositionChange}>
					<Select.Trigger class="w-36">
						{positionLabel}
					</Select.Trigger>
					<Select.Content>
						{#each positionOptions as opt}
							<Select.Item value={opt.value}>{opt.label}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			</AddItemForm>
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
			<div class={fullWidth ? '' : 'max-w-max'}>
				<Table.Root class="table-auto">
					<Table.Header>
						<Table.Row>
							<Table.Head class="w-16">Index</Table.Head>
							<Table.Head class="w-auto">Value</Table.Head>
							{#if !readOnly && showActions}
								<Table.Head class="w-24">Actions</Table.Head>
							{/if}
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each items as item, i}
							{@const realIndex = (currentPage - 1) * pageSize + i}
							<Table.Row>
								<Table.Cell class="align-top font-mono text-muted-foreground"
									>{realIndex}</Table.Cell
								>
								<Table.Cell class="font-mono">
									{#if editingIndex === realIndex}
										<InlineEditor
											bind:value={editingValue}
											onSave={saveEdit}
											onCancel={cancelEditing}
										/>
									{:else}
										<CollapsibleValue value={item} highlight={listHighlights[i]} />
									{/if}
								</Table.Cell>
								{#if !readOnly && showActions}
									<Table.Cell class="align-top">
										<ItemActions
											editing={editingIndex === realIndex}
											{saving}
											onEdit={() => startEditing(realIndex, item)}
											onSave={() => saveEdit(editingValue)}
											onCancel={cancelEditing}
											onDelete={() => openDeleteDialog(realIndex, item)}
										/>
									</Table.Cell>
								{/if}
							</Table.Row>
						{/each}
					</Table.Body>
				</Table.Root>
			</div>
		{/if}
	</div>
</div>

<DeleteItemDialog
	bind:open={deleteDialogOpen}
	itemType="list"
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
