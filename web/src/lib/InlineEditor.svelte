<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import CheckIcon from '@lucide/svelte/icons/check';
	import XIcon from '@lucide/svelte/icons/x';

	interface Props {
		value: string;
		saving?: boolean;
		type?: 'text' | 'number';
		inputClass?: string;
		onSave: (value: string) => void;
		onCancel: () => void;
	}

	let {
		value = $bindable(),
		saving = false,
		type = 'text',
		inputClass = '',
		onSave,
		onCancel
	}: Props = $props();

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' && !e.shiftKey) {
			e.preventDefault();
			onSave(value);
		} else if (e.key === 'Escape') {
			onCancel();
		}
	}
</script>

<div class="flex items-center gap-2">
	<Input
		bind:value
		{type}
		step={type === 'number' ? 'any' : undefined}
		class="flex-1 font-mono text-sm {inputClass}"
		onkeydown={handleKeydown}
	/>
	<Button
		size="sm"
		onclick={() => onSave(value)}
		disabled={saving}
		class="cursor-pointer"
		title="Save"
	>
		<CheckIcon class="h-4 w-4" />
	</Button>
	<Button size="sm" variant="ghost" onclick={onCancel} class="cursor-pointer" title="Cancel">
		<XIcon class="h-4 w-4" />
	</Button>
</div>
