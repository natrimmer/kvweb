<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		expanded: boolean;
		children?: Snippet;
	}

	let { expanded, children }: Props = $props();
</script>

{#if expanded && children}
	<div class="flex flex-col border-t border-dashed py-3">
		{@render children()}
	</div>
{/if}
