<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as ButtonGroup from '$lib/components/ui/button-group';
	import EyeIcon from '@lucide/svelte/icons/eye';
	import EyeOffIcon from '@lucide/svelte/icons/eye-off';

	interface Props {
		showActions: boolean;
		onToggle: (show: boolean) => void;
		disabled: boolean;
	}

	let { showActions, onToggle, disabled }: Props = $props();
</script>

<ButtonGroup.Root>
	<Button
		size="sm"
		variant="outline"
		class={showActions ? 'bg-accent' : ''}
		onclick={() => onToggle(true)}
		title="Show actions"
		aria-label="Show actions"
		{disabled}
	>
		<EyeIcon class="h-4 w-4" />
	</Button>
	<Button
		size="sm"
		variant="outline"
		class={!showActions ? 'bg-accent' : ''}
		onclick={() => onToggle(false)}
		title="Hide actions"
		aria-label="Hide actions"
		{disabled}
	>
		<EyeOffIcon class="h-4 w-4" />
	</Button>
</ButtonGroup.Root>
