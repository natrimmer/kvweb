<script lang="ts">
	import { api } from '$lib/api';
	import { Button } from '$lib/components/ui/button';
	import * as ButtonGroup from '$lib/components/ui/button-group';
	import { Textarea } from '$lib/components/ui/textarea';
	import LargeValueWarningDialog from '$lib/dialogs/LargeValueWarningDialog.svelte';
	import { formatShortcut } from '$lib/keyboard';
	import TypeHeader from '$lib/TypeHeader.svelte';
	import { highlightJson, isLargeValue, toastError } from '$lib/utils';
	import { Braces, Pencil, RemoveFormatting, View } from '@lucide/svelte/icons';
	import { toast } from 'svelte-sonner';

	interface Props {
		keyName: string;
		value: string;
		readOnly: boolean;
		typeHeaderExpanded: boolean;
		onDataChange: () => void;
	}

	let { keyName, value, readOnly, typeHeaderExpanded, onDataChange }: Props = $props();

	// Editor state
	let stringEditMode = $state(false); // false = view, true = edit
	let prettyPrint = $state(false);
	let editValue = $state('');
	let originalValue = $state('');
	let saving = $state(false);

	// Large value warning
	let largeValueWarningOpen = $state(false);
	let largeValueSize = $state(0);
	let pendingSaveValue: string | null = null;

	// Update editValue when prop changes
	$effect(() => {
		editValue = value;
		originalValue = value;
	});

	let hasChanges = $derived(editValue !== originalValue);

	function isJson(str: string): boolean {
		if (!str || str.length < 2) return false;
		const trimmed = str.trim();
		if (
			!(
				(trimmed.startsWith('{') && trimmed.endsWith('}')) ||
				(trimmed.startsWith('[') && trimmed.endsWith(']'))
			)
		) {
			return false;
		}
		try {
			JSON.parse(str);
			return true;
		} catch {
			return false;
		}
	}

	let isJsonValue = $derived(isJson(editValue));

	// Highlight string value when it changes or prettyPrint toggles
	let highlightedHtml = $derived.by(() => {
		if (isJsonValue) {
			return highlightJson(editValue, prettyPrint);
		}
		return '';
	});

	async function saveValue() {
		// Check if value is large and needs confirmation
		if (isLargeValue(editValue) && pendingSaveValue !== editValue) {
			largeValueSize = new Blob([editValue]).size;
			pendingSaveValue = editValue;
			largeValueWarningOpen = true;
			return;
		}

		// Proceed with save
		saving = true;
		try {
			await api.setKey(keyName, editValue, 0); // TTL handled by parent
			toast.success('Value saved');
			onDataChange();
		} catch (e) {
			toastError(e, 'Failed to save');
		} finally {
			saving = false;
			pendingSaveValue = null;
		}
	}

	function confirmLargeValueSave() {
		largeValueWarningOpen = false;
		saveValue();
	}

	function cancelLargeValueSave() {
		largeValueWarningOpen = false;
		pendingSaveValue = null;
	}
</script>

<div class="flex min-h-0 flex-1 flex-col">
	<TypeHeader expanded={typeHeaderExpanded}>
		<div class="flex items-center justify-between gap-2">
			<div class="flex-1"></div>
			{#if !readOnly && hasChanges}
				<Button
					size="sm"
					onclick={saveValue}
					disabled={saving}
					class="cursor-pointer"
					title={`Save changes (${formatShortcut('S', true)})`}
					aria-label="Save changes"
				>
					{saving ? 'Saving...' : 'Save'}
				</Button>
			{/if}
			{#if isJsonValue}
				<ButtonGroup.Root>
					<Button
						size="sm"
						variant="outline"
						onclick={() => (prettyPrint = false)}
						disabled={stringEditMode}
						class="cursor-pointer {!prettyPrint ? 'bg-accent' : ''}"
						title="Show compact JSON"
						aria-label="Show compact JSON"
					>
						<RemoveFormatting class="h-4 w-4" />
					</Button>
					<Button
						size="sm"
						variant="outline"
						onclick={() => (prettyPrint = true)}
						disabled={stringEditMode}
						class="cursor-pointer {prettyPrint ? 'bg-accent' : ''}"
						title="Show formatted JSON"
						aria-label="Show formatted JSON"
					>
						<Braces class="h-4 w-4" />
					</Button>
				</ButtonGroup.Root>
			{/if}
			{#if !readOnly}
				<ButtonGroup.Root>
					<Button
						size="sm"
						variant="outline"
						onclick={() => (stringEditMode = false)}
						class="cursor-pointer {!stringEditMode ? 'bg-accent' : ''}"
						title="View mode"
						aria-label="View mode"
					>
						<View class="h-4 w-4" />
					</Button>
					<Button
						size="sm"
						variant="outline"
						onclick={() => (stringEditMode = true)}
						class="cursor-pointer {stringEditMode ? 'bg-accent' : ''}"
						title="Edit mode"
						aria-label="Edit mode"
					>
						<Pencil class="h-4 w-4" />
					</Button>
				</ButtonGroup.Root>
			{/if}
		</div>
	</TypeHeader>

	<div class="-mx-6 min-h-0 flex-1 overflow-auto border-t border-border px-6 pt-6">
		{#if !stringEditMode && isJsonValue && highlightedHtml}
			<!-- View mode: JSON highlighted -->
			<div
				class="rounded border border-border bg-muted/50 [&>pre]:m-0 [&>pre]:min-h-full [&>pre]:p-4 [&>pre]:text-sm"
			>
				{@html highlightedHtml}
			</div>
		{:else if !stringEditMode}
			<!-- View mode: Plain text -->
			<div
				class="rounded border border-border bg-muted/50 p-4 font-mono text-sm break-all whitespace-pre-wrap"
			>
				{editValue}
			</div>
		{:else}
			<!-- Edit mode: Always editable textarea -->
			<Textarea
				id="value-textarea"
				bind:value={editValue}
				readonly={readOnly}
				title="Key value"
				aria-label="Key value"
				class="min-h-75 flex-1 resize-none text-sm"
			/>
		{/if}
	</div>
</div>

<LargeValueWarningDialog
	bind:open={largeValueWarningOpen}
	valueSize={largeValueSize}
	onConfirm={confirmLargeValueSave}
	onCancel={cancelLargeValueSave}
/>
