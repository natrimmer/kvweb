<script lang="ts">
	interface Props {
		value: string;
		jsonHtml?: string;
		class?: string;
	}

	let { value, jsonHtml, class: className }: Props = $props();
</script>

{#if jsonHtml}
	<div
		class="max-w-sm overflow-hidden text-ellipsis whitespace-nowrap [&>pre]:m-0 [&>pre]:overflow-hidden [&>pre]:bg-transparent [&>pre]:p-0 [&>pre]:text-sm [&>pre]:text-ellipsis [&>pre]:whitespace-nowrap {className ??
			''}"
		title={value}
	>
		{@html jsonHtml}
	</div>
{:else}
	<span
		class="block max-w-sm overflow-hidden text-ellipsis whitespace-nowrap {className ?? ''}"
		title={value}
	>
		{value}
	</span>
{/if}
