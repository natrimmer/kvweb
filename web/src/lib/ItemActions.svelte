<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import CheckIcon from '@lucide/svelte/icons/check';
	import PencilIcon from '@lucide/svelte/icons/pencil';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';
	import XIcon from '@lucide/svelte/icons/x';

	interface Props {
		editing?: boolean;
		saving?: boolean;
		showEdit?: boolean;
		onEdit?: () => void;
		onSave?: () => void;
		onCancel?: () => void;
		onDelete?: () => void;
	}

	let {
		editing = false,
		saving = false,
		showEdit = true,
		onEdit,
		onSave,
		onCancel,
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
			class="h-8 w-8 cursor-pointer p-0"
		>
			<CheckIcon class="h-4 w-4" />
		</Button>
		<Button
			size="sm"
			variant="ghost"
			onclick={onCancel}
			title="Cancel"
			aria-label="Cancel"
			class="h-8 w-8 cursor-pointer p-0"
		>
			<XIcon class="h-4 w-4" />
		</Button>
	{:else}
		{#if showEdit && onEdit}
			<Button
				size="sm"
				variant="ghost"
				onclick={onEdit}
				title="Edit"
				aria-label="Edit"
				class="h-8 w-8 cursor-pointer p-0"
			>
				<PencilIcon class="h-4 w-4" />
			</Button>
		{/if}
		{#if onDelete}
			<Button
				size="sm"
				variant="ghost"
				onclick={onDelete}
				title="Delete"
				aria-label="Delete"
				class="h-8 w-8 cursor-pointer p-0 text-destructive hover:text-destructive"
			>
				<Trash2Icon class="h-4 w-4" />
			</Button>
		{/if}
	{/if}
</div>
