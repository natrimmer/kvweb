<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import { formatBytes, largeValueThreshold } from '$lib/utils';
	import { TriangleAlert } from '@lucide/svelte/icons';

	interface Props {
		open: boolean;
		valueSize: number;
		onConfirm: () => void;
		onCancel: () => void;
	}

	let { open = $bindable(), valueSize, onConfirm, onCancel }: Props = $props();
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="max-w-md">
		<Dialog.Header>
			<Dialog.Title class="flex items-center gap-2">
				<TriangleAlert class="h-5 w-5 text-yellow-500" />
				Large Value
			</Dialog.Title>
			<Dialog.Description>
				This value is {formatBytes(valueSize)}, over the {formatBytes(largeValueThreshold)} warning threshold.
			</Dialog.Description>
		</Dialog.Header>
		<div class="space-y-3 pt-4 text-sm text-muted-foreground">
			<p>Storing values this size means:</p>
			<ul class="ml-5 list-disc space-y-1">
				<li>Slower read/write operations</li>
				<li>Increased memory usage</li>
				<li>Higher network latency</li>
				<li>Replication delays</li>
			</ul>
			<p class="mb-8">
				kvweb caps each request at 1 MB total, so a value near that size may be refused even after
				you confirm.
			</p>
			<p class="font-medium text-foreground">
				Consider storing large data in object storage (S3, etc.) and keeping only references in
				Valkey/Redis.
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
