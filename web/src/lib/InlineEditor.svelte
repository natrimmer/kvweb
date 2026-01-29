<script lang="ts">
	import { Input } from '$lib/components/ui/input';

	interface Props {
		value: string;
		type?: 'text' | 'number';
		inputClass?: string;
		onSave: (value: string) => void;
		onCancel: () => void;
	}

	let { value = $bindable(), type = 'text', inputClass = '', onSave, onCancel }: Props = $props();

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' && !e.shiftKey) {
			e.preventDefault();
			onSave(value);
		} else if (e.key === 'Escape') {
			onCancel();
		}
	}
</script>

<Input
	bind:value
	{type}
	step={type === 'number' ? 'any' : undefined}
	title="Edit value"
	aria-label="Edit value"
	class="font-mono text-sm {inputClass}"
	onkeydown={handleKeydown}
/>
