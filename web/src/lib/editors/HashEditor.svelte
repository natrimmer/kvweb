<script lang="ts">
	import AddItemForm from '$lib/AddItemForm.svelte';
	import { api, type HashPair, type PaginationInfo } from '$lib/api';
	import CollapsibleValue from '$lib/CollapsibleValue.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as ButtonGroup from '$lib/components/ui/button-group';
	import { Input } from '$lib/components/ui/input';
	import * as Table from '$lib/components/ui/table';
	import DeleteItemDialog from '$lib/DeleteItemDialog.svelte';
	import InlineEditor from '$lib/InlineEditor.svelte';
	import ItemActions from '$lib/ItemActions.svelte';
	import PaginationControls from '$lib/PaginationControls.svelte';
	import TypeHeader from '$lib/TypeHeader.svelte';
	import { highlightJson, showPaginationControls, toastError } from '$lib/utils';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import TableIcon from '@lucide/svelte/icons/table';
	import { toast } from 'svelte-sonner';

	interface Props {
		keyName: string;
		fields: HashPair[];
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
		fields,
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

	// Add form state
	let showAddForm = $state(false);
	let addField = $state('');
	let addValue = $state('');
	let adding = $state(false);

	// Edit state
	let editMode = $state<'none' | 'value' | 'field'>('none');
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

	function startEditingValue(field: string, value: string) {
		editMode = 'value';
		editingField = field;
		editingValue = value;
	}

	function startRenamingField(field: string) {
		editMode = 'field';
		editingField = field;
		editingValue = field;
	}

	function cancelEditing() {
		editMode = 'none';
		editingField = null;
		editingValue = '';
	}

	async function saveEdit(value: string) {
		if (editingField === null) return;

		saving = true;
		try {
			if (editMode === 'value') {
				// Edit value
				await api.hashSet(keyName, editingField, value);
				toast.success('Value updated');
			} else if (editMode === 'field') {
				// Rename field
				if (!value.trim()) {
					toast.error('Field name cannot be empty');
					return;
				}
				if (value === editingField) {
					// No change, just cancel
					cancelEditing();
					return;
				}
				await api.hashRename(keyName, editingField, value.trim());
				toast.success('Field renamed');
			}
			cancelEditing();
			onDataChange();
		} catch (e) {
			toastError(e, editMode === 'value' ? 'Failed to update field' : 'Failed to rename field');
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

<div class="flex min-h-0 flex-1 flex-col">
	<TypeHeader expanded={typeHeaderExpanded}>
		{#if pagination && showPaginationControls(pagination.total)}
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
			<div class="flex-1">
				{#if pagination && !showPaginationControls(pagination.total)}
					<span class="text-sm text-muted-foreground">
						{pagination.total} field{pagination.total === 1 ? '' : 's'} total
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
						title="Add field to hash"
						aria-label="Add field to hash"
					>
						<PlusIcon class="mr-1 h-4 w-4" />
						Add Field
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
			</div>
		</div>

		{#if showAddForm}
			<AddItemForm {adding} onAdd={addItem} onClose={() => (showAddForm = false)}>
				<Input
					bind:value={addField}
					placeholder="Field name"
					class="w-48"
					onkeydown={(e) => e.key === 'Enter' && addItem()}
					title="Field name"
					aria-label="Field name"
				/>
				<Input
					bind:value={addValue}
					placeholder="Value"
					class="flex-1"
					onkeydown={(e) => e.key === 'Enter' && addItem()}
					title="Value"
					aria-label="Value"
				/>
			</AddItemForm>
		{/if}
	</TypeHeader>

	<div class="border-border-2 -mx-6 min-h-0 flex-1 overflow-auto border-t px-6 pt-6">
		{#if rawView && rawJsonHtml}
			<div
				class="rounded border border-border [&>pre]:m-0 [&>pre]:min-h-full [&>pre]:p-4 [&>pre]:text-sm"
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
							<Table.Cell class="align-top font-mono text-muted-foreground">
								{#if editMode === 'field' && editingField === field}
									<InlineEditor
										bind:value={editingValue}
										type="text"
										inputClass="w-full"
										onSave={saveEdit}
										onCancel={cancelEditing}
									/>
								{:else}
									{field}
								{/if}
							</Table.Cell>
							<Table.Cell class="font-mono">
								{#if editMode === 'value' && editingField === field}
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
										editing={editMode !== 'none' && editingField === field}
										{saving}
										showRename={true}
										editLabel="Edit value"
										renameLabel="Rename field"
										onEdit={() => startEditingValue(field, value)}
										onRename={() => startRenamingField(field)}
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
</div>

<DeleteItemDialog
	bind:open={deleteDialogOpen}
	itemType="hash"
	itemDisplay={deleteTarget?.field ?? ''}
	onConfirm={confirmDelete}
	onCancel={() => (deleteDialogOpen = false)}
/>
