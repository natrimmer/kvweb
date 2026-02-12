<script lang="ts">
	import AddItemForm from '$lib/AddItemForm.svelte';
	import { api, type GeoMember, type PaginationInfo, type ZSetMember } from '$lib/api';
	import ActionsToggle from '$lib/components/ActionsToggle.svelte';
	import TableWidthToggle from '$lib/components/TableWidthToggle.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as ButtonGroup from '$lib/components/ui/button-group';
	import { Input } from '$lib/components/ui/input';
	import * as Table from '$lib/components/ui/table';
	import DeleteItemDialog from '$lib/dialogs/DeleteItemDialog.svelte';
	import ExpandedItemDialog from '$lib/dialogs/ExpandedItemDialog.svelte';
	import LargeValueWarningDialog from '$lib/dialogs/LargeValueWarningDialog.svelte';
	import InlineEditor from '$lib/InlineEditor.svelte';
	import ItemActions from '$lib/ItemActions.svelte';
	import PaginationControls from '$lib/PaginationControls.svelte';
	import TypeHeader from '$lib/TypeHeader.svelte';
	import {
		formatCoordinate,
		highlightJson,
		isLargeValue,
		isValidLatitude,
		isValidLongitude,
		isValidScore,
		parseScore,
		showPaginationControls,
		toastError
	} from '$lib/utils';
	import {
		Braces,
		ChevronsLeftRight,
		Map,
		Minus,
		Plus,
		RemoveFormatting,
		TableIcon
	} from '@lucide/svelte/icons';
	import { toast } from 'svelte-sonner';
	import GeoMapView from './GeoMapView.svelte';

	interface Props {
		keyName: string;
		members: ZSetMember[];
		pagination: PaginationInfo | undefined;
		currentPage: number;
		pageSize: number;
		readOnly: boolean;
		typeHeaderExpanded: boolean;
		onPageChange: (page: number) => void;
		onPageSizeChange: (size: number) => void;
		onDataChange: () => void;
		getCopyValue?: () => string;
		geoViewActive?: boolean;
	}

	let {
		keyName,
		members,
		pagination,
		currentPage,
		pageSize,
		readOnly,
		typeHeaderExpanded,
		onPageChange,
		onPageSizeChange,
		onDataChange,
		getCopyValue = $bindable(),
		geoViewActive = $bindable(false)
	}: Props = $props();

	// View state
	let rawView = $state(false);
	let fullWidth = $state(false);
	let showActions = $state(true);
	let prettyPrint = $state(false);
	let viewMode = $state<'zset' | 'geo'>('zset');
	let geoMembers = $state<GeoMember[]>([]);
	let geoDisplayMode = $state<'table' | 'map' | 'json'>('table');

	// Add form state
	let showAddForm = $state(false);
	let addMember = $state('');
	let addScore = $state<string | number>('');
	let addLongitude = $state<string | number>('');
	let addLatitude = $state<string | number>('');
	let adding = $state(false);

	// Edit state
	let editMode = $state<'none' | 'score' | 'member' | 'longitude' | 'latitude' | 'coordinates'>(
		'none'
	);
	let editingMember = $state<string | null>(null);
	let editingValue = $state('');
	let editingLongitude = $state('');
	let editingLatitude = $state('');
	let saving = $state(false);

	// Delete state
	let deleteDialogOpen = $state(false);
	let deleteTarget = $state<{ member: string; display: string } | null>(null);

	// Large value warning
	let largeValueWarningOpen = $state(false);
	let largeValueSize = $state(0);
	let pendingAddMember: string | null = null;
	let pendingEditMember: { old: string; new: string } | null = null;

	// Expanded view state
	let expandedDialogOpen = $state(false);
	let expandedMember = $state<string>('');
	let expandedScore = $state<number>(0);
	let expandedLongitude = $state<number>(0);
	let expandedLatitude = $state<number>(0);

	let rawJsonHtml = $derived(rawView ? highlightJson(JSON.stringify(members, null, 2), true) : '');
	let geoJsonHtml = $derived(
		geoDisplayMode === 'json' ? highlightJson(JSON.stringify(geoMembers, null, 2), true) : ''
	);

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

	// JSON highlighting for zset members
	let memberHighlights = $derived.by(() => {
		const highlights: Record<string, string> = {};
		for (const { member } of members) {
			if (isJson(member)) {
				highlights[member] = highlightJson(member, prettyPrint);
			}
		}
		return highlights;
	});

	let hasAnyJson = $derived(members.some(({ member }) => isJson(member)));

	// Reload geo data when key changes or when members change (if in geo mode)
	let previousKeyName: string | null = null;
	let previousMembersLength = 0;
	$effect(() => {
		const keyChanged = previousKeyName !== null && keyName !== previousKeyName;
		const membersChanged = members.length !== previousMembersLength;

		if (viewMode === 'geo' && (keyChanged || membersChanged)) {
			loadGeoData();
		}

		previousKeyName = keyName;
		previousMembersLength = members.length;
	});

	// Provide copy value function based on view mode
	$effect(() => {
		getCopyValue = () => {
			if (viewMode === 'geo' && geoMembers.length > 0) {
				return JSON.stringify(geoMembers, null, 2);
			}
			return JSON.stringify(members, null, 2);
		};
	});

	async function loadGeoData() {
		try {
			const result = await api.geoGet(keyName, currentPage, pageSize);
			geoMembers = result.value as GeoMember[];
		} catch (e) {
			toastError(e, 'Failed to load geo data');
			viewMode = 'zset';
		}
	}

	function toggleGeoView() {
		if (viewMode === 'zset') {
			viewMode = 'geo';
			geoViewActive = true;
			loadGeoData();
		} else {
			viewMode = 'zset';
			geoViewActive = false;
		}
		rawView = false;
	}

	function startEditingScore(member: string, score: number) {
		editMode = 'score';
		editingMember = member;
		editingValue = String(score);
	}

	function startRenamingMember(member: string) {
		editMode = 'member';
		editingMember = member;
		editingValue = member;
	}

	function startEditingCoordinates(member: string, longitude: number, latitude: number) {
		editMode = 'coordinates';
		editingMember = member;
		editingLongitude = String(longitude);
		editingLatitude = String(latitude);
	}

	function cancelEditing() {
		editMode = 'none';
		editingMember = null;
		editingValue = '';
		editingLongitude = '';
		editingLatitude = '';
	}

	async function saveEdit(value: string) {
		if (editingMember === null) return;

		saving = true;
		try {
			if (editMode === 'score') {
				// Edit score
				if (!isValidScore(value)) {
					toast.error('Invalid score value');
					return;
				}
				await api.zsetAdd(keyName, editingMember, parseScore(value));
				toast.success('Score updated');
			} else if (editMode === 'member') {
				// Rename member
				if (!value.trim()) {
					toast.error('Member name cannot be empty');
					return;
				}
				if (value === editingMember) {
					// No change, just cancel
					cancelEditing();
					return;
				}

				// Check if value is large and needs confirmation
				if (
					isLargeValue(value) &&
					(pendingEditMember === null || pendingEditMember.new !== value)
				) {
					largeValueSize = new Blob([value]).size;
					pendingEditMember = { old: editingMember, new: value };
					largeValueWarningOpen = true;
					return;
				}

				await api.zsetRename(keyName, editingMember, value.trim());
				toast.success('Member renamed');
			} else if (editMode === 'coordinates') {
				// Edit both geo coordinates
				const lon = parseFloat(editingLongitude);
				const lat = parseFloat(editingLatitude);

				if (isNaN(lon) || isNaN(lat)) {
					toast.error('Invalid coordinate values');
					return;
				}
				if (!isValidLongitude(lon)) {
					toast.error('Longitude must be between -180 and 180');
					return;
				}
				if (!isValidLatitude(lat)) {
					toast.error('Latitude must be between -85.05 and 85.05');
					return;
				}

				await api.geoAdd(keyName, editingMember, lon, lat);
				toast.success('Coordinates updated');
				loadGeoData();
			}
			cancelEditing();
			onDataChange();
		} catch (e) {
			const errorMsg =
				editMode === 'score'
					? 'Failed to update score'
					: editMode === 'coordinates'
						? 'Failed to update coordinates'
						: 'Failed to rename member';
			toastError(e, errorMsg);
		} finally {
			saving = false;
			pendingEditMember = null;
		}
	}

	async function addItem() {
		if (!addMember.trim()) {
			toast.error('Member cannot be empty');
			return;
		}
		if (!isValidScore(addScore)) {
			toast.error('Invalid score value');
			return;
		}

		// Check if value is large and needs confirmation
		if (isLargeValue(addMember) && pendingAddMember !== addMember) {
			largeValueSize = new Blob([addMember]).size;
			pendingAddMember = addMember;
			largeValueWarningOpen = true;
			return;
		}

		adding = true;
		try {
			await api.zsetAdd(keyName, addMember, parseScore(addScore));
			toast.success('Member added');
			addMember = '';
			addScore = '';
			onDataChange();
		} catch (e) {
			toastError(e, 'Failed to add member');
		} finally {
			adding = false;
			pendingAddMember = null;
		}
	}

	async function addGeoItem() {
		if (!addMember.trim()) {
			toast.error('Member cannot be empty');
			return;
		}
		if (!isValidLongitude(addLongitude)) {
			toast.error('Longitude must be between -180 and 180');
			return;
		}
		if (!isValidLatitude(addLatitude)) {
			toast.error('Latitude must be between -85.05 and 85.05');
			return;
		}
		adding = true;
		try {
			const lon =
				typeof addLongitude === 'number' ? addLongitude : parseFloat(String(addLongitude));
			const lat = typeof addLatitude === 'number' ? addLatitude : parseFloat(String(addLatitude));
			await api.geoAdd(keyName, addMember, lon, lat);
			toast.success('Location added');
			addMember = '';
			addLongitude = '';
			addLatitude = '';
			onDataChange();
			loadGeoData();
		} catch (e) {
			toastError(e, 'Failed to add location');
		} finally {
			adding = false;
		}
	}

	function confirmLargeValue() {
		largeValueWarningOpen = false;
		if (pendingAddMember !== null) {
			addItem();
		} else if (pendingEditMember !== null) {
			saveEdit(pendingEditMember.new);
		}
	}

	function cancelLargeValue() {
		largeValueWarningOpen = false;
		pendingAddMember = null;
		pendingEditMember = null;
	}

	function openDeleteDialog(member: string) {
		deleteTarget = {
			member,
			display: member.length > 50 ? member.slice(0, 50) + '...' : member
		};
		deleteDialogOpen = true;
	}

	async function confirmDelete() {
		if (!deleteTarget) return;
		try {
			await api.zsetRemove(keyName, deleteTarget.member);
			toast.success(viewMode === 'geo' ? 'Location deleted' : 'Member deleted');
			onDataChange();
			if (viewMode === 'geo') {
				loadGeoData();
			}
		} catch (e) {
			toastError(e, viewMode === 'geo' ? 'Failed to delete location' : 'Failed to delete member');
		} finally {
			deleteDialogOpen = false;
			deleteTarget = null;
		}
	}

	async function incrementScore(member: string, amount: number) {
		saving = true;
		try {
			const result = await api.zsetIncrScore(keyName, member, amount);
			toast.success(`Score ${amount > 0 ? 'incremented' : 'decremented'}`);
			onDataChange();
		} catch (e) {
			toastError(e, 'Failed to modify score');
		} finally {
			saving = false;
		}
	}

	function openExpandedView(member: string, score: number, longitude?: number, latitude?: number) {
		expandedMember = member;
		expandedScore = score;
		expandedLongitude = longitude ?? 0;
		expandedLatitude = latitude ?? 0;
		expandedDialogOpen = true;
	}

	async function saveExpandedEdit(newValue: string) {
		if (!expandedMember) return;
		if (!newValue.trim()) {
			throw new Error('Member cannot be empty');
		}
		if (newValue === expandedMember) {
			return;
		}
		await api.zsetRename(keyName, expandedMember, newValue.trim());
		onDataChange();
	}

	function closeExpandedView() {
		expandedDialogOpen = false;
		expandedMember = '';
		expandedScore = 0;
		expandedLongitude = 0;
		expandedLatitude = 0;
	}
</script>

<div class="flex min-h-0 flex-1 flex-col">
	<TypeHeader expanded={typeHeaderExpanded}>
		<div class="flex items-center justify-between">
			<div class="flex-1">
				{#if pagination && !showPaginationControls(pagination.total)}
					<span class="text-sm text-muted-foreground">
						{pagination.total} member{pagination.total === 1 ? '' : 's'} total
					</span>
				{/if}
			</div>
			<div class="flex items-center gap-2">
				{#if !readOnly}
					<Button
						size="sm"
						variant="outline"
						onclick={() => (showAddForm = true)}
						title={viewMode === 'geo' ? 'Add location to geo index' : 'Add member to sorted set'}
						aria-label={viewMode === 'geo'
							? 'Add location to geo index'
							: 'Add member to sorted set'}
					>
						<Plus class="mr-1 h-4 w-4" />
						{viewMode === 'geo' ? 'Add Location' : 'Add Member'}
					</Button>
				{/if}
				<Button
					size="sm"
					variant="outline"
					onclick={toggleGeoView}
					title={viewMode === 'zset' ? 'View as geo coordinates' : 'View as sorted set scores'}
					aria-label={viewMode === 'zset' ? 'View as geo coordinates' : 'View as sorted set scores'}
				>
					{viewMode === 'zset' ? 'View as Geo' : 'View as ZSet'}
				</Button>
				{#if hasAnyJson}
					<ButtonGroup.Root>
						<Button
							size="sm"
							variant="outline"
							onclick={() => (prettyPrint = false)}
							disabled={rawView}
							class={!prettyPrint ? 'bg-accent' : ''}
							title="Compact JSON"
							aria-label="Compact JSON"
						>
							<RemoveFormatting class="h-4 w-4" />
						</Button>
						<Button
							size="sm"
							variant="outline"
							onclick={() => (prettyPrint = true)}
							disabled={rawView}
							class={prettyPrint ? 'bg-accent' : ''}
							title="Format JSON"
							aria-label="Format JSON"
						>
							<Braces class="h-4 w-4" />
						</Button>
					</ButtonGroup.Root>
				{/if}
				<ButtonGroup.Root>
					<Button
						size="sm"
						variant="outline"
						onclick={() => {
							if (viewMode === 'geo') {
								geoDisplayMode = 'map';
							}
						}}
						disabled={viewMode === 'zset'}
						class={viewMode === 'geo' && geoDisplayMode === 'map' ? 'bg-accent' : ''}
						title="Show on Map"
						aria-label="Show on Map"
					>
						<Map class="h-4 w-4" />
					</Button>
					<Button
						size="sm"
						variant="outline"
						onclick={() => {
							if (viewMode === 'geo') {
								geoDisplayMode = 'table';
							} else {
								rawView = false;
							}
						}}
						class={(viewMode === 'zset' && !rawView) ||
						(viewMode === 'geo' && geoDisplayMode === 'table')
							? 'bg-accent'
							: ''}
						title="Show as Table"
						aria-label="Show as Table"
					>
						<TableIcon class="h-4 w-4" />
					</Button>
					<Button
						size="sm"
						variant="outline"
						onclick={() => {
							if (viewMode === 'zset') {
								rawView = true;
							} else {
								geoDisplayMode = 'json';
							}
						}}
						class={(viewMode === 'zset' && rawView) ||
						(viewMode === 'geo' && geoDisplayMode === 'json')
							? 'bg-accent'
							: ''}
						title="Show as Raw JSON"
						aria-label="Show as Raw JSON"
					>
						{'{ }'}
					</Button>
				</ButtonGroup.Root>
				<TableWidthToggle
					{fullWidth}
					onToggle={(fw) => (fullWidth = fw)}
					disabled={rawView || (viewMode === 'geo' && geoDisplayMode !== 'table')}
				/>
				{#if !readOnly}
					<ActionsToggle
						{showActions}
						onToggle={(sa) => (showActions = sa)}
						disabled={rawView || (viewMode === 'geo' && geoDisplayMode !== 'table')}
					/>
				{/if}
			</div>
		</div>

		{#if showAddForm}
			<div class="pt-3">
				<AddItemForm
					{adding}
					onAdd={viewMode === 'geo' ? addGeoItem : addItem}
					onClose={() => (showAddForm = false)}
				>
					<Input
						bind:value={addMember}
						placeholder="Member"
						class="flex-1"
						onkeydown={(e) => e.key === 'Enter' && (viewMode === 'geo' ? addGeoItem() : addItem())}
						title="Member"
						aria-label="Member"
					/>
					{#if viewMode === 'geo'}
						<Input
							bind:value={addLongitude}
							placeholder="Longitude"
							type="number"
							step="any"
							class="w-28"
							onkeydown={(e) => e.key === 'Enter' && addGeoItem()}
							title="Longitude (-180 to 180)"
							aria-label="Longitude"
						/>
						<Input
							bind:value={addLatitude}
							placeholder="Latitude"
							type="number"
							step="any"
							class="w-28"
							onkeydown={(e) => e.key === 'Enter' && addGeoItem()}
							title="Latitude (-85.05 to 85.05)"
							aria-label="Latitude"
						/>
					{:else}
						<Input
							bind:value={addScore}
							placeholder="Score"
							type="number"
							step="any"
							class="w-32"
							onkeydown={(e) => e.key === 'Enter' && addItem()}
							title="Score"
							aria-label="Score"
						/>
					{/if}
				</AddItemForm>
			</div>
		{/if}

		{#if pagination && showPaginationControls(pagination.total)}
			<div class="pt-3">
				<PaginationControls
					page={currentPage}
					{pageSize}
					total={pagination.total}
					itemLabel="members"
					{onPageChange}
					{onPageSizeChange}
				/>
			</div>
		{/if}
	</TypeHeader>

	<div class="-mx-6 min-h-0 flex-1 overflow-auto border-t border-border px-6 pt-6">
		{#if rawView && rawJsonHtml && viewMode === 'zset'}
			<div
				class="rounded border border-border [&>pre]:m-0 [&>pre]:min-h-full [&>pre]:p-4 [&>pre]:text-sm"
			>
				{@html rawJsonHtml}
			</div>
		{:else if viewMode === 'geo'}
			{#if geoDisplayMode === 'map'}
				<GeoMapView members={geoMembers} {keyName} />
			{:else if geoDisplayMode === 'json' && geoJsonHtml}
				<div
					class="rounded border border-border [&>pre]:m-0 [&>pre]:min-h-full [&>pre]:p-4 [&>pre]:text-sm"
				>
					{@html geoJsonHtml}
				</div>
			{:else}
				<div class={fullWidth ? '' : 'max-w-max'}>
					<Table.Root class="table-auto">
						<Table.Header>
							<Table.Row>
								<Table.Head class="w-8"></Table.Head>
								<Table.Head class="w-auto">Member</Table.Head>
								<Table.Head class="w-36">Longitude</Table.Head>
								<Table.Head class="w-36">Latitude</Table.Head>
								{#if !readOnly && showActions}
									<Table.Head class="w-24">Actions</Table.Head>
								{/if}
							</Table.Row>
						</Table.Header>
						<Table.Body>
							{#each geoMembers as { member, longitude, latitude }}
								<Table.Row>
									<Table.Cell class="align-top">
										<Button
											size="sm"
											variant="outline"
											onclick={() => openExpandedView(member, 0, longitude, latitude)}
											class="h-6 w-6 shrink-0 p-0"
											title="Expand to full view"
											aria-label="Expand to full view"
										>
											<ChevronsLeftRight class="h-3 w-3" />
										</Button>
									</Table.Cell>
									<Table.Cell class="font-mono">
										{#if editMode === 'member' && editingMember === member}
											<InlineEditor
												bind:value={editingValue}
												type="text"
												inputClass="w-full"
												onSave={saveEdit}
												onCancel={cancelEditing}
											/>
										{:else}
											<div class="flex items-center gap-1">
												{#if memberHighlights[member]}
													<!-- JSON value with highlighting -->
													<div
														class="[&>pre]:m-0 [&>pre]:overflow-hidden [&>pre]:bg-transparent [&>pre]:p-0 [&>pre]:text-sm [&>pre]:text-ellipsis [&>pre]:whitespace-nowrap"
													>
														{@html memberHighlights[member]}
													</div>
												{:else}
													<!-- Plain text value -->
													<span class="break-all">
														{member.length > 100 ? member.slice(0, 100) + '…' : member}
													</span>
												{/if}
											</div>
										{/if}
									</Table.Cell>
									<Table.Cell class="font-mono text-muted-foreground">
										{#if editMode === 'coordinates' && editingMember === member}
											<InlineEditor
												bind:value={editingLongitude}
												type="number"
												inputClass="w-32"
												onSave={saveEdit}
												onCancel={cancelEditing}
											/>
										{:else}
											{formatCoordinate(longitude)}
										{/if}
									</Table.Cell>
									<Table.Cell class="font-mono text-muted-foreground">
										{#if editMode === 'coordinates' && editingMember === member}
											<InlineEditor
												bind:value={editingLatitude}
												type="number"
												inputClass="w-32"
												onSave={saveEdit}
												onCancel={cancelEditing}
											/>
										{:else}
											{formatCoordinate(latitude)}
										{/if}
									</Table.Cell>
									{#if !readOnly && showActions}
										<Table.Cell class="align-top">
											<ItemActions
												editing={editMode !== 'none' && editingMember === member}
												{saving}
												showRename={true}
												editLabel="Edit coordinates"
												renameLabel="Rename location"
												onEdit={() => startEditingCoordinates(member, longitude, latitude)}
												onRename={() => startRenamingMember(member)}
												onSave={() => saveEdit(editingValue)}
												onCancel={cancelEditing}
												onDelete={() => openDeleteDialog(member)}
											/>
										</Table.Cell>
									{/if}
								</Table.Row>
							{/each}
						</Table.Body>
					</Table.Root>
				</div>
			{/if}
		{:else}
			<div class={fullWidth ? '' : 'max-w-max'}>
				<Table.Root class="table-auto">
					<Table.Header>
						<Table.Row>
							<Table.Head class="w-8"></Table.Head>
							<Table.Head class="w-auto">Member</Table.Head>
							<Table.Head class="w-32">Score</Table.Head>
							{#if !readOnly && showActions}
								<Table.Head class="w-24">Actions</Table.Head>
							{/if}
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each members as { member, score }}
							<Table.Row>
								<Table.Cell>
									<Button
										size="sm"
										variant="outline"
										onclick={() => openExpandedView(member, score)}
										class="size-6"
										title="Expand to full view"
										aria-label="Expand to full view"
									>
										<ChevronsLeftRight class="h-3 w-3" />
									</Button>
								</Table.Cell>
								<Table.Cell class="font-mono">
									{#if editMode === 'member' && editingMember === member}
										<InlineEditor
											bind:value={editingValue}
											type="text"
											inputClass="w-full"
											onSave={saveEdit}
											onCancel={cancelEditing}
										/>
									{:else}
										<div class="flex items-center gap-1">
											{#if memberHighlights[member]}
												<!-- JSON value with highlighting -->
												<div
													class="[&>pre]:m-0 [&>pre]:overflow-hidden [&>pre]:bg-transparent [&>pre]:p-0 [&>pre]:text-sm [&>pre]:text-ellipsis [&>pre]:whitespace-nowrap"
												>
													{@html memberHighlights[member]}
												</div>
											{:else}
												<!-- Plain text value -->
												<span class="break-all">
													{member.length > 100 ? member.slice(0, 100) + '…' : member}
												</span>
											{/if}
										</div>
									{/if}
								</Table.Cell>
								<Table.Cell class="font-mono text-muted-foreground">
									{#if editMode === 'score' && editingMember === member}
										<InlineEditor
											bind:value={editingValue}
											type="number"
											inputClass="w-24"
											onSave={saveEdit}
											onCancel={cancelEditing}
										/>
									{:else}
										<div class="flex items-center gap-1">
											{#if !readOnly && showActions && viewMode === 'zset'}
												<Button
													size="sm"
													variant="ghost"
													onclick={() => incrementScore(member, -1)}
													disabled={saving}
													class="h-6 w-6 p-0"
													title="Decrement score by 1"
													aria-label="Decrement score by 1"
												>
													<Minus class="h-3 w-3" />
												</Button>
											{:else}
												<div class="h-6 w-6"></div>
											{/if}
											<span class="inline-block min-w-12 text-right">{score}</span>
											{#if !readOnly && showActions && viewMode === 'zset'}
												<Button
													size="sm"
													variant="ghost"
													onclick={() => incrementScore(member, 1)}
													disabled={saving}
													class="h-6 w-6 p-0"
													title="Increment score by 1"
													aria-label="Increment score by 1"
												>
													<Plus class="h-3 w-3" />
												</Button>
											{:else}
												<div class="h-6 w-6"></div>
											{/if}
										</div>
									{/if}
								</Table.Cell>
								{#if !readOnly && showActions}
									<Table.Cell class="align-top">
										<ItemActions
											editing={editMode !== 'none' && editingMember === member}
											{saving}
											showRename={true}
											editLabel="Edit score"
											renameLabel="Rename member"
											onEdit={() => startEditingScore(member, score)}
											onRename={() => startRenamingMember(member)}
											onSave={() => saveEdit(editingValue)}
											onCancel={cancelEditing}
											onDelete={() => openDeleteDialog(member)}
										/>
									</Table.Cell>
								{/if}
							</Table.Row>
						{/each}
					</Table.Body>
				</Table.Root>
			</div>
		{/if}
	</div>
</div>

<DeleteItemDialog
	bind:open={deleteDialogOpen}
	itemType={viewMode === 'geo' ? 'geo' : 'zset'}
	itemDisplay={deleteTarget?.display ?? ''}
	onConfirm={confirmDelete}
	onCancel={() => (deleteDialogOpen = false)}
/>

<LargeValueWarningDialog
	bind:open={largeValueWarningOpen}
	valueSize={largeValueSize}
	onConfirm={confirmLargeValue}
	onCancel={cancelLargeValue}
/>

<ExpandedItemDialog
	bind:open={expandedDialogOpen}
	title={viewMode === 'geo' ? 'Geo Member' : 'ZSet Member'}
	value={expandedMember}
	metadata={viewMode === 'geo'
		? [
				{ label: 'Longitude', value: formatCoordinate(expandedLongitude) },
				{ label: 'Latitude', value: formatCoordinate(expandedLatitude) }
			]
		: [{ label: 'Score', value: String(expandedScore) }]}
	{readOnly}
	onSave={saveExpandedEdit}
	onCancel={closeExpandedView}
/>
