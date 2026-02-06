<script lang="ts">
	import AddItemForm from '$lib/AddItemForm.svelte';
	import { api, type HLLData } from '$lib/api';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import LargeValueWarningDialog from '$lib/dialogs/LargeValueWarningDialog.svelte';
	import TypeHeader from '$lib/TypeHeader.svelte';
	import { isLargeValue, toastError } from '$lib/utils';
	import { Plus } from '@lucide/svelte/icons';
	import { toast } from 'svelte-sonner';

	interface Props {
		keyName: string;
		data: HLLData;
		readOnly: boolean;
		typeHeaderExpanded: boolean;
		onDataChange: () => void;
	}

	let { keyName, data, readOnly, typeHeaderExpanded, onDataChange }: Props = $props();

	// Add form state
	let showAddForm = $state(false);
	let addElement = $state('');
	let adding = $state(false);

	// Large value warning
	let largeValueWarningOpen = $state(false);
	let largeValueSize = $state(0);
	let pendingAddElement: string | null = null;

	async function addItem() {
		if (!addElement.trim()) {
			toast.error('Element cannot be empty');
			return;
		}

		// Check if value is large and needs confirmation
		if (isLargeValue(addElement) && pendingAddElement !== addElement) {
			largeValueSize = new Blob([addElement]).size;
			pendingAddElement = addElement;
			largeValueWarningOpen = true;
			return;
		}

		adding = true;
		try {
			await api.hllAdd(keyName, addElement);
			toast.success('Element added');
			addElement = '';
			onDataChange();
		} catch (e) {
			toastError(e, 'Failed to add element');
		} finally {
			adding = false;
			pendingAddElement = null;
		}
	}

	function confirmLargeValue() {
		largeValueWarningOpen = false;
		if (pendingAddElement !== null) {
			addItem();
		}
	}

	function cancelLargeValue() {
		largeValueWarningOpen = false;
		pendingAddElement = null;
	}
</script>

<div class="flex min-h-0 flex-1 flex-col">
	<TypeHeader expanded={typeHeaderExpanded}>
		<div class="flex items-center justify-between">
			<div class="flex-1"></div>
			<div class="flex items-center gap-2">
				{#if !readOnly}
					<Button
						size="sm"
						variant="outline"
						onclick={() => (showAddForm = true)}
						class="cursor-pointer"
						title="Add element to HyperLogLog"
						aria-label="Add element to HyperLogLog"
					>
						<Plus class="mr-1 h-4 w-4" />
						Add Element
					</Button>
				{/if}
			</div>
		</div>

		{#if showAddForm}
			<AddItemForm {adding} onAdd={addItem} onClose={() => (showAddForm = false)}>
				<Input
					bind:value={addElement}
					placeholder="Element"
					class="flex-1"
					onkeydown={(e) => e.key === 'Enter' && addItem()}
					title="Element"
					aria-label="Element"
				/>
			</AddItemForm>
		{/if}

		<div class="mt-2 mb-4 rounded bg-muted p-4 text-sm text-muted-foreground">
			<p>
				HyperLogLog is a probabilistic data structure that estimates cardinality with ~0.81%
				standard error using only ~12KB of memory.
			</p>
			<p class="mt-1">Individual elements cannot be retrieved - only the count is available.</p>
		</div>
	</TypeHeader>

	<div class="-mx-6 min-h-0 flex-1 overflow-auto border-t border-border px-6 pt-6">
		<div class="flex flex-col items-center justify-center gap-4 py-8">
			<div class="text-6xl font-bold tabular-nums">{data.count.toLocaleString()}</div>
			<div class="text-sm text-muted-foreground">Estimated unique elements</div>
		</div>
	</div>
</div>

<LargeValueWarningDialog
	bind:open={largeValueWarningOpen}
	valueSize={largeValueSize}
	onConfirm={confirmLargeValue}
	onCancel={cancelLargeValue}
/>
