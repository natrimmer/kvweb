<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Check, FileEdit, Pencil, Trash2, X } from '@lucide/svelte/icons';

	interface Props {
		editing?: boolean;
		saving?: boolean;
		showEdit?: boolean;
		showRename?: boolean;
		editLabel?: string;
		renameLabel?: string;
		onEdit?: () => void;
		onSave?: () => void;
		onCancel?: () => void;
		onRename?: () => void;
		onDelete?: () => void;
	}

	let {
		editing = false,
		saving = false,
		showEdit = true,
		showRename = false,
		editLabel = 'Edit',
		renameLabel = 'Rename',
		onEdit,
		onSave,
		onCancel,
		onRename,
		onDelete
	}: Props = $props();
</script>

<div class="flex gap-1">
	{#if editing}
		<Button
			size="sm"
			onclick={onSave}
			disabled={saving}
			title="Save"
			aria-label="Save"
			class="h-8 w-8 p-0"
		>
			<Check class="h-4 w-4" />
		</Button>
		<Button
			size="sm"
			variant="ghost"
			onclick={onCancel}
			title="Cancel"
			aria-label="Cancel"
			class="h-8 w-8 p-0"
		>
			<X class="h-4 w-4" />
		</Button>
	{:else}
		{#if showEdit && onEdit}
			<Button
				size="sm"
				variant="ghost"
				onclick={onEdit}
				title={editLabel}
				aria-label={editLabel}
				class="h-8 w-8 p-0"
			>
				<Pencil class="h-4 w-4" />
			</Button>
		{/if}
		{#if showRename && onRename}
			<Button
				size="sm"
				variant="ghost"
				onclick={onRename}
				title={renameLabel}
				aria-label={renameLabel}
				class="h-8 w-8 p-0"
			>
				<FileEdit class="h-4 w-4" />
			</Button>
		{/if}
		{#if onDelete}
			<Button
				size="sm"
				variant="ghost"
				onclick={onDelete}
				title="Delete"
				aria-label="Delete"
				class="h-8 w-8 p-0 text-destructive hover:text-destructive"
			>
				<Trash2 class="h-4 w-4" />
			</Button>
		{/if}
	{/if}
</div>
