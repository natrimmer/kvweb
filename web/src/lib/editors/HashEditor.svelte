<script lang="ts">
	import AddItemForm from '$lib/AddItemForm.svelte';
	import { api, type HashPair, type PaginationInfo } from '$lib/api';
	import ActionsToggle from '$lib/components/ActionsToggle.svelte';
	import TableWidthToggle from '$lib/components/TableWidthToggle.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as ButtonGroup from '$lib/components/ui/button-group';
	import { Input } from '$lib/components/ui/input';
	import * as Table from '$lib/components/ui/table';
	import DeleteItemDialog from '$lib/dialogs/DeleteItemDialog.svelte';
	import ExpandedItemDialog from '$lib/dialogs/ExpandedItemDialog.svelte';
	import LargeValueWarningDialog from '$lib/dialogs/LargeValueWarningDialog.svelte';
	import InlineEditor from '$lib/InlineEditor.svelte';
	import ItemActions from '$lib/ItemActions.svelte';
	import PaginationControls from '$lib/PaginationControls.svelte';
	import TypeHeader from '$lib/TypeHeader.svelte';
	import { highlightJson, isLargeValue, showPaginationControls, toastError } from '$lib/utils';
	import {
		Braces,
		ChevronsLeftRight,
		Plus,
		RemoveFormatting,
		TableIcon
	} from '@lucide/svelte/icons';
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
	let fullWidth = $state(false);
	let showActions = $state(true);
	let prettyPrint = $state(false);

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

	// Large value warning
	let largeValueWarningOpen = $state(false);
	let largeValueSize = $state(0);
	let pendingAddField: { field: string; value: string } | null = null;
	let pendingEditField: { field: string; value: string } | null = null;

	// Expanded view state
	let expandedDialogOpen = $state(false);
	let expandedValue = $state<string>('');
	let expandedField = $state<string>('');

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

		// Check if we're editing a value and it's large
		if (editMode === 'value') {
			if (isLargeValue(value) && (pendingEditField === null || pendingEditField.value !== value)) {
				largeValueSize = new Blob([value]).size;
				pendingEditField = { field: editingField, value };
				largeValueWarningOpen = true;
				return;
			}
		}

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
			pendingEditField = null;
		}
	}

	async function addItem() {
		if (!addField.trim()) {
			toast.error('Field name cannot be empty');
			return;
		}

		// Check if value is large and needs confirmation
		if (
			isLargeValue(addValue) &&
			(pendingAddField === null || pendingAddField.value !== addValue)
		) {
			largeValueSize = new Blob([addValue]).size;
			pendingAddField = { field: addField, value: addValue };
			largeValueWarningOpen = true;
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
			pendingAddField = null;
		}
	}

	function confirmLargeValue() {
		largeValueWarningOpen = false;
		if (pendingAddField !== null) {
			addItem();
		} else if (pendingEditField !== null) {
			saveEdit(pendingEditField.value);
		}
	}

	function cancelLargeValue() {
		largeValueWarningOpen = false;
		pendingAddField = null;
		pendingEditField = null;
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

	function openExpandedView(field: string, value: string) {
		expandedField = field;
		expandedValue = value;
		expandedDialogOpen = true;
	}

	async function saveExpandedEdit(newValue: string) {
		if (!expandedField) return;
		await api.hashSet(keyName, expandedField, newValue);
		onDataChange();
	}

	function closeExpandedView() {
		expandedDialogOpen = false;
		expandedValue = '';
		expandedField = '';
	}
</script>

<div class="flex min-h-0 flex-1 flex-col">
	<TypeHeader expanded={typeHeaderExpanded}>
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
						<Plus class="mr-1 h-4 w-4" />
						Add Field
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
			<div class="mt-3">
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
			</div>
		{/if}

		{#if pagination && showPaginationControls(pagination.total)}
			<div class="pt-3">
				<PaginationControls
					page={currentPage}
					{pageSize}
					total={pagination.total}
					itemLabel="fields"
					{onPageChange}
					{onPageSizeChange}
				/>
			</div>
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
			<div class={fullWidth ? '' : 'max-w-max'}>
				<Table.Root class="table-auto">
					<Table.Header>
						<Table.Row>
							<Table.Head class="w-8"></Table.Head>
							<Table.Head class="w-auto">Field</Table.Head>
							<Table.Head class="w-auto">Value</Table.Head>
							{#if !readOnly && showActions}
								<Table.Head class="w-24">Actions</Table.Head>
							{/if}
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each fields as { field, value }}
							<Table.Row>
								<Table.Cell class="align-top">
									<Button
										size="sm"
										variant="outline"
										onclick={() => openExpandedView(field, value)}
										class="h-6 w-6 shrink-0 cursor-pointer p-0"
										title="Expand to full view"
										aria-label="Expand to full view"
									>
										<ChevronsLeftRight class="h-3 w-3" />
									</Button>
								</Table.Cell>
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
										<div class="flex items-center gap-1">
											{#if isJson(value)}
												<!-- JSON value with highlighting -->
												<div
													class="[&>pre]:m-0 [&>pre]:overflow-hidden [&>pre]:bg-transparent [&>pre]:p-0 [&>pre]:text-sm [&>pre]:text-ellipsis [&>pre]:whitespace-nowrap"
												>
													{@html highlightJson(value, prettyPrint)}
												</div>
											{:else}
												<!-- Plain text value -->
												<span class="break-all">
													{value.length > 100 ? value.slice(0, 100) + '…' : value}
												</span>
											{/if}
										</div>
									{/if}
								</Table.Cell>
								{#if !readOnly && showActions}
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
			</div>
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

<LargeValueWarningDialog
	bind:open={largeValueWarningOpen}
	valueSize={largeValueSize}
	onConfirm={confirmLargeValue}
	onCancel={cancelLargeValue}
/>

<ExpandedItemDialog
	bind:open={expandedDialogOpen}
	title="Hash Field '{expandedField}': {expandedValue.slice(0, 50)}{expandedValue.length > 50
		? '...'
		: ''}"
	value={expandedValue}
	{readOnly}
	onSave={saveExpandedEdit}
	onCancel={closeExpandedView}
/>
