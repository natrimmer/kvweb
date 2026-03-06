<script lang="ts">
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { api } from '$lib/api';
	import { toastError } from '$lib/utils';
	import { toast } from 'svelte-sonner';

	interface Props {
		open: boolean;
		keys: Set<string>;
		onDeleted: (count: number) => void;
		onCancel: () => void;
	}

	let { open = $bindable(), keys, onDeleted, onCancel }: Props = $props();

	let deleting = $state(false);

	const MAX_DISPLAY = 20;

	let keyList = $derived([...keys]);
	let displayKeys = $derived(keyList.slice(0, MAX_DISPLAY));
	let overflowCount = $derived(Math.max(0, keyList.length - MAX_DISPLAY));

	async function confirmDelete() {
		deleting = true;
		try {
			const result = await api.deleteKeys(keyList);
			toast.success(`Deleted ${result.deleted} key${result.deleted === 1 ? '' : 's'}`);
			open = false;
			onDeleted(result.deleted);
		} catch (e) {
			toastError(e, 'Failed to delete keys');
		} finally {
			deleting = false;
		}
	}
</script>

<AlertDialog.Root bind:open>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Delete {keys.size} Key{keys.size === 1 ? '' : 's'}</AlertDialog.Title>
			<AlertDialog.Description>
				Are you sure you want to delete {keys.size} key{keys.size === 1 ? '' : 's'}? This action
				cannot be undone.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<div class="max-h-48 overflow-y-auto rounded border border-border bg-muted/50 p-2">
			{#each displayKeys as key}
				<div class="truncate font-mono text-sm">{key}</div>
			{/each}
			{#if overflowCount > 0}
				<div class="mt-1 text-sm text-muted-foreground">+{overflowCount} more</div>
			{/if}
		</div>
		<AlertDialog.Footer>
			<AlertDialog.Cancel onclick={onCancel} title="Cancel" aria-label="Cancel">
				Cancel
			</AlertDialog.Cancel>
			<AlertDialog.Action
				onclick={confirmDelete}
				disabled={deleting}
				title="Delete"
				aria-label="Delete"
			>
				{deleting ? 'Deleting...' : 'Delete'}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
