<script lang="ts">
	import AddItemForm from '$lib/AddItemForm.svelte';
	import { api, type PaginationInfo } from '$lib/api';
	import CollapsibleValue from '$lib/CollapsibleValue.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as ButtonGroup from '$lib/components/ui/button-group';
	import { Input } from '$lib/components/ui/input';
	import DeleteItemDialog from '$lib/DeleteItemDialog.svelte';
	import PaginationControls from '$lib/PaginationControls.svelte';
	import TypeHeader from '$lib/TypeHeader.svelte';
	import { highlightJson, showPaginationControls, toastError } from '$lib/utils';
	import ListIcon from '@lucide/svelte/icons/list';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';
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

	// Add form state
	let showAddForm = $state(false);
	let addMember = $state('');
	let adding = $state(false);

	// Delete state
	let deleteDialogOpen = $state(false);
	let deleteTarget = $state<{ member: string; display: string } | null>(null);

	let rawJsonHtml = $derived(rawView ? highlightJson(JSON.stringify(members, null, 2), true) : '');

	async function addItem() {
		if (!addMember.trim()) {
			toast.error('Member cannot be empty');
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

<div class="flex min-h-0 flex-1 flex-col gap-2">
	<TypeHeader expanded={typeHeaderExpanded}>
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
						title="Add member to set"
						aria-label="Add member to set"
					>
						<PlusIcon class="mr-1 h-4 w-4" />
						Add Member
					</Button>
				{/if}
				<ButtonGroup.Root>
					<Button
						size="sm"
						variant="outline"
						onclick={() => (rawView = false)}
						class="cursor-pointer {!rawView ? 'bg-accent' : ''}"
						title="Show as List"
						aria-label="Show as List"
					>
						<ListIcon class="h-4 w-4" />
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
					bind:value={addMember}
					placeholder="Member"
					class="flex-1"
					onkeydown={(e) => e.key === 'Enter' && addItem()}
					title="Member"
					aria-label="Member"
				/>
			</AddItemForm>
		{/if}
	</TypeHeader>

	<div class="-mx-6 min-h-0 flex-1 overflow-auto border-t border-border px-6 pt-2">
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
						class="flex items-center justify-between rounded bg-muted px-2 py-1 font-mono text-sm"
					>
						<CollapsibleValue value={member} maxLength={100} />
						{#if !readOnly}
							<Button
								size="sm"
								variant="ghost"
								onclick={() => openDeleteDialog(member)}
								title="Remove member"
								aria-label="Remove member"
								class="h-6 w-6 cursor-pointer p-0 text-destructive hover:text-destructive"
							>
								<Trash2Icon class="h-4 w-4" />
							</Button>
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
