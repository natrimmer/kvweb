<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import { TriangleAlert } from '@lucide/svelte/icons';

	interface Props {
		open: boolean;
		valueSize: number;
		onConfirm: () => void;
		onCancel: () => void;
	}

	let { open = $bindable(), valueSize, onConfirm, onCancel }: Props = $props();

	function formatSize(bytes: number): string {
		if (bytes < 1024) return `${bytes} bytes`;
		if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
		return `${(bytes / (1024 * 1024)).toFixed(2)} MB`;
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="max-w-md">
		<Dialog.Header>
			<Dialog.Title class="flex items-center gap-2">
				<TriangleAlert class="h-5 w-5 text-yellow-500" />
				Large Value Warning
			</Dialog.Title>
			<Dialog.Description>
				The value you're trying to add is {formatSize(valueSize)}, which exceeds the recommended 1MB
				limit.
			</Dialog.Description>
		</Dialog.Header>
		<div class="space-y-3 pt-4 text-sm text-muted-foreground">
			<p>Large values can cause performance issues:</p>
			<ul class="mb-8 ml-5 list-disc space-y-1">
				<li>Slower read/write operations</li>
				<li>Increased memory usage</li>
				<li>Higher network latency</li>
				<li>Replication delays</li>
			</ul>
			<p class="font-medium text-foreground">
				Consider storing large data in object storage (S3, etc.) and keeping only references in
				Redis/Valkey.
			</p>
		</div>
		<Dialog.Footer>
			<Button variant="outline" onclick={onCancel}>Cancel</Button>
			<Button variant="default" onclick={onConfirm} class="bg-yellow-600 hover:bg-yellow-700">
				Proceed Anyway
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
