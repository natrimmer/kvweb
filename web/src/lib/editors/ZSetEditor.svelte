<script lang="ts">
	import AddItemForm from '$lib/AddItemForm.svelte';
	import { api, type PaginationInfo, type ZSetMember } from '$lib/api';
	import CollapsibleValue from '$lib/CollapsibleValue.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import * as Table from '$lib/components/ui/table';
	import DeleteItemDialog from '$lib/DeleteItemDialog.svelte';
	import InlineEditor from '$lib/InlineEditor.svelte';
	import ItemActions from '$lib/ItemActions.svelte';
	import PaginationControls from '$lib/PaginationControls.svelte';
	import {
		highlightJson,
		isValidScore,
		parseScore,
		showPaginationControls,
		toastError
	} from '$lib/utils';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import { toast } from 'svelte-sonner';

	interface Props {
		keyName: string;
		members: ZSetMember[];
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
		members,
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
	let addMember = $state('');
	let addScore = $state<string | number>('');
	let adding = $state(false);

	// Edit state
	let editingMember = $state<string | null>(null);
	let editingValue = $state('');
	let saving = $state(false);

	// Delete state
	let deleteDialogOpen = $state(false);
	let deleteTarget = $state<{ member: string; display: string } | null>(null);

	let rawJsonHtml = $derived(rawView ? highlightJson(JSON.stringify(members, null, 2), true) : '');

	function startEditing(member: string, score: number) {
		editingMember = member;
		editingValue = String(score);
	}

	function cancelEditing() {
		editingMember = null;
		editingValue = '';
	}

	async function saveEdit(value: string) {
		if (editingMember === null) return;
		if (!isValidScore(value)) {
			toast.error('Invalid score value');
			return;
		}
		saving = true;
		try {
			await api.zsetAdd(keyName, editingMember, parseScore(value));
			toast.success('Score updated');
			cancelEditing();
			onDataChange();
		} catch (e) {
			toastError(e, 'Failed to update score');
		} finally {
			saving = false;
		}
	}

	async function addItem() {
		if (!addMember.trim()) {
			toast.error('Member cannot be empty');
			return;
		}
		if (!isValidScore(addScore)) {
			toast.error('Invalid score value');
			return;
		}
		adding = true;
		try {
			await api.zsetAdd(keyName, addMember, parseScore(addScore));
			toast.success('Member added');
			addMember = '';
			addScore = '';
			onDataChange();
		} catch (e) {
			toastError(e, 'Failed to add member');
		} finally {
			adding = false;
		}
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
			await api.zsetRemove(keyName, deleteTarget.member);
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

<div class="flex flex-1 flex-col gap-2 overflow-auto">
	{#if pagination && showPaginationControls(pagination.total)}
		<PaginationControls
			page={currentPage}
			{pageSize}
			total={pagination.total}
			itemLabel="members"
			{onPageChange}
			{onPageSizeChange}
		/>
	{/if}

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
			>
				{rawView ? 'Show as Table' : 'Show as Raw JSON'}
			</button>
		</div>
	</div>

	{#if showAddForm}
		<AddItemForm {adding} onAdd={addItem} onClose={() => (showAddForm = false)}>
			<Input
				bind:value={addMember}
				placeholder="Member"
				class="flex-1"
				onkeydown={(e) => e.key === 'Enter' && addItem()}
			/>
			<Input
				bind:value={addScore}
				placeholder="Score"
				type="number"
				step="any"
				class="w-32"
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
					<Table.Head>Member</Table.Head>
					<Table.Head class="w-32">Score</Table.Head>
					{#if !readOnly}
						<Table.Head class="w-24">Actions</Table.Head>
					{/if}
				</Table.Row>
			</Table.Header>
			<Table.Body>
				{#each members as { member, score }}
					<Table.Row>
						<Table.Cell class="font-mono">
							<CollapsibleValue value={member} />
						</Table.Cell>
						<Table.Cell class="font-mono text-muted-foreground">
							{#if editingMember === member}
								<InlineEditor
									bind:value={editingValue}
									type="number"
									inputClass="w-24"
									onSave={saveEdit}
									onCancel={cancelEditing}
								/>
							{:else}
								{score}
							{/if}
						</Table.Cell>
						{#if !readOnly}
							<Table.Cell class="align-top">
								<ItemActions
									editing={editingMember === member}
									{saving}
									onEdit={() => startEditing(member, score)}
									onSave={() => saveEdit(editingValue)}
									onCancel={cancelEditing}
									onDelete={() => openDeleteDialog(member)}
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
	itemType="zset"
	itemDisplay={deleteTarget?.display ?? ''}
	onConfirm={confirmDelete}
	onCancel={() => (deleteDialogOpen = false)}
/>
