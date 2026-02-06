<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as ButtonGroup from '$lib/components/ui/button-group';
	import ChevronsLeftRightIcon from '@lucide/svelte/icons/chevrons-left-right';
	import ChevronsRightLeftIcon from '@lucide/svelte/icons/chevrons-right-left';

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
		class="cursor-pointer {fullWidth ? 'bg-accent' : ''}"
		onclick={() => onToggle(true)}
		title="Full width"
		aria-label="Full width"
		{disabled}
	>
		<ChevronsLeftRightIcon class="h-4 w-4" />
	</Button>
	<Button
		size="sm"
		variant="outline"
		class="cursor-pointer {!fullWidth ? 'bg-accent' : ''}"
		onclick={() => onToggle(false)}
		title="Fit content"
		aria-label="Fit content"
		{disabled}
	>
		<ChevronsRightLeftIcon class="h-4 w-4" />
	</Button>
</ButtonGroup.Root>
