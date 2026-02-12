<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as ButtonGroup from '$lib/components/ui/button-group';
	import { ChevronsLeftRight, ChevronsRightLeft } from '@lucide/svelte/icons';

	interface Props {
		fullWidth: boolean;
		onToggle: (fullWidth: boolean) => void;
		disabled: boolean;
	}

	let { fullWidth, onToggle, disabled }: Props = $props();
</script>

<ButtonGroup.Root>
	<Button
		size="sm"
		variant="outline"
		class={fullWidth ? 'bg-accent' : ''}
		onclick={() => onToggle(true)}
		title="Full width"
		aria-label="Full width"
		{disabled}
	>
		<ChevronsLeftRight class="h-4 w-4" />
	</Button>
	<Button
		size="sm"
		variant="outline"
		class={!fullWidth ? 'bg-accent' : ''}
		onclick={() => onToggle(false)}
		title="Fit content"
		aria-label="Fit content"
		{disabled}
	>
		<ChevronsRightLeft class="h-4 w-4" />
	</Button>
</ButtonGroup.Root>
