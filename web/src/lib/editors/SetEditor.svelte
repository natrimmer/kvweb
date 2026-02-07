<script lang="ts">
	import AddItemForm from '$lib/AddItemForm.svelte';
	import { api, type PaginationInfo } from '$lib/api';
	import CollapsibleValue from '$lib/CollapsibleValue.svelte';
	import ActionsToggle from '$lib/components/ActionsToggle.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as ButtonGroup from '$lib/components/ui/button-group';
	import { Input } from '$lib/components/ui/input';
	import DeleteItemDialog from '$lib/dialogs/DeleteItemDialog.svelte';
	import LargeValueWarningDialog from '$lib/dialogs/LargeValueWarningDialog.svelte';
	import InlineEditor from '$lib/InlineEditor.svelte';
	import ItemActions from '$lib/ItemActions.svelte';
	import PaginationControls from '$lib/PaginationControls.svelte';
	import TypeHeader from '$lib/TypeHeader.svelte';
	import { highlightJson, isLargeValue, showPaginationControls, toastError } from '$lib/utils';
	import { Braces, List, Plus, RemoveFormatting } from '@lucide/svelte/icons';
	import { toast } from 'svelte-sonner';

	interface Props {
		keyName: string;
		members: string[];
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
		members,
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
	let addMember = $state('');
	let adding = $state(false);

	// Edit state
	let editingMember = $state<string | null>(null);
	let editingValue = $state('');
	let saving = $state(false);

	// Delete state
	let deleteDialogOpen = $state(false);
	let deleteTarget = $state<{ member: string; display: string } | null>(null);

	// Large value warning
	let largeValueWarningOpen = $state(false);
	let largeValueSize = $state(0);
	let pendingAddMember: string | null = null;
	let pendingEditMember: { old: string; new: string } | null = null;

	let rawJsonHtml = $derived(rawView ? highlightJson(JSON.stringify(members, null, 2), true) : '');

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

	// JSON highlighting for members
	let memberHighlights = $derived.by(() => {
		const highlights: Record<string, string> = {};
		for (const member of members) {
			if (isJson(member)) {
				highlights[member] = highlightJson(member, prettyPrint);
			}
		}
		return highlights;
	});

	async function addItem() {
		if (!addMember.trim()) {
			toast.error('Member cannot be empty');
			return;
		}

		// Check if value is large and needs confirmation
		if (isLargeValue(addMember) && pendingAddMember !== addMember) {
			largeValueSize = new Blob([addMember]).size;
			pendingAddMember = addMember;
			largeValueWarningOpen = true;
			return;
		}

		adding = true;
		try {
			await api.setAdd(keyName, addMember);
			toast.success('Member added');
			addMember = '';
			onDataChange();
		} catch (e) {
			toastError(e, 'Failed to add member');
		} finally {
			adding = false;
			pendingAddMember = null;
		}
	}

	function startEditingMember(member: string) {
		editingMember = member;
		editingValue = member;
	}

	function cancelEditing() {
		editingMember = null;
		editingValue = '';
	}

	async function saveEdit(value: string) {
		if (editingMember === null) return;

		if (!value.trim()) {
			toast.error('Member cannot be empty');
			return;
		}

		if (value === editingMember) {
			// No change, just cancel
			cancelEditing();
			return;
		}

		// Check if value is large and needs confirmation
		if (isLargeValue(value) && (pendingEditMember === null || pendingEditMember.new !== value)) {
			largeValueSize = new Blob([value]).size;
			pendingEditMember = { old: editingMember, new: value };
			largeValueWarningOpen = true;
			return;
		}

		saving = true;
		try {
			await api.setRename(keyName, editingMember, value.trim());
			toast.success('Member updated');
			cancelEditing();
			onDataChange();
		} catch (e) {
			toastError(e, 'Failed to update member');
		} finally {
			saving = false;
			pendingEditMember = null;
		}
	}

	function confirmLargeValue() {
		largeValueWarningOpen = false;
		if (pendingAddMember !== null) {
			addItem();
		} else if (pendingEditMember !== null) {
			saveEdit(pendingEditMember.new);
		}
	}

	function cancelLargeValue() {
		largeValueWarningOpen = false;
		pendingAddMember = null;
		pendingEditMember = null;
	}

	function openDeleteDialog(member: string) {
		deleteTarget = {
			member,
			display: member.length > 50 ? member.slice(0, 50) + '...' : member
		};
		deleteDialogOpen = true;
	}

	async function confirmDelete() {
		if (!deleteTarget) return;
		try {
			await api.setRemove(keyName, deleteTarget.member);
			toast.success('Member deleted');
			onDataChange();
		} catch (e) {
			toastError(e, 'Failed to delete member');
		} finally {
			deleteDialogOpen = false;
			deleteTarget = null;
		}
	}
</script>

<div class="flex min-h-0 flex-1 flex-col">
	<TypeHeader expanded={typeHeaderExpanded}>
		<div class="flex items-center justify-between">
			<div class="flex-1">
				{#if pagination && !showPaginationControls(pagination.total)}
					<span class="text-sm text-muted-foreground">
						{pagination.total} member{pagination.total === 1 ? '' : 's'} total
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
						title="Add member to set"
						aria-label="Add member to set"
					>
						<Plus class="mr-1 h-4 w-4" />
						Add Member
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
						title="Show as List"
						aria-label="Show as List"
					>
						<List class="h-4 w-4" />
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
			<div class="pt-3">
				<AddItemForm {adding} onAdd={addItem} onClose={() => (showAddForm = false)}>
					<Input
						bind:value={addMember}
						placeholder="Member"
						class="flex-1"
						onkeydown={(e) => e.key === 'Enter' && addItem()}
						title="Member"
						aria-label="Member"
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
					itemLabel="members"
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
			<div class="flex flex-col gap-1">
				{#each members as member}
					<div
						class="flex items-center justify-between gap-2 rounded bg-muted px-2 py-1 font-mono text-sm"
					>
						{#if editingMember === member}
							<div class="flex-1">
								<InlineEditor
									bind:value={editingValue}
									onSave={saveEdit}
									onCancel={cancelEditing}
								/>
							</div>
						{:else}
							<CollapsibleValue
								value={member}
								maxLength={100}
								highlight={memberHighlights[member]}
							/>
						{/if}
						{#if !readOnly && showActions}
							<ItemActions
								editing={editingMember === member}
								{saving}
								editLabel="Edit member"
								onEdit={() => startEditingMember(member)}
								onSave={() => saveEdit(editingValue)}
								onCancel={cancelEditing}
								onDelete={() => openDeleteDialog(member)}
							/>
						{/if}
					</div>
				{/each}
			</div>
		{/if}
	</div>
</div>

<DeleteItemDialog
	bind:open={deleteDialogOpen}
	itemType="set"
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
