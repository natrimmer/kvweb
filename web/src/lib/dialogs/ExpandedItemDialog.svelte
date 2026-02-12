<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as ButtonGroup from '$lib/components/ui/button-group';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Textarea } from '$lib/components/ui/textarea';
	import LargeValueWarningDialog from '$lib/dialogs/LargeValueWarningDialog.svelte';
	import { highlightJson, isLargeValue, toastError } from '$lib/utils';
	import { Braces, Pencil, RemoveFormatting, View } from '@lucide/svelte/icons';
	import { toast } from 'svelte-sonner';

	interface Props {
		open: boolean;
		title: string;
		value: string;
		readOnly: boolean;
		onSave?: (value: string) => Promise<void>;
		onCancel: () => void;
	}

	let { open = $bindable(), title, value, readOnly, onSave, onCancel }: Props = $props();

	// Editor state
	let editMode = $state(false);
	let prettyPrint = $state(false);
	let editValue = $state('');
	let originalValue = $state('');
	let saving = $state(false);

	// Large value warning
	let largeValueWarningOpen = $state(false);
	let largeValueSize = $state(0);
	let pendingSaveValue: string | null = null;

	// Update editValue when value prop changes
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

	let highlightedHtml = $derived.by(() => {
		if (isJsonValue) {
			return highlightJson(editValue, prettyPrint);
		}
		return '';
	});

	let valueStats = $derived.by(() => {
		const chars = editValue.length;
		const bytes = new Blob([editValue]).size;
		const lines = editValue.split('\n').length;
		return { chars, bytes, lines };
	});

	async function handleSave() {
		if (!onSave) return;

		// Check if value is large and needs confirmation
		if (isLargeValue(editValue) && pendingSaveValue !== editValue) {
			largeValueSize = new Blob([editValue]).size;
			pendingSaveValue = editValue;
			largeValueWarningOpen = true;
			return;
		}

		saving = true;
		try {
			await onSave(editValue);
			toast.success('Value saved');
			originalValue = editValue;
			open = false;
		} catch (e) {
			toastError(e, 'Failed to save');
		} finally {
			saving = false;
			pendingSaveValue = null;
		}
	}

	function confirmLargeValueSave() {
		largeValueWarningOpen = false;
		handleSave();
	}

	function cancelLargeValueSave() {
		largeValueWarningOpen = false;
		pendingSaveValue = null;
	}

	function handleCancel() {
		editValue = originalValue;
		open = false;
		onCancel();
	}

	function formatBytes(bytes: number): string {
		if (bytes < 1024) return `${bytes} B`;
		if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
		return `${(bytes / (1024 * 1024)).toFixed(2)} MB`;
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="flex max-h-[85vh] max-w-4xl flex-col">
		<Dialog.Header>
			<Dialog.Title>{title}</Dialog.Title>
			<Dialog.Description>
				{valueStats.chars} characters · {formatBytes(valueStats.bytes)} · {valueStats.lines} lines
			</Dialog.Description>
		</Dialog.Header>

		<div class="mb-4 flex items-center justify-between gap-2">
			<div class="flex-1"></div>
			{#if isJsonValue}
				<ButtonGroup.Root>
					<Button
						size="sm"
						variant="outline"
						onclick={() => (prettyPrint = false)}
						disabled={editMode}
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
						disabled={editMode}
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
						onclick={() => (editMode = false)}
						class="cursor-pointer {!editMode ? 'bg-accent' : ''}"
						title="View mode"
						aria-label="View mode"
					>
						<View class="h-4 w-4" />
					</Button>
					<Button
						size="sm"
						variant="outline"
						onclick={() => (editMode = true)}
						class="cursor-pointer {editMode ? 'bg-accent' : ''}"
						title="Edit mode"
						aria-label="Edit mode"
					>
						<Pencil class="h-4 w-4" />
					</Button>
				</ButtonGroup.Root>
			{/if}
		</div>

		<div class="min-h-0 flex-1 overflow-auto">
			{#if !editMode && isJsonValue && highlightedHtml}
				<!-- View mode: JSON highlighted -->
				<div
					class="rounded border border-border bg-muted/50 [&>pre]:m-0 [&>pre]:min-h-full [&>pre]:p-4 [&>pre]:text-sm"
				>
					{@html highlightedHtml}
				</div>
			{:else if !editMode}
				<!-- View mode: Plain text -->
				<div
					class="rounded border border-border bg-muted/50 p-4 font-mono text-sm break-all whitespace-pre-wrap"
				>
					{editValue}
				</div>
			{:else}
				<!-- Edit mode: Editable textarea -->
				<Textarea
					bind:value={editValue}
					readonly={readOnly}
					title="Edit value"
					aria-label="Edit value"
					class="min-h-100 resize-none font-mono text-sm"
				/>
			{/if}
		</div>

		<Dialog.Footer>
			<Button variant="outline" onclick={handleCancel}>
				{readOnly || !hasChanges ? 'Close' : 'Cancel'}
			</Button>
			{#if !readOnly && onSave}
				<Button onclick={handleSave} disabled={saving || !hasChanges}>
					{saving ? 'Saving...' : 'Save'}
				</Button>
			{/if}
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<LargeValueWarningDialog
	bind:open={largeValueWarningOpen}
	valueSize={largeValueSize}
	onConfirm={confirmLargeValueSave}
	onCancel={cancelLargeValueSave}
/>
