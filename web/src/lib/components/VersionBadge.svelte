<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import AboutDialog from '$lib/dialogs/AboutDialog.svelte';

	interface Props {
		version: string;
		commit: string;
		dirty: boolean;
	}

	let { version, commit, dirty }: Props = $props();

	let aboutOpen = $state(false);

	let label = $derived.by(() => {
		if (!version) return '';
		let s = version;
		if (commit) s += ` ∘ ${commit}`;
		if (dirty) s += ' ∘ dirty';
		return s;
	});
</script>

{#if label}
	<button type="button" onclick={() => (aboutOpen = true)} class="cursor-pointer">
		<Badge variant="default" class="bg-primary/25 font-mono text-xs text-background">
			{label}
		</Badge>
	</button>
	<AboutDialog bind:open={aboutOpen} {version} {commit} {dirty} />
{/if}
