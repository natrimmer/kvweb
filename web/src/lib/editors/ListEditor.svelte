<script lang="ts">
	import AddItemForm from '$lib/AddItemForm.svelte';
	import { api, type PaginationInfo } from '$lib/api';
	import CollapsibleValue from '$lib/CollapsibleValue.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import * as Table from '$lib/components/ui/table';
	import DeleteItemDialog from '$lib/DeleteItemDialog.svelte';
	import InlineEditor from '$lib/InlineEditor.svelte';
	import ItemActions from '$lib/ItemActions.svelte';
	import PaginationControls from '$lib/PaginationControls.svelte';
	import { highlightJson, toastError } from '$lib/utils';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import { toast } from 'svelte-sonner';

	interface Props {
		keyName: string;
		items: string[];
		pagination: PaginationInfo | undefined;
		currentPage: number;
		pageSize: number;
		readOnly: boolean;
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
		onPageChange,
		onPageSizeChange,
		onDataChange
	}: Props = $props();

	// View state
	let rawView = $state(false);
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
		}
	}

	async function addItem() {
		if (!addValue.trim()) {
			toast.error('Value cannot be empty');
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
		}
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

<div class="flex flex-1 flex-col gap-2 overflow-auto">
	{#if pagination}
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
		<span class="text-sm text-muted-foreground">
			{pagination?.total ?? items.length} items total
		</span>
		<div class="flex items-center gap-2">
			{#if !readOnly}
				<Button
					size="sm"
					variant="outline"
					onclick={() => (showAddForm = true)}
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
				>
					{prettyPrint ? 'Compact JSON' : 'Format JSON'}
				</button>
			{/if}
			<button
				type="button"
				onclick={() => (rawView = !rawView)}
				class="cursor-pointer rounded bg-muted px-2 py-1 text-xs text-foreground hover:bg-secondary"
			>
				{rawView ? 'Show as Table' : 'Show as Raw JSON'}
			</button>
		</div>
	</div>

	{#if showAddForm}
		<AddItemForm {adding} onAdd={addItem} onClose={() => (showAddForm = false)}>
			<Input
				bind:value={addValue}
				placeholder="Value"
				class="flex-1"
				onkeydown={(e) => e.key === 'Enter' && addItem()}
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
				{#each items as item, i}
					{@const realIndex = (currentPage - 1) * pageSize + i}
					<Table.Row>
						<Table.Cell class="align-top font-mono text-muted-foreground">{realIndex}</Table.Cell>
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
						{#if !readOnly}
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
	{/if}
</div>

<DeleteItemDialog
	bind:open={deleteDialogOpen}
	itemType="list"
	itemDisplay={deleteTarget?.display ?? ''}
	onConfirm={confirmDelete}
	onCancel={() => (deleteDialogOpen = false)}
/>
