<script lang="ts">
  import * as AlertDialog from '$lib/components/ui/alert-dialog';
  import { Badge } from '$lib/components/ui/badge';
  import { Button } from '$lib/components/ui/button';
  import * as ButtonGroup from '$lib/components/ui/button-group';
  import { Input } from '$lib/components/ui/input';
  import * as Table from '$lib/components/ui/table';
  import { Textarea } from '$lib/components/ui/textarea';
  import CheckIcon from '@lucide/svelte/icons/check';
  import ChevronLeftIcon from '@lucide/svelte/icons/chevron-left';
  import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
  import ChevronsLeftIcon from '@lucide/svelte/icons/chevrons-left';
  import ChevronsRightIcon from '@lucide/svelte/icons/chevrons-right';
  import CopyIcon from '@lucide/svelte/icons/copy';
  import { toast } from 'svelte-sonner';
  import { api, type HashPair, type KeyInfo, type StreamEntry, type ZSetMember } from './api';
  import CollapsibleValue from './CollapsibleValue.svelte';
  import { copyToClipboard, deleteOps, formatTtl, getErrorMessage, highlightJson, modifyOps, toastError } from './utils';
  import { ws } from './ws';

  interface Props {
    key: string
    ondeleted: () => void
    readOnly: boolean
  }

  let { key, ondeleted, readOnly }: Props = $props()

  let keyInfo = $state<KeyInfo | null>(null)
  let loading = $state(false)
  let saving = $state(false)
  let updatingTtl = $state(false)
  let editValue = $state('')
  let editTtl = $state('')
  let error = $state('')
  let liveTtl = $state<number | null>(null)
  let ttlInterval: ReturnType<typeof setInterval> | null = null
  let expiresAt: number | null = null

  // JSON highlighting state
  let prettyPrint = $state(false)
  let highlightedHtml = $state('')
  let listHighlights = $state<Record<number, string>>({})

  // Raw view toggle for complex types
  let rawView = $state(false)

  // Pagination state
  let currentPage = $state(1)
  let pageSize = $state(100)

  // External modification detection
  let externallyModified = $state(false)
  let keyDeleted = $state(false)

  // Copy to clipboard state
  let copiedValue = $state(false)
  let copiedKey = $state(false)

  // Delete confirmation dialog
  let deleteDialogOpen = $state(false)

  function openDeleteDialog() {
    deleteDialogOpen = true
  }

  async function copyValue() {
    if (!keyInfo) return
    const text = typeof keyInfo.value === 'string' ? keyInfo.value : JSON.stringify(keyInfo.value, null, 2)
    await copyToClipboard(text, (v) => copiedValue = v)
  }

  async function copyKeyName() {
    await copyToClipboard(key, (v) => copiedKey = v)
  }

  function startTtlCountdown(ttl: number) {
    stopTtlCountdown()
    if (ttl > 0) {
      expiresAt = Date.now() + ttl * 1000
      updateLiveTtl()
      ttlInterval = setInterval(updateLiveTtl, 1000)
    } else {
      liveTtl = ttl
    }
  }

  function updateLiveTtl() {
    if (expiresAt !== null) {
      const remaining = Math.round((expiresAt - Date.now()) / 1000)
      liveTtl = Math.max(0, remaining)
      if (remaining <= 0) {
        stopTtlCountdown()
      }
    }
  }

  function stopTtlCountdown() {
    if (ttlInterval) {
      clearInterval(ttlInterval)
      ttlInterval = null
    }
    expiresAt = null
  }

  // Type-safe accessors for complex types
  function asArray(): string[] {
    return Array.isArray(keyInfo?.value) ? keyInfo.value as string[] : []
  }
  function asHash(): HashPair[] {
    // Backend now returns array of {field, value} pairs for pagination
    return Array.isArray(keyInfo?.value) ? keyInfo.value as HashPair[] : []
  }
  function asZSet(): ZSetMember[] {
    return Array.isArray(keyInfo?.value) ? keyInfo.value as ZSetMember[] : []
  }
  function asStream(): StreamEntry[] {
    return Array.isArray(keyInfo?.value) ? keyInfo.value as StreamEntry[] : []
  }

  function isJson(str: string): boolean {
    if (!str || str.length < 2) return false
    const trimmed = str.trim()
    if (!((trimmed.startsWith('{') && trimmed.endsWith('}')) ||
          (trimmed.startsWith('[') && trimmed.endsWith(']')))) {
      return false
    }
    try {
      JSON.parse(str)
      return true
    } catch {
      return false
    }
  }

  // Highlight string value when it changes or prettyPrint toggles
  $effect(() => {
    if (keyInfo?.type === 'string' && isJson(editValue)) {
      highlightedHtml = highlightJson(editValue, prettyPrint)
    } else {
      highlightedHtml = ''
    }
  })

  // Highlight list items containing JSON
  $effect(() => {
    if (keyInfo?.type === 'list') {
      const items = asArray()
      const highlights: Record<number, string> = {}
      for (let i = 0; i < items.length; i++) {
        if (isJson(items[i])) {
          highlights[i] = highlightJson(items[i], prettyPrint)
        }
      }
      listHighlights = highlights
    } else {
      listHighlights = {}
    }
  })

  let isJsonValue = $derived(keyInfo?.type === 'string' && isJson(editValue))

  // Check if current type is a complex type that supports raw view
  let isComplexType = $derived(
    keyInfo?.type === 'list' ||
    keyInfo?.type === 'set' ||
    keyInfo?.type === 'hash' ||
    keyInfo?.type === 'zset' ||
    keyInfo?.type === 'stream'
  )

  // Generate highlighted raw JSON for complex types
  let rawJsonHtml = $derived.by(() => {
    if (!keyInfo || !isComplexType || !rawView) return ''
    return highlightJson(JSON.stringify(keyInfo.value, null, 2), true)
  })

  let previousKey = $state<string | null>(null)

  $effect(() => {
    // Reset to page 1 only when key changes (not on pagination)
    if (previousKey !== key) {
      currentPage = 1
      previousKey = key
    }
    loadKey(key)
    // Reset external modification state when key changes
    externallyModified = false
    keyDeleted = false
    return () => stopTtlCountdown()
  })

  // Subscribe to WebSocket key events for external modification detection
  $effect(() => {
    if (!key) return

    const unsubscribe = ws.onKeyEvent((event) => {
      if (event.key !== key) return

      if (deleteOps.has(event.op)) {
        keyDeleted = true
        externallyModified = false
      } else if (modifyOps.has(event.op)) {
        // Only mark as externally modified if we're not currently saving
        if (!saving) {
          externallyModified = true
        }
      }
    })

    return unsubscribe
  })

  async function loadKey(k: string) {
    loading = true
    error = ''
    stopTtlCountdown()
    try {
      // Always fetch with pagination params - backend will add metadata only for complex types
      keyInfo = await api.getKey(k, currentPage, pageSize)
      editValue = typeof keyInfo.value === 'string' ? keyInfo.value : JSON.stringify(keyInfo.value, null, 2)
      editTtl = keyInfo.ttl > 0 ? String(keyInfo.ttl) : ''
      startTtlCountdown(keyInfo.ttl)
    } catch (e) {
      error = getErrorMessage(e, 'Failed to load key')
      keyInfo = null
    } finally {
      loading = false
    }
  }

  function goToPage(page: number) {
    currentPage = page
    loadKey(key)
  }

  function changePageSize(newSize: number) {
    pageSize = newSize
    currentPage = 1 // Reset to first page when changing page size
    loadKey(key)
  }

  // Computed pagination info
  let totalPages = $derived(
    keyInfo?.pagination ? Math.ceil(keyInfo.pagination.total / keyInfo.pagination.pageSize) : 0
  )
  let showingStart = $derived(
    keyInfo?.pagination ? (keyInfo.pagination.page - 1) * keyInfo.pagination.pageSize + 1 : 0
  )
  let showingEnd = $derived(
    keyInfo?.pagination ? Math.min(keyInfo.pagination.page * keyInfo.pagination.pageSize, keyInfo.pagination.total) : 0
  )

  async function saveValue() {
    if (!keyInfo) return
    saving = true
    error = ''
    try {
      const ttl = editTtl ? parseInt(editTtl, 10) : 0
      await api.setKey(key, editValue, ttl)
      await loadKey(key)
      toast.success('Value saved')
    } catch (e) {
      toastError(e, 'Failed to save')
    } finally {
      saving = false
    }
  }

  async function deleteKey() {
    try {
      await api.deleteKey(key)
      toast.success('Key deleted')
      ondeleted()
    } catch (e) {
      toastError(e, 'Failed to delete')
    } finally {
      deleteDialogOpen = false
    }
  }

  async function updateTtl() {
    if (!keyInfo) return
    updatingTtl = true
    try {
      const ttl = editTtl ? parseInt(editTtl, 10) : 0
      await api.expireKey(key, ttl)
      await loadKey(key)
      toast.success('TTL updated')
    } catch (e) {
      toastError(e, 'Failed to update TTL')
    } finally {
      updatingTtl = false
    }
  }
</script>

<div class="p-6 h-full flex flex-col gap-4">
  {#if loading}
    <div class="flex items-center justify-center h-full text-muted-foreground">Loading...</div>
  {:else if error}
    <div class="flex items-center justify-center h-full text-destructive">{error}</div>
  {:else if keyInfo}
    <div class="flex items-center gap-4">
      <h2 class="font-mono text-xl break-all flex-1">{key}</h2>
      <Badge variant="secondary" class="uppercase">{keyInfo.type}</Badge>
      <ButtonGroup.Root>
        <Button variant="outline" size="sm" onclick={copyKeyName} title="Copy key name to clipboard" class="cursor-pointer">
          {#if copiedKey}
            <CheckIcon class="w-4 h-4 text-primary" />
          {:else}
            <CopyIcon class="w-4 h-4" />
          {/if}
          Key
        </Button>
        <Button variant="outline" size="sm" onclick={copyValue} title="Copy value to clipboard" class="cursor-pointer">
          {#if copiedValue}
            <CheckIcon class="w-4 h-4 text-primary" />
          {:else}
            <CopyIcon class="w-4 h-4" />
          {/if}
          Value
        </Button>
      </ButtonGroup.Root>
    </div>

    {#if keyDeleted}
      <div class="bg-destructive/10 text-destructive p-3 rounded flex items-center justify-between text-sm">
        <span>This key was deleted externally</span>
        <Button variant="secondary" size="sm" onclick={ondeleted} class="cursor-pointer">
          Close
        </Button>
      </div>
    {:else if externallyModified}
      <div class="bg-accent/10 text-accent-foreground p-3 rounded flex items-center justify-between text-sm">
        <span>Modified externally</span>
        <Button variant="secondary" size="sm" onclick={() => { loadKey(key); externallyModified = false }} class="cursor-pointer" title="Reload key data">
          Reload
        </Button>
      </div>
    {/if}

    <div class="p-3 bg-muted rounded flex items-center justify-between gap-4">
      <label class="flex items-center gap-2">
        <span class="text-sm">TTL:</span>
        {#if readOnly}
          <span class="text-muted-foreground text-sm">{formatTtl(liveTtl ?? keyInfo.ttl)}</span>
        {:else}
          <Input
            type="number"
            bind:value={editTtl}
            placeholder="seconds"
            class="w-25"
          />
          <Button variant="secondary" size="sm" onclick={updateTtl} disabled={updatingTtl} class="cursor-pointer" title="Update TTL">
            {updatingTtl ? 'Setting...' : 'Set'}
          </Button>
          <span class="text-muted-foreground text-sm">{formatTtl(liveTtl ?? keyInfo.ttl)}</span>
        {/if}
      </label>
      {#if !readOnly}
        <div class="flex gap-2">
          {#if keyInfo.type === 'string'}
            <Button size="sm" onclick={saveValue} disabled={saving} class="cursor-pointer" title="Save changes">
              {saving ? 'Saving...' : 'Save'}
            </Button>
          {/if}
          <Button variant="destructive" size="sm" onclick={openDeleteDialog} class="cursor-pointer" title="Delete this key">Delete</Button>
        </div>
      {/if}
    </div>

    {#if keyInfo.type === 'string'}
      <div class="flex-1 flex flex-col gap-2">
        <div class="flex items-center justify-between">
          <label for="value-textarea">Value:</label>
          {#if isJsonValue}
            <button
              type="button"
              onclick={() => prettyPrint = !prettyPrint}
              class="px-2 py-1 text-xs rounded cursor-pointer bg-muted hover:bg-secondary text-foreground"
              title={prettyPrint ? 'Compact JSON formatting' : 'Pretty-print JSON formatting'}
            >
              {prettyPrint ? 'Compact JSON' : 'Format JSON'}
            </button>
          {/if}
        </div>

        {#if isJsonValue && highlightedHtml}
          <div class="flex-1 overflow-auto rounded border border-border min-h-75 [&>pre]:p-4 [&>pre]:m-0 [&>pre]:min-h-full [&>pre]:text-sm">
            {@html highlightedHtml}
          </div>
        {:else}
          <Textarea
            id="value-textarea"
            bind:value={editValue}
            readonly={readOnly}
            class="flex-1 resize-none text-sm min-h-75"
          />
        {/if}
      </div>
    {:else if keyInfo.type === 'list'}
      <div class="flex-1 flex flex-col gap-2 overflow-auto">
        {#if keyInfo.pagination}
          <div class="flex items-center justify-between gap-4 pb-2 border-b border-border">
            <span class="text-sm text-muted-foreground">
              Showing {showingStart}–{showingEnd} of {keyInfo.pagination.total} items
            </span>
            <div class="flex items-center gap-2">
              <span class="text-xs text-muted-foreground">Page size:</span>
              <select
                bind:value={pageSize}
                onchange={(e) => changePageSize(Number(e.currentTarget.value))}
                class="px-2 py-1 text-xs rounded border border-border bg-background cursor-pointer"
              >
                <option value={50}>50</option>
                <option value={100}>100</option>
                <option value={200}>200</option>
                <option value={500}>500</option>
              </select>
              <div class="flex gap-1">
                <Button
                  size="sm"
                  variant="outline"
                  onclick={() => goToPage(1)}
                  disabled={currentPage === 1}
                  class="cursor-pointer h-8 w-8 p-0"
                  title="First page"
                >
                  <ChevronsLeftIcon class="w-4 h-4" />
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  onclick={() => goToPage(currentPage - 1)}
                  disabled={currentPage === 1}
                  class="cursor-pointer h-8 w-8 p-0"
                  title="Previous page"
                >
                  <ChevronLeftIcon class="w-4 h-4" />
                </Button>
                <span class="px-3 py-1 text-sm flex items-center">
                  Page {currentPage} of {totalPages}
                </span>
                <Button
                  size="sm"
                  variant="outline"
                  onclick={() => goToPage(currentPage + 1)}
                  disabled={currentPage >= totalPages}
                  class="cursor-pointer h-8 w-8 p-0"
                  title="Next page"
                >
                  <ChevronRightIcon class="w-4 h-4" />
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  onclick={() => goToPage(totalPages)}
                  disabled={currentPage >= totalPages}
                  class="cursor-pointer h-8 w-8 p-0"
                  title="Last page"
                >
                  <ChevronsRightIcon class="w-4 h-4" />
                </Button>
              </div>
            </div>
          </div>
        {/if}
        <div class="flex items-center justify-between">
          <span class="text-sm text-muted-foreground">
            {#if keyInfo.pagination}
              {keyInfo.pagination.total} items total
            {:else}
              {keyInfo.length} items
            {/if}
          </span>
          <div class="flex gap-1 items-center">
            {#if !rawView && Object.keys(listHighlights).length > 0}
              <button
                type="button"
                onclick={() => prettyPrint = !prettyPrint}
                class="px-2 py-1 text-xs rounded cursor-pointer bg-muted hover:bg-secondary text-foreground"
                title={prettyPrint ? 'Compact JSON formatting' : 'Pretty-print JSON formatting'}
              >
                {prettyPrint ? 'Compact JSON' : 'Format JSON'}
              </button>
            {/if}
            <button
              type="button"
              onclick={() => rawView = !rawView}
              class="px-2 py-1 text-xs rounded cursor-pointer bg-muted hover:bg-secondary text-foreground"
              title={rawView ? 'View as structured table' : 'View as raw JSON document'}
            >
              {rawView ? 'Show as Table' : 'Show as Raw JSON'}
            </button>
          </div>
        </div>
        {#if rawView && rawJsonHtml}
          <div class="flex-1 overflow-auto rounded border border-border [&>pre]:p-4 [&>pre]:m-0 [&>pre]:min-h-full [&>pre]:text-sm">
            {@html rawJsonHtml}
          </div>
        {:else}
          <Table.Root>
            <Table.Header>
              <Table.Row>
                <Table.Head class="w-16">Index</Table.Head>
                <Table.Head>Value</Table.Head>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {#each asArray() as item, i}
                <Table.Row>
                  <Table.Cell class="text-muted-foreground font-mono align-top">{i}</Table.Cell>
                  <Table.Cell class="font-mono">
                    <CollapsibleValue value={item} highlight={listHighlights[i]} />
                  </Table.Cell>
                </Table.Row>
              {/each}
            </Table.Body>
          </Table.Root>
        {/if}
      </div>
    {:else if keyInfo.type === 'set'}
      <div class="flex-1 flex flex-col gap-2 overflow-auto">
        {#if keyInfo.pagination}
          <div class="flex items-center justify-between gap-4 pb-2 border-b border-border">
            <span class="text-sm text-muted-foreground">
              Showing {showingStart}–{showingEnd} of {keyInfo.pagination.total} members
            </span>
            <div class="flex items-center gap-2">
              <span class="text-xs text-muted-foreground">Page size:</span>
              <select
                bind:value={pageSize}
                onchange={(e) => changePageSize(Number(e.currentTarget.value))}
                class="px-2 py-1 text-xs rounded border border-border bg-background cursor-pointer"
              >
                <option value={50}>50</option>
                <option value={100}>100</option>
                <option value={200}>200</option>
                <option value={500}>500</option>
              </select>
              <div class="flex gap-1">
                <Button size="sm" variant="outline" onclick={() => goToPage(1)} disabled={currentPage === 1} class="cursor-pointer h-8 w-8 p-0" title="First page">
                  <ChevronsLeftIcon class="w-4 h-4" />
                </Button>
                <Button size="sm" variant="outline" onclick={() => goToPage(currentPage - 1)} disabled={currentPage === 1} class="cursor-pointer h-8 w-8 p-0" title="Previous page">
                  <ChevronLeftIcon class="w-4 h-4" />
                </Button>
                <span class="px-3 py-1 text-sm flex items-center">Page {currentPage} of {totalPages}</span>
                <Button size="sm" variant="outline" onclick={() => goToPage(currentPage + 1)} disabled={currentPage >= totalPages} class="cursor-pointer h-8 w-8 p-0" title="Next page">
                  <ChevronRightIcon class="w-4 h-4" />
                </Button>
                <Button size="sm" variant="outline" onclick={() => goToPage(totalPages)} disabled={currentPage >= totalPages} class="cursor-pointer h-8 w-8 p-0" title="Last page">
                  <ChevronsRightIcon class="w-4 h-4" />
                </Button>
              </div>
            </div>
          </div>
        {/if}
        <div class="flex items-center justify-between">
          <span class="text-sm text-muted-foreground">
            {#if keyInfo.pagination}
              {keyInfo.pagination.total} members total
            {:else}
              {keyInfo.length} members
            {/if}
          </span>
          <div class="flex gap-1 items-center">
            <button
              type="button"
              onclick={() => rawView = !rawView}
              class="px-2 py-1 text-xs rounded cursor-pointer bg-muted hover:bg-secondary text-foreground"
              title={rawView ? 'View as list of members' : 'View as raw JSON document'}
            >
              {rawView ? 'Show as List' : 'Show as Raw JSON'}
            </button>
          </div>
        </div>
        {#if rawView && rawJsonHtml}
          <div class="flex-1 overflow-auto rounded border border-border [&>pre]:p-4 [&>pre]:m-0 [&>pre]:min-h-full [&>pre]:text-sm">
            {@html rawJsonHtml}
          </div>
        {:else}
          <div class="flex flex-col gap-1">
            {#each asArray() as member}
              <div class="px-2 py-1 bg-muted rounded font-mono text-sm">
                <CollapsibleValue value={member} maxLength={100} />
              </div>
            {/each}
          </div>
        {/if}
      </div>
    {:else if keyInfo.type === 'hash'}
      <div class="flex-1 flex flex-col gap-2 overflow-auto">
        {#if keyInfo.pagination}
          <div class="flex items-center justify-between gap-4 pb-2 border-b border-border">
            <span class="text-sm text-muted-foreground">
              Showing {showingStart}–{showingEnd} of {keyInfo.pagination.total} fields
            </span>
            <div class="flex items-center gap-2">
              <span class="text-xs text-muted-foreground">Page size:</span>
              <select
                bind:value={pageSize}
                onchange={(e) => changePageSize(Number(e.currentTarget.value))}
                class="px-2 py-1 text-xs rounded border border-border bg-background cursor-pointer"
              >
                <option value={50}>50</option>
                <option value={100}>100</option>
                <option value={200}>200</option>
                <option value={500}>500</option>
              </select>
              <div class="flex gap-1">
                <Button size="sm" variant="outline" onclick={() => goToPage(1)} disabled={currentPage === 1} class="cursor-pointer h-8 w-8 p-0" title="First page">
                  <ChevronsLeftIcon class="w-4 h-4" />
                </Button>
                <Button size="sm" variant="outline" onclick={() => goToPage(currentPage - 1)} disabled={currentPage === 1} class="cursor-pointer h-8 w-8 p-0" title="Previous page">
                  <ChevronLeftIcon class="w-4 h-4" />
                </Button>
                <span class="px-3 py-1 text-sm flex items-center">Page {currentPage} of {totalPages}</span>
                <Button size="sm" variant="outline" onclick={() => goToPage(currentPage + 1)} disabled={currentPage >= totalPages} class="cursor-pointer h-8 w-8 p-0" title="Next page">
                  <ChevronRightIcon class="w-4 h-4" />
                </Button>
                <Button size="sm" variant="outline" onclick={() => goToPage(totalPages)} disabled={currentPage >= totalPages} class="cursor-pointer h-8 w-8 p-0" title="Last page">
                  <ChevronsRightIcon class="w-4 h-4" />
                </Button>
              </div>
            </div>
          </div>
        {/if}
        <div class="flex items-center justify-between">
          <span class="text-sm text-muted-foreground">
            {#if keyInfo.pagination}
              {keyInfo.pagination.total} fields total
            {:else}
              {keyInfo.length} fields
            {/if}
          </span>
          <div class="flex gap-1 items-center">
            <button
              type="button"
              onclick={() => rawView = !rawView}
              class="px-2 py-1 text-xs rounded cursor-pointer bg-muted hover:bg-secondary text-foreground"
              title={rawView ? 'View as field/value table' : 'View as raw JSON document'}
            >
              {rawView ? 'Show as Table' : 'Show as Raw JSON'}
            </button>
          </div>
        </div>
        {#if rawView && rawJsonHtml}
          <div class="flex-1 overflow-auto rounded border border-border [&>pre]:p-4 [&>pre]:m-0 [&>pre]:min-h-full [&>pre]:text-sm">
            {@html rawJsonHtml}
          </div>
        {:else}
          <Table.Root>
            <Table.Header>
              <Table.Row>
                <Table.Head>Field</Table.Head>
                <Table.Head>Value</Table.Head>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {#each asHash() as { field, value }}
                <Table.Row>
                  <Table.Cell class="font-mono text-muted-foreground align-top">{field}</Table.Cell>
                  <Table.Cell class="font-mono">
                    <CollapsibleValue value={value} highlight={isJson(value) ? highlightJson(value, false) : undefined} />
                  </Table.Cell>
                </Table.Row>
              {/each}
            </Table.Body>
          </Table.Root>
        {/if}
      </div>
    {:else if keyInfo.type === 'zset'}
      <div class="flex-1 flex flex-col gap-2 overflow-auto">
        {#if keyInfo.pagination}
          <div class="flex items-center justify-between gap-4 pb-2 border-b border-border">
            <span class="text-sm text-muted-foreground">
              Showing {showingStart}–{showingEnd} of {keyInfo.pagination.total} members
            </span>
            <div class="flex items-center gap-2">
              <span class="text-xs text-muted-foreground">Page size:</span>
              <select
                bind:value={pageSize}
                onchange={(e) => changePageSize(Number(e.currentTarget.value))}
                class="px-2 py-1 text-xs rounded border border-border bg-background cursor-pointer"
              >
                  <option value={50}>50</option>
                  <option value={100}>100</option>
                  <option value={200}>200</option>
                  <option value={500}>500</option>
              </select>
              <div class="flex gap-1">
                <Button size="sm" variant="outline" onclick={() => goToPage(1)} disabled={currentPage === 1} class="cursor-pointer h-8 w-8 p-0" title="First page">
                  <ChevronsLeftIcon class="w-4 h-4" />
                </Button>
                <Button size="sm" variant="outline" onclick={() => goToPage(currentPage - 1)} disabled={currentPage === 1} class="cursor-pointer h-8 w-8 p-0" title="Previous page">
                  <ChevronLeftIcon class="w-4 h-4" />
                </Button>
                <span class="px-3 py-1 text-sm flex items-center">Page {currentPage} of {totalPages}</span>
                <Button size="sm" variant="outline" onclick={() => goToPage(currentPage + 1)} disabled={currentPage >= totalPages} class="cursor-pointer h-8 w-8 p-0" title="Next page">
                  <ChevronRightIcon class="w-4 h-4" />
                </Button>
                <Button size="sm" variant="outline" onclick={() => goToPage(totalPages)} disabled={currentPage >= totalPages} class="cursor-pointer h-8 w-8 p-0" title="Last page">
                  <ChevronsRightIcon class="w-4 h-4" />
                </Button>
              </div>
            </div>
          </div>
        {/if}
        <div class="flex items-center justify-between">
          <span class="text-sm text-muted-foreground">
            {#if keyInfo.pagination}
              {keyInfo.pagination.total} members total
            {:else}
              {keyInfo.length} members
            {/if}
          </span>
          <div class="flex gap-1 items-center">
            <button
              type="button"
              onclick={() => rawView = !rawView}
              class="px-2 py-1 text-xs rounded cursor-pointer bg-muted hover:bg-secondary text-foreground"
              title={rawView ? 'View as member/score table' : 'View as raw JSON document'}
            >
              {rawView ? 'Show as Table' : 'Show as Raw JSON'}
            </button>
          </div>
        </div>
        {#if rawView && rawJsonHtml}
          <div class="flex-1 overflow-auto rounded border border-border [&>pre]:p-4 [&>pre]:m-0 [&>pre]:min-h-full [&>pre]:text-sm">
            {@html rawJsonHtml}
          </div>
        {:else}
          <Table.Root>
            <Table.Header>
              <Table.Row>
                <Table.Head>Member</Table.Head>
                <Table.Head class="w-24">Score</Table.Head>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {#each asZSet() as zitem}
                <Table.Row>
                  <Table.Cell class="font-mono">
                    <CollapsibleValue value={zitem.member} />
                  </Table.Cell>
                  <Table.Cell class="font-mono text-muted-foreground">{zitem.score}</Table.Cell>
                </Table.Row>
              {/each}
            </Table.Body>
          </Table.Root>
        {/if}
      </div>
    {:else if keyInfo.type === 'stream'}
      <div class="flex-1 flex flex-col gap-2 overflow-auto">
        {#if keyInfo.pagination}
          <div class="flex items-center justify-between gap-4 pb-2 border-b border-border">
            <span class="text-sm text-muted-foreground">
              Showing {showingStart}–{showingEnd} of {keyInfo.pagination.total} entries
            </span>
            <div class="flex items-center gap-2">
              <span class="text-xs text-muted-foreground">Page size:</span>
              <select
                bind:value={pageSize}
                onchange={(e) => changePageSize(Number(e.currentTarget.value))}
                class="px-2 py-1 text-xs rounded border border-border bg-background cursor-pointer"
              >
                <option value={50}>50</option>
                <option value={100}>100</option>
                <option value={200}>200</option>
                <option value={500}>500</option>
              </select>
              <div class="flex gap-1">
                <Button size="sm" variant="outline" onclick={() => goToPage(1)} disabled={currentPage === 1} class="cursor-pointer h-8 w-8 p-0" title="First page">
                  <ChevronsLeftIcon class="w-4 h-4" />
                </Button>
                <Button size="sm" variant="outline" onclick={() => goToPage(currentPage - 1)} disabled={currentPage === 1} class="cursor-pointer h-8 w-8 p-0" title="Previous page">
                  <ChevronLeftIcon class="w-4 h-4" />
                </Button>
                <span class="px-3 py-1 text-sm flex items-center">Page {currentPage} of {totalPages}</span>
                <Button size="sm" variant="outline" onclick={() => goToPage(currentPage + 1)} disabled={currentPage >= totalPages} class="cursor-pointer h-8 w-8 p-0" title="Next page">
                  <ChevronRightIcon class="w-4 h-4" />
                </Button>
                <Button size="sm" variant="outline" onclick={() => goToPage(totalPages)} disabled={currentPage >= totalPages} class="cursor-pointer h-8 w-8 p-0" title="Last page">
                  <ChevronsRightIcon class="w-4 h-4" />
                </Button>
              </div>
            </div>
          </div>
        {/if}
        <div class="flex items-center justify-between">
          <span class="text-sm text-muted-foreground">
            {#if keyInfo.pagination}
              {keyInfo.pagination.total} entries total
            {:else}
              {keyInfo.length} entries
            {/if}
          </span>
          <div class="flex gap-1 items-center">
            <button
              type="button"
              onclick={() => rawView = !rawView}
              class="px-2 py-1 text-xs rounded cursor-pointer bg-muted hover:bg-secondary text-foreground"
              title={rawView ? 'View as entry cards' : 'View as raw JSON document'}
            >
              {rawView ? 'Show as Cards' : 'Show as Raw JSON'}
            </button>
          </div>
        </div>
        {#if rawView && rawJsonHtml}
          <div class="flex-1 overflow-auto rounded border border-border [&>pre]:p-4 [&>pre]:m-0 [&>pre]:min-h-full [&>pre]:text-sm">
            {@html rawJsonHtml}
          </div>
        {:else}
          <div class="flex flex-col gap-2">
            {#each asStream() as entry}
              <div class="border border-border rounded p-3">
                <div class="font-mono text-xs text-muted-foreground mb-2">{entry.id}</div>
                <div class="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm">
                  {#each Object.entries(entry.fields) as [field, val]}
                    <span class="font-mono text-muted-foreground">{field}</span>
                    <span class="font-mono">
                      <CollapsibleValue value={val} maxLength={150} />
                    </span>
                  {/each}
                </div>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    {:else}
      <div class="flex flex-col gap-4">
        <p>Unknown type: {keyInfo.type}</p>
        <pre class="bg-muted p-4 rounded overflow-auto font-mono text-sm">{JSON.stringify(keyInfo.value, null, 2)}</pre>
      </div>
    {/if}

    <AlertDialog.Root bind:open={deleteDialogOpen}>
      <AlertDialog.Content>
        <AlertDialog.Header>
          <AlertDialog.Title>Delete Key</AlertDialog.Title>
          <AlertDialog.Description>
            Are you sure you want to delete <code class="font-mono bg-muted px-1 rounded">{key}</code>? This action cannot be undone.
          </AlertDialog.Description>
        </AlertDialog.Header>
        <AlertDialog.Footer>
          <AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
          <AlertDialog.Action onclick={deleteKey}>Delete</AlertDialog.Action>
        </AlertDialog.Footer>
      </AlertDialog.Content>
    </AlertDialog.Root>
  {/if}
</div>

<style>
  :global(.json-highlight) {
    margin: 0;
    font-family: ui-monospace, monospace;
    font-size: 0.875rem;
    line-height: 1.5;
    white-space: pre-wrap;
    word-break: break-all;
  }
  :global(.json-string) {
    color: #0550ae;
  }
  :global(.json-number) {
    color: #116329;
  }
  :global(.json-keyword) {
    color: #cf222e;
  }
</style>
