<script lang="ts">
	import AddItemForm from '$lib/AddItemForm.svelte';
	import { api, type GeoMember, type PaginationInfo, type ZSetMember } from '$lib/api';
	import CollapsibleValue from '$lib/CollapsibleValue.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import * as Table from '$lib/components/ui/table';
	import DeleteItemDialog from '$lib/DeleteItemDialog.svelte';
	import InlineEditor from '$lib/InlineEditor.svelte';
	import ItemActions from '$lib/ItemActions.svelte';
	import PaginationControls from '$lib/PaginationControls.svelte';
	import TypeHeader from '$lib/TypeHeader.svelte';
	import {
		formatCoordinate,
		highlightJson,
		isValidLatitude,
		isValidLongitude,
		isValidScore,
		parseScore,
		showPaginationControls,
		toastError
	} from '$lib/utils';
	import Map from '@lucide/svelte/icons/map';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import TableIcon from '@lucide/svelte/icons/table';
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
	let viewMode = $state<'zset' | 'geo'>('zset');
	let geoMembers = $state<GeoMember[]>([]);
	let loadingGeo = $state(false);
	let showMap = $state(false);

	// Add form state
	let showAddForm = $state(false);
	let addMember = $state('');
	let addScore = $state<string | number>('');
	let addLongitude = $state<string | number>('');
	let addLatitude = $state<string | number>('');
	let adding = $state(false);

	// Edit state
	let editingMember = $state<string | null>(null);
	let editingValue = $state('');
	let saving = $state(false);

	// Delete state
	let deleteDialogOpen = $state(false);
	let deleteTarget = $state<{ member: string; display: string } | null>(null);

	let rawJsonHtml = $derived(rawView ? highlightJson(JSON.stringify(members, null, 2), true) : '');

	// Reload geo data when key changes (if in geo mode)
	let previousKeyName: string | null = null;
	$effect(() => {
		if (previousKeyName !== null && keyName !== previousKeyName) {
			if (viewMode === 'geo') {
				loadGeoData();
			}
		}
		previousKeyName = keyName;
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
		loadingGeo = true;
		try {
			const result = await api.geoGet(keyName, currentPage, pageSize);
			geoMembers = result.value as GeoMember[];
		} catch (e) {
			toastError(e, 'Failed to load geo data');
			viewMode = 'zset';
		} finally {
			loadingGeo = false;
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

	function startEditing(member: string, score: number) {
		editingMember = member;
		editingValue = String(score);
	}

	function cancelEditing() {
		editingMember = null;
		editingValue = '';
	}

	async function saveEdit(value: string) {
		if (editingMember === null) return;
		if (!isValidScore(value)) {
			toast.error('Invalid score value');
			return;
		}
		saving = true;
		try {
			await api.zsetAdd(keyName, editingMember, parseScore(value));
			toast.success('Score updated');
			cancelEditing();
			onDataChange();
		} catch (e) {
			toastError(e, 'Failed to update score');
		} finally {
			saving = false;
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
</script>

<div class="flex min-h-0 flex-1 flex-col gap-2">
	<TypeHeader expanded={typeHeaderExpanded}>
		{#if pagination && showPaginationControls(pagination.total)}
			<PaginationControls
				page={currentPage}
				{pageSize}
				total={pagination.total}
				itemLabel="members"
				{onPageChange}
				{onPageSizeChange}
			/>
		{/if}

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
						class="cursor-pointer"
						title={viewMode === 'geo' ? 'Add location to geo index' : 'Add member to sorted set'}
						aria-label={viewMode === 'geo'
							? 'Add location to geo index'
							: 'Add member to sorted set'}
					>
						<PlusIcon class="mr-1 h-4 w-4" />
						{viewMode === 'geo' ? 'Add Location' : 'Add Member'}
					</Button>
				{/if}
				<Button
					size="sm"
					variant="outline"
					onclick={toggleGeoView}
					class="cursor-pointer"
					title={viewMode === 'zset' ? 'View as geo coordinates' : 'View as sorted set scores'}
					aria-label={viewMode === 'zset' ? 'View as geo coordinates' : 'View as sorted set scores'}
				>
					{viewMode === 'zset' ? 'View as Geo' : 'View as ZSet'}
				</Button>
				{#if viewMode === 'zset'}
					<Button
						size="sm"
						variant="outline"
						onclick={() => (rawView = !rawView)}
						class="cursor-pointer"
						title={rawView ? 'Show as Table' : 'Show as Raw JSON'}
						aria-label={rawView ? 'Show as Table' : 'Show as Raw JSON'}
					>
						{rawView ? 'Show as Table' : 'Show as Raw JSON'}
					</Button>
				{:else}
					<Button
						size="sm"
						variant="outline"
						onclick={() => (showMap = !showMap)}
						class="cursor-pointer"
						title={showMap ? 'Show as Table' : 'Show on Map'}
						aria-label={showMap ? 'Show as Table' : 'Show on Map'}
					>
						{#if showMap}
							<TableIcon /> Show as Table
						{:else}
							<Map /> Show on Map
						{/if}
					</Button>
				{/if}
			</div>
		</div>

		{#if showAddForm}
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
		{/if}
	</TypeHeader>

	<div class="-mx-6 min-h-0 flex-1 overflow-auto border-t border-border px-6 pt-2">
		{#if rawView && rawJsonHtml && viewMode === 'zset'}
			<div
				class="rounded border border-border [&>pre]:m-0 [&>pre]:min-h-full [&>pre]:p-4 [&>pre]:text-sm"
			>
				{@html rawJsonHtml}
			</div>
		{:else if viewMode === 'geo'}
			{#if loadingGeo}
				<div class="flex items-center justify-center py-8 text-muted-foreground">
					Loading geo data...
				</div>
			{:else if showMap}
				<GeoMapView members={geoMembers} />
			{:else}
				<Table.Root>
					<Table.Header>
						<Table.Row>
							<Table.Head>Member</Table.Head>
							<Table.Head class="w-36">Longitude</Table.Head>
							<Table.Head class="w-36">Latitude</Table.Head>
							{#if !readOnly}
								<Table.Head class="w-24">Actions</Table.Head>
							{/if}
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each geoMembers as { member, longitude, latitude }}
							<Table.Row>
								<Table.Cell class="font-mono">
									<CollapsibleValue value={member} />
								</Table.Cell>
								<Table.Cell class="font-mono text-muted-foreground">
									{formatCoordinate(longitude)}
								</Table.Cell>
								<Table.Cell class="font-mono text-muted-foreground">
									{formatCoordinate(latitude)}
								</Table.Cell>
								{#if !readOnly}
									<Table.Cell class="align-top">
										<ItemActions
											editing={false}
											{saving}
											showEdit={false}
											onDelete={() => openDeleteDialog(member)}
										/>
									</Table.Cell>
								{/if}
							</Table.Row>
						{/each}
					</Table.Body>
				</Table.Root>
			{/if}
		{:else}
			<Table.Root>
				<Table.Header>
					<Table.Row>
						<Table.Head>Member</Table.Head>
						<Table.Head class="w-32">Score</Table.Head>
						{#if !readOnly}
							<Table.Head class="w-24">Actions</Table.Head>
						{/if}
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each members as { member, score }}
						<Table.Row>
							<Table.Cell class="font-mono">
								<CollapsibleValue value={member} />
							</Table.Cell>
							<Table.Cell class="font-mono text-muted-foreground">
								{#if editingMember === member}
									<InlineEditor
										bind:value={editingValue}
										type="number"
										inputClass="w-24"
										onSave={saveEdit}
										onCancel={cancelEditing}
									/>
								{:else}
									{score}
								{/if}
							</Table.Cell>
							{#if !readOnly}
								<Table.Cell class="align-top">
									<ItemActions
										editing={editingMember === member}
										{saving}
										onEdit={() => startEditing(member, score)}
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
