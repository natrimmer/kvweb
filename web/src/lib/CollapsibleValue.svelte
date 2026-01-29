<script lang="ts">
	interface Props {
		value: string;
		maxLength?: number;
		highlight?: string; // Pre-rendered HTML for JSON highlighting
	}

	let { value, maxLength = 200, highlight }: Props = $props();

	let expanded = $state(false);

	let needsCollapse = $derived(value.length > maxLength);
	let displayValue = $derived(needsCollapse && !expanded ? value.slice(0, maxLength) : value);
</script>

{#if highlight}
	{#if expanded}
		<div class="[&>pre]:m-0 [&>pre]:bg-transparent [&>pre]:p-0 [&>pre]:text-sm">
			{@html highlight}
		</div>
	{:else}
		<div
			class="[&>pre]:m-0 [&>pre]:overflow-hidden [&>pre]:bg-transparent [&>pre]:p-0 [&>pre]:text-sm [&>pre]:text-ellipsis [&>pre]:whitespace-nowrap"
		>
			{@html highlight}
		</div>
	{/if}
{:else}
	<span class="break-all"
		>{displayValue}{#if needsCollapse && !expanded}…{/if}</span
	>
{/if}

{#if needsCollapse}
	<button
		type="button"
		onclick={() => (expanded = !expanded)}
		class="ml-1 cursor-pointer text-xs text-primary hover:text-primary/80 hover:underline"
		title={expanded ? 'Collapse value' : `Expand to show full value (${value.length} characters)`}
		aria-label={expanded
			? 'Collapse value'
			: `Expand to show full value (${value.length} characters)`}
	>
		{expanded ? 'Show less' : `Show all (${value.length} chars)`}
	</button>
{/if}
