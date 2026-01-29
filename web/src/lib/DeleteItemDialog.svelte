<script lang="ts">
	import * as AlertDialog from '$lib/components/ui/alert-dialog';

	interface Props {
		open: boolean;
		itemType: string;
		itemDisplay: string;
		onConfirm: () => void;
		onCancel: () => void;
	}

	let { open = $bindable(), itemType, itemDisplay, onConfirm, onCancel }: Props = $props();

	function getItemLabel(type: string): string {
		switch (type) {
			case 'list':
				return 'list item';
			case 'set':
				return 'set member';
			case 'hash':
				return 'hash field';
			case 'zset':
				return 'sorted set member';
			default:
				return 'item';
		}
	}
</script>

<AlertDialog.Root bind:open>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>Delete Item</AlertDialog.Title>
			<AlertDialog.Description>
				Are you sure you want to delete this {getItemLabel(itemType)}?
				<div class="mt-2">
					<code class="rounded bg-muted px-1 font-mono break-all">{itemDisplay}</code>
				</div>
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel onclick={onCancel} title="Cancel" aria-label="Cancel">
				Cancel
			</AlertDialog.Cancel>
			<AlertDialog.Action onclick={onConfirm} title="Delete" aria-label="Delete">
				Delete
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
