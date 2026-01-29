<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		expanded: boolean;
		children?: Snippet;
	}

	let { expanded, children }: Props = $props();
</script>

{#if expanded && children}
	<div class="flex flex-col gap-2 pt-2">
		{@render children()}
	</div>
{/if}
