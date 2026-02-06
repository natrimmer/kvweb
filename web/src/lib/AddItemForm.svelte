<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { X } from '@lucide/svelte/icons';
	import type { Snippet } from 'svelte';

	interface Props {
		adding?: boolean;
		onAdd: () => void;
		onClose: () => void;
		children: Snippet;
	}

	let { adding = false, onAdd, onClose, children }: Props = $props();
</script>

<div class="flex items-center gap-2 rounded border border-border bg-muted/50 p-3">
	{@render children()}
	<Button
		size="sm"
		onclick={onAdd}
		disabled={adding}
		class="cursor-pointer"
		title="Add"
		aria-label="Add"
	>
		{adding ? 'Adding...' : 'Add'}
	</Button>
	<Button
		size="sm"
		variant="ghost"
		onclick={onClose}
		class="cursor-pointer"
		title="Cancel"
		aria-label="Cancel"
	>
		<X class="h-4 w-4" />
	</Button>
</div>
