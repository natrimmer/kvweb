<script lang="ts">
	import AddItemForm from '$lib/AddItemForm.svelte';
	import { api, type HashPair, type PaginationInfo } from '$lib/api';
	import CollapsibleValue from '$lib/CollapsibleValue.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
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
		fields: HashPair[];
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
		fields,
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

	// Add form state
	let showAddForm = $state(false);
	let addField = $state('');
	let addValue = $state('');
	let adding = $state(false);

	// Edit state
	let editingField = $state<string | null>(null);
	let editingValue = $state('');
	let saving = $state(false);

	// Delete state
	let deleteDialogOpen = $state(false);
	let deleteTarget = $state<{ field: string } | null>(null);

	let rawJsonHtml = $derived.by(() => {
		if (!rawView) return '';
		const obj: Record<string, string> = {};
		for (const { field, value } of fields) {
			obj[field] = value;
		}
		return highlightJson(JSON.stringify(obj, null, 2), true);
	});

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

	function startEditing(field: string, value: string) {
		editingField = field;
		editingValue = value;
	}

	function cancelEditing() {
		editingField = null;
		editingValue = '';
	}

	async function saveEdit(value: string) {
		if (editingField === null) return;
		saving = true;
		try {
			await api.hashSet(keyName, editingField, value);
			toast.success('Field updated');
			cancelEditing();
			onDataChange();
		} catch (e) {
			toastError(e, 'Failed to update field');
		} finally {
			saving = false;
		}
	}

	async function addItem() {
		if (!addField.trim()) {
			toast.error('Field name cannot be empty');
			return;
		}
		adding = true;
		try {
			await api.hashSet(keyName, addField, addValue);
			toast.success('Field added');
			addField = '';
			addValue = '';
			onDataChange();
		} catch (e) {
			toastError(e, 'Failed to add field');
		} finally {
			adding = false;
		}
	}

	function openDeleteDialog(field: string) {
		deleteTarget = { field };
		deleteDialogOpen = true;
	}

	async function confirmDelete() {
		if (!deleteTarget) return;
		try {
			await api.hashRemove(keyName, deleteTarget.field);
			toast.success('Field deleted');
			onDataChange();
		} catch (e) {
			toastError(e, 'Failed to delete field');
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
			itemLabel="fields"
			{onPageChange}
			{onPageSizeChange}
		/>
	{/if}

	<div class="flex items-center justify-between">
		<span class="text-sm text-muted-foreground">
			{pagination?.total ?? fields.length} fields total
		</span>
		<div class="flex items-center gap-2">
			{#if !readOnly}
				<Button
					size="sm"
					variant="outline"
					onclick={() => (showAddForm = true)}
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
			>
				{rawView ? 'Show as Table' : 'Show as Raw JSON'}
			</button>
		</div>
	</div>

	{#if showAddForm}
		<AddItemForm {adding} onAdd={addItem} onClose={() => (showAddForm = false)}>
			<Input
				bind:value={addField}
				placeholder="Field name"
				class="w-48"
				onkeydown={(e) => e.key === 'Enter' && addItem()}
			/>
			<Input
				bind:value={addValue}
				placeholder="Value"
				class="flex-1"
				onkeydown={(e) => e.key === 'Enter' && addItem()}
			/>
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
					<Table.Head>Field</Table.Head>
					<Table.Head>Value</Table.Head>
					{#if !readOnly}
						<Table.Head class="w-24">Actions</Table.Head>
					{/if}
				</Table.Row>
			</Table.Header>
			<Table.Body>
				{#each fields as { field, value }}
					<Table.Row>
						<Table.Cell class="align-top font-mono text-muted-foreground">{field}</Table.Cell>
						<Table.Cell class="font-mono">
							{#if editingField === field}
								<InlineEditor
									bind:value={editingValue}
									onSave={saveEdit}
									onCancel={cancelEditing}
								/>
							{:else}
								<CollapsibleValue
									{value}
									highlight={isJson(value) ? highlightJson(value, false) : undefined}
								/>
							{/if}
						</Table.Cell>
						{#if !readOnly}
							<Table.Cell class="align-top">
								<ItemActions
									editing={editingField === field}
									{saving}
									onEdit={() => startEditing(field, value)}
									onSave={() => saveEdit(editingValue)}
									onCancel={cancelEditing}
									onDelete={() => openDeleteDialog(field)}
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
	itemType="hash"
	itemDisplay={deleteTarget?.field ?? ''}
	onConfirm={confirmDelete}
	onCancel={() => (deleteDialogOpen = false)}
/>
