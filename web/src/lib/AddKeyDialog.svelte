<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as RadioGroup from '$lib/components/ui/radio-group';
	import * as Select from '$lib/components/ui/select';
	import { Textarea } from '$lib/components/ui/textarea';
	import { CirclePlus, Trash2 } from '@lucide/svelte/icons';
	import { toast } from 'svelte-sonner';
	import { api, type KeyType } from './api';
	import { formatTtl, isValidScore, toastError } from './utils';

	interface Props {
		open: boolean;
		prefix?: string;
		onCreated: (keyName: string) => void;
		onCancel: () => void;
	}

	let { open = $bindable(), prefix = '', onCreated, onCancel }: Props = $props();

	// Common state
	let keyName = $state('');
	let selectedType = $state<KeyType>('string');
	let ttl = $state(0);
	let creating = $state(false);

	// Type-specific state
	let stringValue = $state('');
	let listItems = $state<string[]>(['']);
	let listPosition = $state<'head' | 'tail'>('tail');
	let setMembers = $state<string[]>(['']);
	let hashFields = $state<{ field: string; value: string }[]>([{ field: '', value: '' }]);
	let zsetMembers = $state<{ member: string; score: string | number }[]>([
		{ member: '', score: '' }
	]);
	let streamFields = $state<{ key: string; value: string }[]>([{ key: '', value: '' }]);
	let hllElements = $state<string[]>(['']);

	// Validation errors
	let errors = $state<Record<string, string>>({});

	// Type options for selector
	const typeOptions = [
		{ value: 'string', label: 'String', description: 'Simple text value' },
		{ value: 'list', label: 'List', description: 'Ordered collection of strings' },
		{ value: 'set', label: 'Set', description: 'Unordered unique strings' },
		{ value: 'hash', label: 'Hash', description: 'Field-value pairs' },
		{ value: 'zset', label: 'Sorted Set', description: 'Scored unique strings' },
		{ value: 'stream', label: 'Stream', description: 'Append-only log of entries' },
		{ value: 'hyperloglog', label: 'HyperLogLog', description: 'Cardinality estimation' }
	];

	// Reset type-specific state when type changes
	$effect(() => {
		selectedType;
		stringValue = '';
		listItems = [''];
		listPosition = 'tail';
		setMembers = [''];
		hashFields = [{ field: '', value: '' }];
		zsetMembers = [{ member: '', score: '' }];
		streamFields = [{ key: '', value: '' }];
		hllElements = [''];
		errors = {};
	});

	function resetForm() {
		keyName = '';
		selectedType = 'string';
		ttl = 0;
		stringValue = '';
		listItems = [''];
		listPosition = 'tail';
		setMembers = [''];
		hashFields = [{ field: '', value: '' }];
		zsetMembers = [{ member: '', score: '' }];
		streamFields = [{ key: '', value: '' }];
		hllElements = [''];
		errors = {};
	}

	function handleCancel() {
		resetForm();
		open = false;
		onCancel();
	}

	// Dynamic field management
	function addListItem() {
		listItems = [...listItems, ''];
	}

	function removeListItem(index: number) {
		if (listItems.length > 1) {
			listItems = listItems.filter((_, i) => i !== index);
		}
	}

	function addSetMember() {
		setMembers = [...setMembers, ''];
	}

	function removeSetMember(index: number) {
		if (setMembers.length > 1) {
			setMembers = setMembers.filter((_, i) => i !== index);
		}
	}

	function addHashField() {
		hashFields = [...hashFields, { field: '', value: '' }];
	}

	function removeHashField(index: number) {
		if (hashFields.length > 1) {
			hashFields = hashFields.filter((_, i) => i !== index);
		}
	}

	function addZSetMember() {
		zsetMembers = [...zsetMembers, { member: '', score: '' }];
	}

	function removeZSetMember(index: number) {
		if (zsetMembers.length > 1) {
			zsetMembers = zsetMembers.filter((_, i) => i !== index);
		}
	}

	function addStreamField() {
		streamFields = [...streamFields, { key: '', value: '' }];
	}

	function removeStreamField(index: number) {
		if (streamFields.length > 1) {
			streamFields = streamFields.filter((_, i) => i !== index);
		}
	}

	function addHLLElement() {
		hllElements = [...hllElements, ''];
	}

	function removeHLLElement(index: number) {
		if (hllElements.length > 1) {
			hllElements = hllElements.filter((_, i) => i !== index);
		}
	}

	// Validation
	function validateInputs(): boolean {
		errors = {};

		if (!keyName.trim()) {
			errors.keyName = 'Key name is required';
		}

		if (ttl < 0) {
			errors.ttl = 'TTL must be 0 (no expiry) or positive';
		}

		switch (selectedType) {
			case 'string':
				// String can be empty
				break;

			case 'list': {
				const validItems = listItems.filter((item) => item.trim());
				if (validItems.length === 0) {
					errors.list = 'At least one list item is required';
				}
				break;
			}

			case 'set': {
				const validMembers = setMembers.filter((m) => m.trim());
				if (validMembers.length === 0) {
					errors.set = 'At least one set member is required';
				}
				// Check for duplicates
				if (new Set(validMembers).size !== validMembers.length) {
					errors.set = 'Set members must be unique';
				}
				break;
			}

			case 'hash': {
				const validFields = hashFields.filter((f) => f.field.trim());
				if (validFields.length === 0) {
					errors.hash = 'At least one hash field is required';
				}
				// Check for duplicate field names
				const fieldNames = validFields.map((f) => f.field);
				if (new Set(fieldNames).size !== fieldNames.length) {
					errors.hash = 'Hash field names must be unique';
				}
				break;
			}

			case 'zset': {
				const validMembers = zsetMembers.filter((m) => m.member.trim());
				if (validMembers.length === 0) {
					errors.zset = 'At least one sorted set member is required';
				}
				// Validate scores
				for (let i = 0; i < zsetMembers.length; i++) {
					if (zsetMembers[i].member.trim() && !isValidScore(zsetMembers[i].score)) {
						errors[`zset_${i}`] = 'Invalid score (must be a number)';
					}
				}
				// Check for duplicates
				const memberNames = validMembers.map((m) => m.member);
				if (new Set(memberNames).size !== memberNames.length) {
					errors.zset = 'Sorted set members must be unique';
				}
				break;
			}

			case 'stream': {
				const validFields = streamFields.filter((f) => f.key.trim());
				if (validFields.length === 0) {
					errors.stream = 'At least one stream field is required';
				}
				// Check for duplicate keys
				const streamKeys = validFields.map((f) => f.key);
				if (new Set(streamKeys).size !== streamKeys.length) {
					errors.stream = 'Stream field keys must be unique';
				}
				// Validate all values are non-empty
				for (let i = 0; i < streamFields.length; i++) {
					if (streamFields[i].key.trim() && !streamFields[i].value.trim()) {
						errors[`stream_${i}`] = 'Field value cannot be empty';
					}
				}
				break;
			}

			case 'hyperloglog': {
				const validElements = hllElements.filter((e) => e.trim());
				if (validElements.length === 0) {
					errors.hll = 'At least one element is required';
				}
				break;
			}
		}

		return Object.keys(errors).length === 0;
	}

	// Creation flow
	async function handleCreate() {
		if (!validateInputs()) return;

		creating = true;
		const fullKeyName = prefix + keyName;

		try {
			// Phase 1: Create key with first item
			switch (selectedType) {
				case 'string':
					await api.setKey(fullKeyName, stringValue, ttl);
					break;

				case 'list': {
					const validItems = listItems.filter((item) => item.trim());
					if (validItems.length > 0) {
						await api.listPush(fullKeyName, validItems[0], listPosition);
						if (ttl > 0) {
							// TTL will be set via setKey, but API doesn't support it for listPush
							// We need to set it separately
							await api.setKey(fullKeyName, '', ttl);
						}
					}
					break;
				}

				case 'set': {
					const validMembers = setMembers.filter((m) => m.trim());
					if (validMembers.length > 0) {
						await api.setAdd(fullKeyName, validMembers[0]);
						if (ttl > 0) {
							await api.setKey(fullKeyName, '', ttl);
						}
					}
					break;
				}

				case 'hash': {
					const validFields = hashFields.filter((f) => f.field.trim());
					if (validFields.length > 0) {
						await api.hashSet(fullKeyName, validFields[0].field, validFields[0].value);
						if (ttl > 0) {
							await api.setKey(fullKeyName, '', ttl);
						}
					}
					break;
				}

				case 'zset': {
					const validMembers = zsetMembers.filter((m) => m.member.trim());
					if (validMembers.length > 0) {
						const score =
							typeof validMembers[0].score === 'number'
								? validMembers[0].score
								: parseFloat(String(validMembers[0].score));
						await api.zsetAdd(fullKeyName, validMembers[0].member, score);
						if (ttl > 0) {
							await api.setKey(fullKeyName, '', ttl);
						}
					}
					break;
				}

				case 'stream': {
					const validFields = streamFields.filter((f) => f.key.trim());
					if (validFields.length > 0) {
						const fields: Record<string, string> = {};
						validFields.forEach((f) => {
							fields[f.key] = f.value;
						});
						await api.streamAdd(fullKeyName, fields);
						if (ttl > 0) {
							await api.setKey(fullKeyName, '', ttl);
						}
					}
					break;
				}

				case 'hyperloglog': {
					const validElements = hllElements.filter((e) => e.trim());
					if (validElements.length > 0) {
						await api.hllAdd(fullKeyName, validElements[0]);
						if (ttl > 0) {
							await api.setKey(fullKeyName, '', ttl);
						}
					}
					break;
				}
			}

			// Phase 2: Add remaining items
			await addRemainingItems(fullKeyName);

			// Success
			toast.success(`Key "${fullKeyName}" created`);
			resetForm();
			open = false;
			onCreated(fullKeyName);
		} catch (e) {
			toastError(e, 'Failed to create key');
		} finally {
			creating = false;
		}
	}

	async function addRemainingItems(fullKeyName: string) {
		const promises: Promise<void>[] = [];

		switch (selectedType) {
			case 'list': {
				const validItems = listItems.filter((item) => item.trim());
				for (let i = 1; i < validItems.length; i++) {
					promises.push(api.listPush(fullKeyName, validItems[i], listPosition));
				}
				break;
			}

			case 'set': {
				const validMembers = setMembers.filter((m) => m.trim());
				for (let i = 1; i < validMembers.length; i++) {
					promises.push(api.setAdd(fullKeyName, validMembers[i]));
				}
				break;
			}

			case 'hash': {
				const validFields = hashFields.filter((f) => f.field.trim());
				for (let i = 1; i < validFields.length; i++) {
					promises.push(api.hashSet(fullKeyName, validFields[i].field, validFields[i].value));
				}
				break;
			}

			case 'zset': {
				const validMembers = zsetMembers.filter((m) => m.member.trim());
				for (let i = 1; i < validMembers.length; i++) {
					const scoreValue = validMembers[i].score;
					const score =
						typeof scoreValue === 'number' ? scoreValue : parseFloat(String(scoreValue));
					promises.push(api.zsetAdd(fullKeyName, validMembers[i].member, score));
				}
				break;
			}

			case 'hyperloglog': {
				const validElements = hllElements.filter((e) => e.trim());
				for (let i = 1; i < validElements.length; i++) {
					promises.push(api.hllAdd(fullKeyName, validElements[i]));
				}
				break;
			}
		}

		if (promises.length > 0) {
			const results = await Promise.allSettled(promises);
			const failed = results.filter((r) => r.status === 'rejected').length;
			if (failed > 0) {
				toast.warning(`Key created, but ${failed} item(s) failed to add`, { duration: 5000 });
			}
		}
	}

	let canCreate = $derived(keyName.trim().length > 0 && !creating);
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="flex max-h-[85vh] max-w-2xl flex-col">
		<Dialog.Header>
			<Dialog.Title>Create New Key</Dialog.Title>
			<Dialog.Description>Configure the key name, type, and initial data.</Dialog.Description>
		</Dialog.Header>

		<div class="min-h-0 flex-1 space-y-4 overflow-y-auto px-1">
			<!-- Key Name -->
			<div class="space-y-2">
				<Label for="keyName">Key Name *</Label>
				<div class="flex items-center gap-2">
					{#if prefix}
						<Badge variant="secondary" class="shrink-0">{prefix}</Badge>
					{/if}
					<Input
						id="keyName"
						bind:value={keyName}
						placeholder="Enter key name"
						class={errors.keyName ? 'border-destructive' : ''}
						onkeydown={(e) => {
							if (e.key === 'Enter' && canCreate) {
								handleCreate();
							}
						}}
					/>
				</div>
				{#if errors.keyName}
					<p class="text-sm text-destructive">{errors.keyName}</p>
				{/if}
			</div>

			<!-- Type Selector -->
			<div class="space-y-2">
				<Label>Type *</Label>
				<Select.Root
					type="single"
					value={selectedType}
					onValueChange={(v: string) => {
						if (v) selectedType = v as KeyType;
					}}
				>
					<Select.Trigger class="w-full">
						{typeOptions.find((t) => t.value === selectedType)?.label ?? 'Select type'}
					</Select.Trigger>
					<Select.Content>
						{#each typeOptions as option}
							<Select.Item value={option.value}>
								<div>
									<div class="font-medium">{option.label}</div>
									<div class="text-xs text-muted-foreground">{option.description}</div>
								</div>
							</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			</div>

			<!-- TTL -->
			<div class="space-y-2">
				<Label for="ttl">TTL (seconds)</Label>
				<div class="flex items-center gap-2">
					<Input
						id="ttl"
						type="number"
						bind:value={ttl}
						placeholder="0 (no expiry)"
						min="0"
						class={`w-32 ${errors.ttl ? 'border-destructive' : ''}`}
					/>
					<span class="text-sm text-muted-foreground">
						{ttl > 0 ? formatTtl(ttl) : 'No expiry'}
					</span>
				</div>
				{#if errors.ttl}
					<p class="text-sm text-destructive">{errors.ttl}</p>
				{/if}
			</div>

			<!-- Type-Specific Forms -->
			{#if selectedType === 'string'}
				<div class="space-y-2">
					<Label for="stringValue">Initial Value (optional)</Label>
					<Textarea
						id="stringValue"
						bind:value={stringValue}
						placeholder="Enter initial value"
						rows={4}
					/>
				</div>
			{:else if selectedType === 'list'}
				<div class="space-y-2">
					<Label>Initial Items *</Label>
					<RadioGroup.Root bind:value={listPosition} class="mb-2 flex gap-4">
						<div class="flex items-center space-x-2">
							<RadioGroup.Item value="head" id="head" />
							<Label for="head">Push to Head (LPUSH)</Label>
						</div>
						<div class="flex items-center space-x-2">
							<RadioGroup.Item value="tail" id="tail" />
							<Label for="tail">Push to Tail (RPUSH)</Label>
						</div>
					</RadioGroup.Root>
					<div class="space-y-2">
						{#each listItems as item, i}
							<div class="flex gap-2">
								<Input bind:value={listItems[i]} placeholder={`Item ${i + 1}`} class="flex-1" />
								<Button
									variant="outline"
									size="icon"
									onclick={() => removeListItem(i)}
									disabled={listItems.length === 1}
									title="Remove item"
								>
									<Trash2 class="h-4 w-4" />
								</Button>
							</div>
						{/each}
					</div>
					<Button variant="outline" size="sm" onclick={addListItem}>
						<CirclePlus class="h-4 w-4" />
						Add Item
					</Button>
					{#if errors.list}
						<p class="text-sm text-destructive">{errors.list}</p>
					{/if}
				</div>
			{:else if selectedType === 'set'}
				<div class="space-y-2">
					<Label>Initial Members *</Label>
					<div class="space-y-2">
						{#each setMembers as member, i}
							<div class="flex gap-2">
								<Input bind:value={setMembers[i]} placeholder={`Member ${i + 1}`} class="flex-1" />
								<Button
									variant="outline"
									size="icon"
									onclick={() => removeSetMember(i)}
									disabled={setMembers.length === 1}
									title="Remove member"
								>
									<Trash2 class="h-4 w-4" />
								</Button>
							</div>
						{/each}
					</div>
					<Button variant="outline" size="sm" onclick={addSetMember}>
						<CirclePlus class="h-4 w-4" />
						Add Member
					</Button>
					{#if errors.set}
						<p class="text-sm text-destructive">{errors.set}</p>
					{/if}
				</div>
			{:else if selectedType === 'hash'}
				<div class="space-y-2">
					<Label>Initial Fields *</Label>
					<div class="space-y-2">
						{#each hashFields as field, i}
							<div class="flex gap-2">
								<Input bind:value={hashFields[i].field} placeholder="Field" class="flex-1" />
								<Input bind:value={hashFields[i].value} placeholder="Value" class="flex-1" />
								<Button
									variant="outline"
									size="icon"
									onclick={() => removeHashField(i)}
									disabled={hashFields.length === 1}
									title="Remove field"
								>
									<Trash2 class="h-4 w-4" />
								</Button>
							</div>
						{/each}
					</div>
					<Button variant="outline" size="sm" onclick={addHashField}>
						<CirclePlus class="h-4 w-4" />
						Add Field
					</Button>
					{#if errors.hash}
						<p class="text-sm text-destructive">{errors.hash}</p>
					{/if}
				</div>
			{:else if selectedType === 'zset'}
				<div class="space-y-2">
					<Label>Initial Members *</Label>
					<div class="space-y-2">
						{#each zsetMembers as member, i}
							<div class="flex gap-2">
								<Input bind:value={zsetMembers[i].member} placeholder="Member" class="flex-1" />
								<Input
									bind:value={zsetMembers[i].score}
									type="number"
									step="any"
									placeholder="Score"
									class="w-32"
								/>
								<Button
									variant="outline"
									size="icon"
									onclick={() => removeZSetMember(i)}
									disabled={zsetMembers.length === 1}
									title="Remove member"
								>
									<Trash2 class="h-4 w-4" />
								</Button>
							</div>
							{#if errors[`zset_${i}`]}
								<p class="text-sm text-destructive">{errors[`zset_${i}`]}</p>
							{/if}
						{/each}
					</div>
					<Button variant="outline" size="sm" onclick={addZSetMember}>
						<CirclePlus class="h-4 w-4" />
						Add Member
					</Button>
					{#if errors.zset}
						<p class="text-sm text-destructive">{errors.zset}</p>
					{/if}
				</div>
			{:else if selectedType === 'stream'}
				<div class="space-y-2">
					<Label>Initial Entry Fields *</Label>
					<div class="space-y-2">
						{#each streamFields as field, i}
							<div class="flex gap-2">
								<Input bind:value={streamFields[i].key} placeholder="Field" class="flex-1" />
								<Input bind:value={streamFields[i].value} placeholder="Value" class="flex-1" />
								<Button
									variant="outline"
									size="icon"
									onclick={() => removeStreamField(i)}
									disabled={streamFields.length === 1}
									title="Remove field"
								>
									<Trash2 class="h-4 w-4" />
								</Button>
							</div>
							{#if errors[`stream_${i}`]}
								<p class="text-sm text-destructive">{errors[`stream_${i}`]}</p>
							{/if}
						{/each}
					</div>
					<Button variant="outline" size="sm" onclick={addStreamField}>
						<CirclePlus class="h-4 w-4" />
						Add Field
					</Button>
					{#if errors.stream}
						<p class="text-sm text-destructive">{errors.stream}</p>
					{/if}
				</div>
			{:else if selectedType === 'hyperloglog'}
				<div class="space-y-2">
					<Label>Initial Elements *</Label>
					<div class="space-y-2">
						{#each hllElements as element, i}
							<div class="flex gap-2">
								<Input
									bind:value={hllElements[i]}
									placeholder={`Element ${i + 1}`}
									class="flex-1"
								/>
								<Button
									variant="outline"
									size="icon"
									onclick={() => removeHLLElement(i)}
									disabled={hllElements.length === 1}
									title="Remove element"
								>
									<Trash2 class="h-4 w-4" />
								</Button>
							</div>
						{/each}
					</div>
					<Button variant="outline" size="sm" onclick={addHLLElement}>
						<CirclePlus class="h-4 w-4" />
						Add Element
					</Button>
					{#if errors.hll}
						<p class="text-sm text-destructive">{errors.hll}</p>
					{/if}
				</div>
			{/if}
		</div>

		<Dialog.Footer>
			<Button variant="outline" onclick={handleCancel}>Cancel</Button>
			<Button onclick={handleCreate} disabled={!canCreate}>
				{creating ? 'Creating...' : 'Create Key'}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
