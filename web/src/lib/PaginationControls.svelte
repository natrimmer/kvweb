<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as ButtonGroup from '$lib/components/ui/button-group';
	import * as Select from '$lib/components/ui/select';
	import { pageSizes } from '$lib/utils';
	import { ChevronLeft, ChevronRight, ChevronsLeft, ChevronsRight } from '@lucide/svelte/icons';

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

	let totalPages = $derived(Math.ceil(total / pageSize));
	let showingStart = $derived((page - 1) * pageSize + 1);
	let showingEnd = $derived(Math.min(page * pageSize, total));

	function handlePageSizeChange(value: string | undefined) {
		if (value !== undefined) {
			onPageSizeChange(Number(value));
		}
	}
</script>

<div class="flex items-center justify-between gap-4 pb-2">
	<span class="text-sm text-muted-foreground">
		Showing {showingStart}–{showingEnd} of {total}
		{itemLabel}
	</span>
	<div class="flex items-center gap-2">
		<span class="text-sm text-muted-foreground">Page size:</span>
		<ButtonGroup.Root>
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
			<Button
				size="sm"
				variant="outline"
				onclick={() => onPageChange(1)}
				disabled={page === 1}
				title="First page"
				aria-label="First page"
				class="size-9 cursor-pointer p-0"
			>
				<ChevronsLeft class="h-4 w-4" />
			</Button>
			<Button
				size="sm"
				variant="outline"
				onclick={() => onPageChange(page - 1)}
				disabled={page === 1}
				title="Previous page"
				aria-label="Previous page"
				class="size-9 cursor-pointer p-0"
			>
				<ChevronLeft class="h-4 w-4" />
			</Button>
		</ButtonGroup.Root>

		<span class="flex items-center px-3 py-1 text-sm">
			Page {page} of {totalPages}
		</span>
		<ButtonGroup.Root>
			<Button
				size="sm"
				variant="outline"
				onclick={() => onPageChange(page + 1)}
				disabled={page >= totalPages}
				title="Next page"
				aria-label="Next page"
				class="size-9 cursor-pointer p-0"
			>
				<ChevronRight class="h-4 w-4" />
			</Button>
			<Button
				size="sm"
				variant="outline"
				onclick={() => onPageChange(totalPages)}
				disabled={page >= totalPages}
				title="Last page"
				aria-label="Last page"
				class="size-9 cursor-pointer p-0"
			>
				<ChevronsRight class="h-4 w-4" />
			</Button>
		</ButtonGroup.Root>
	</div>
</div>
