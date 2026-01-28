<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Select from '$lib/components/ui/select';
	import ChevronLeftIcon from '@lucide/svelte/icons/chevron-left';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import ChevronsLeftIcon from '@lucide/svelte/icons/chevrons-left';
	import ChevronsRightIcon from '@lucide/svelte/icons/chevrons-right';

	interface Props {
		page: number;
		pageSize: number;
		total: number;
		itemLabel?: string;
		onPageChange: (page: number) => void;
		onPageSizeChange: (size: number) => void;
	}

	let {
		page,
		pageSize,
		total,
		itemLabel = 'items',
		onPageChange,
		onPageSizeChange
	}: Props = $props();

	const pageSizes = [50, 100, 200, 500] as const;
	const minPageSize = pageSizes[0];

	let totalPages = $derived(Math.ceil(total / pageSize));
	let showingStart = $derived((page - 1) * pageSize + 1);
	let showingEnd = $derived(Math.min(page * pageSize, total));
	let showPagination = $derived(total > minPageSize);

	function handlePageSizeChange(value: string | undefined) {
		if (value !== undefined) {
			onPageSizeChange(Number(value));
		}
	}
</script>

{#if showPagination}
	<div class="flex items-center justify-between gap-4 border-b border-border pb-2">
		<span class="text-sm text-muted-foreground">
			Showing {showingStart}–{showingEnd} of {total}
			{itemLabel}
		</span>
		<div class="flex items-center gap-2">
			<span class="text-xs text-muted-foreground">Page size:</span>
			<Select.Root type="single" value={String(pageSize)} onValueChange={handlePageSizeChange}>
				<Select.Trigger class="h-9 w-20 text-xs">
					{pageSize}
				</Select.Trigger>
				<Select.Content>
					{#each pageSizes as size}
						<Select.Item value={String(size)}>{size}</Select.Item>
					{/each}
				</Select.Content>
			</Select.Root>
			<div class="flex gap-1">
				<Button
					size="sm"
					variant="outline"
					onclick={() => onPageChange(1)}
					disabled={page === 1}
					class="size-9 cursor-pointer p-0"
					title="First page"
				>
					<ChevronsLeftIcon class="h-4 w-4" />
				</Button>
				<Button
					size="sm"
					variant="outline"
					onclick={() => onPageChange(page - 1)}
					disabled={page === 1}
					class="size-9 cursor-pointer p-0"
					title="Previous page"
				>
					<ChevronLeftIcon class="h-4 w-4" />
				</Button>
				<span class="flex items-center px-3 py-1 text-sm">
					Page {page} of {totalPages}
				</span>
				<Button
					size="sm"
					variant="outline"
					onclick={() => onPageChange(page + 1)}
					disabled={page >= totalPages}
					class="size-9 cursor-pointer p-0"
					title="Next page"
				>
					<ChevronRightIcon class="h-4 w-4" />
				</Button>
				<Button
					size="sm"
					variant="outline"
					onclick={() => onPageChange(totalPages)}
					disabled={page >= totalPages}
					class="size-9 cursor-pointer p-0"
					title="Last page"
				>
					<ChevronsRightIcon class="h-4 w-4" />
				</Button>
			</div>
		</div>
	</div>
{/if}
