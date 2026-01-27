<script lang="ts">
  import * as AlertDialog from '$lib/components/ui/alert-dialog';
  import { Badge } from '$lib/components/ui/badge';
  import { Button } from '$lib/components/ui/button';
  import * as ButtonGroup from '$lib/components/ui/button-group';
  import { Input } from '$lib/components/ui/input';
  import { Textarea } from '$lib/components/ui/textarea';
  import CheckIcon from '@lucide/svelte/icons/check';
  import CopyIcon from '@lucide/svelte/icons/copy';
  import { toast } from 'svelte-sonner';
  import CollapsibleValue from './CollapsibleValue.svelte';
  import { ws } from './ws';

  import { api, type KeyInfo, type StreamEntry, type ZSetMember } from './api';

  interface Props {
    key: string
    ondeleted: () => void
    readOnly: boolean
  }

  let { key, ondeleted, readOnly }: Props = $props()

  let keyInfo = $state<KeyInfo | null>(null)
  let loading = $state(false)
  let saving = $state(false)
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
    await navigator.clipboard.writeText(text)
    copiedValue = true
    setTimeout(() => copiedValue = false, 2000)
  }

  async function copyKeyName() {
    await navigator.clipboard.writeText(key)
    copiedKey = true
    setTimeout(() => copiedKey = false, 2000)
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
  function asHash(): Record<string, string> {
    return keyInfo?.value && typeof keyInfo.value === 'object' && !Array.isArray(keyInfo.value)
      ? keyInfo.value as Record<string, string>
      : {}
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

  // Lightweight JSON syntax highlighter (no dependencies)
  function highlight(str: string, format: boolean): string {
    try {
      const code = format ? JSON.stringify(JSON.parse(str), null, 2) : str
      // Escape HTML and apply syntax highlighting
      const escaped = code
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')

      // Apply highlighting with regex
      const highlighted = escaped
        // Strings (including keys)
        .replace(/"([^"\\]|\\.)*"/g, (match) => {
          return `<span class="json-string">${match}</span>`
        })
        // Numbers
        .replace(/\b(-?\d+\.?\d*(?:[eE][+-]?\d+)?)\b/g, '<span class="json-number">$1</span>')
        // Booleans and null
        .replace(/\b(true|false|null)\b/g, '<span class="json-keyword">$1</span>')

      return `<pre class="json-highlight">${highlighted}</pre>`
    } catch {
      return ''
    }
  }

  // Highlight string value when it changes or prettyPrint toggles
  $effect(() => {
    if (keyInfo?.type === 'string' && isJson(editValue)) {
      highlightedHtml = highlight(editValue, prettyPrint)
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
          highlights[i] = highlight(items[i], prettyPrint)
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
    return highlight(JSON.stringify(keyInfo.value, null, 2), true)
  })

  $effect(() => {
    loadKey(key)
    // Reset external modification state when key changes
    externallyModified = false
    keyDeleted = false
    rawView = false
    return () => stopTtlCountdown()
  })

  // Subscribe to WebSocket key events for external modification detection
  // Operations that indicate a key was deleted
  const deleteOps = new Set(['del', 'expired'])
  // Operations that indicate a key was modified
  const modifyOps = new Set([
    'set',           // string
    'lpush', 'rpush', 'lpop', 'rpop', 'lset', 'ltrim',  // list
    'hset', 'hdel', 'hincrby', 'hincrbyfloat',          // hash
    'sadd', 'srem', 'spop',                              // set
    'zadd', 'zrem', 'zincrby',                           // sorted set
    'xadd', 'xtrim',                                     // stream
    'append', 'incr', 'decr', 'incrby', 'decrby',       // string modifications
    'setex', 'psetex', 'setnx',                          // string variants
  ])

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
      keyInfo = await api.getKey(k)
      editValue = typeof keyInfo.value === 'string' ? keyInfo.value : JSON.stringify(keyInfo.value, null, 2)
      editTtl = keyInfo.ttl > 0 ? String(keyInfo.ttl) : ''
      startTtlCountdown(keyInfo.ttl)
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to load key'
      keyInfo = null
    } finally {
      loading = false
    }
  }

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
      toast.error(e instanceof Error ? e.message : 'Failed to save')
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
      toast.error(e instanceof Error ? e.message : 'Failed to delete')
    } finally {
      deleteDialogOpen = false
    }
  }

  async function updateTtl() {
    if (!keyInfo) return
    try {
      const ttl = editTtl ? parseInt(editTtl, 10) : 0
      await api.expireKey(key, ttl)
      await loadKey(key)
      toast.success('TTL updated')
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Failed to update TTL')
    }
  }

  function formatTtl(seconds: number): string {
    if (seconds < 0) return 'No expiry'
    if (seconds < 60) return `${seconds}s`
    if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`
    return `${Math.floor(seconds / 3600)}h ${Math.floor((seconds % 3600) / 60)}m`
  }
</script>

<div class="p-6 h-full flex flex-col gap-4">
  {#if loading}
    <div class="flex items-center justify-center h-full text-black-400">Loading...</div>
  {:else if error}
    <div class="flex items-center justify-center h-full text-scarlet-rush-400">{error}</div>
  {:else if keyInfo}
    <div class="flex items-center gap-4">
      <h2 class="font-mono text-xl break-all flex-1">{key}</h2>
      <Badge variant="secondary" class="uppercase">{keyInfo.type}</Badge>
      <ButtonGroup.Root>
        <Button variant="outline" size="sm" onclick={copyKeyName} title="Copy key name">
          {#if copiedKey}
            <CheckIcon class="w-4 h-4 text-crayola-blue-500" />
          {:else}
            <CopyIcon class="w-4 h-4" />
          {/if}
          Key
        </Button>
        <Button variant="outline" size="sm" onclick={copyValue} title="Copy value">
          {#if copiedValue}
            <CheckIcon class="w-4 h-4 text-crayola-blue-500" />
          {:else}
            <CopyIcon class="w-4 h-4" />
          {/if}
          Value
        </Button>
      </ButtonGroup.Root>
    </div>

    {#if keyDeleted}
      <div class="bg-scarlet-rush-100 text-scarlet-rush-800 p-3 rounded flex items-center justify-between text-sm">
        <span>This key was deleted externally</span>
        <Button variant="secondary" size="sm" onclick={ondeleted}>
          Close
        </Button>
      </div>
    {:else if externallyModified}
      <div class="bg-golden-pollen-100 text-golden-pollen-800 p-3 rounded flex items-center justify-between text-sm">
        <span>Modified externally</span>
        <Button variant="secondary" size="sm" onclick={() => { loadKey(key); externallyModified = false }}>
          Reload
        </Button>
      </div>
    {/if}

    <div class="p-3 bg-alabaster-grey-50 rounded flex items-center justify-between gap-4">
      <label class="flex items-center gap-2">
        <span class="text-sm">TTL:</span>
        {#if readOnly}
          <span class="text-black-400 text-sm">{formatTtl(liveTtl ?? keyInfo.ttl)}</span>
        {:else}
          <Input
            type="number"
            bind:value={editTtl}
            placeholder="seconds"
            class="w-25"
          />
          <Button variant="secondary" size="sm" onclick={updateTtl}>Set</Button>
          <span class="text-black-400 text-sm">{formatTtl(liveTtl ?? keyInfo.ttl)}</span>
        {/if}
      </label>
      {#if !readOnly}
        <div class="flex gap-2">
          {#if keyInfo.type === 'string'}
            <Button size="sm" onclick={saveValue} disabled={saving}>
              {saving ? 'Saving...' : 'Save'}
            </Button>
          {/if}
          <Button variant="destructive" size="sm" onclick={openDeleteDialog}>Delete</Button>
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
              class="px-2 py-1 text-xs rounded font-mono {prettyPrint ? 'bg-crayola-blue-100 text-crayola-blue-700' : 'bg-alabaster-grey-100 hover:bg-alabaster-grey-200'}"
              title="Toggle JSON formatting"
            >
              {"{ }"}
            </button>
          {/if}
        </div>

        {#if isJsonValue && highlightedHtml}
          <div class="flex-1 overflow-auto rounded border border-alabaster-grey-200 min-h-75 [&>pre]:p-4 [&>pre]:m-0 [&>pre]:min-h-full [&>pre]:text-sm">
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
        <div class="flex items-center justify-between">
          <span class="text-sm text-black-400">
            {keyInfo.length} items{keyInfo.length && keyInfo.length > 100 ? ' (showing first 100)' : ''}
          </span>
          <div class="flex gap-1">
            {#if !rawView && Object.keys(listHighlights).length > 0}
              <button
                type="button"
                onclick={() => prettyPrint = !prettyPrint}
                class="px-2 py-1 text-xs rounded font-mono {prettyPrint ? 'bg-crayola-blue-100 text-crayola-blue-700' : 'bg-alabaster-grey-100 hover:bg-alabaster-grey-200'}"
                title="Toggle JSON formatting"
              >
                {"{ }"}
              </button>
            {/if}
            <button
              type="button"
              onclick={() => rawView = !rawView}
              class="px-2 py-1 text-xs rounded bg-alabaster-grey-100 hover:bg-alabaster-grey-200"
            >
              {rawView ? 'View as table' : 'View as JSON'}
            </button>
          </div>
        </div>
        {#if rawView && rawJsonHtml}
          <div class="flex-1 overflow-auto rounded border border-alabaster-grey-200 [&>pre]:p-4 [&>pre]:m-0 [&>pre]:min-h-full [&>pre]:text-sm">
            {@html rawJsonHtml}
          </div>
        {:else}
          <table class="w-full text-sm border-collapse">
            <thead class="bg-alabaster-grey-50 sticky top-0">
              <tr>
                <th class="text-left p-2 border-b w-16">Index</th>
                <th class="text-left p-2 border-b">Value</th>
              </tr>
            </thead>
            <tbody>
              {#each asArray() as item, i}
                <tr class="border-b border-alabaster-grey-100 hover:bg-alabaster-grey-50">
                  <td class="p-2 text-black-400 font-mono align-top">{i}</td>
                  <td class="p-2 font-mono">
                    <CollapsibleValue value={item} highlight={listHighlights[i]} />
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      </div>
    {:else if keyInfo.type === 'set'}
      <div class="flex-1 flex flex-col gap-2 overflow-auto">
        <div class="flex items-center justify-between">
          <span class="text-sm text-black-400">
            {keyInfo.length} members{keyInfo.length && keyInfo.length > 100 ? ' (showing first 100)' : ''}
          </span>
          <button
            type="button"
            onclick={() => rawView = !rawView}
            class="px-2 py-1 text-xs rounded bg-alabaster-grey-100 hover:bg-alabaster-grey-200"
          >
            {rawView ? 'View as list' : 'View as JSON'}
          </button>
        </div>
        {#if rawView && rawJsonHtml}
          <div class="flex-1 overflow-auto rounded border border-alabaster-grey-200 [&>pre]:p-4 [&>pre]:m-0 [&>pre]:min-h-full [&>pre]:text-sm">
            {@html rawJsonHtml}
          </div>
        {:else}
          <div class="flex flex-col gap-1">
            {#each asArray() as member}
              <div class="px-2 py-1 bg-alabaster-grey-100 rounded font-mono text-sm">
                <CollapsibleValue value={member} maxLength={100} />
              </div>
            {/each}
          </div>
        {/if}
      </div>
    {:else if keyInfo.type === 'hash'}
      <div class="flex-1 flex flex-col gap-2 overflow-auto">
        <div class="flex items-center justify-between">
          <span class="text-sm text-black-400">
            {keyInfo.length} fields{keyInfo.length && keyInfo.length > 100 ? ' (showing first 100)' : ''}
          </span>
          <button
            type="button"
            onclick={() => rawView = !rawView}
            class="px-2 py-1 text-xs rounded bg-alabaster-grey-100 hover:bg-alabaster-grey-200"
          >
            {rawView ? 'View as table' : 'View as JSON'}
          </button>
        </div>
        {#if rawView && rawJsonHtml}
          <div class="flex-1 overflow-auto rounded border border-alabaster-grey-200 [&>pre]:p-4 [&>pre]:m-0 [&>pre]:min-h-full [&>pre]:text-sm">
            {@html rawJsonHtml}
          </div>
        {:else}
          <table class="w-full text-sm border-collapse">
            <thead class="bg-alabaster-grey-50 sticky top-0">
              <tr>
                <th class="text-left p-2 border-b">Field</th>
                <th class="text-left p-2 border-b">Value</th>
              </tr>
            </thead>
            <tbody>
              {#each Object.entries(asHash()) as [field, val]}
                <tr class="border-b border-alabaster-grey-100 hover:bg-alabaster-grey-50">
                  <td class="p-2 font-mono text-black-600 align-top">{field}</td>
                  <td class="p-2 font-mono">
                    <CollapsibleValue value={val} highlight={isJson(val) ? highlight(val, false) : undefined} />
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      </div>
    {:else if keyInfo.type === 'zset'}
      <div class="flex-1 flex flex-col gap-2 overflow-auto">
        <div class="flex items-center justify-between">
          <span class="text-sm text-black-400">
            {keyInfo.length} members{keyInfo.length && keyInfo.length > 100 ? ' (showing first 100)' : ''}
          </span>
          <button
            type="button"
            onclick={() => rawView = !rawView}
            class="px-2 py-1 text-xs rounded bg-alabaster-grey-100 hover:bg-alabaster-grey-200"
          >
            {rawView ? 'View as table' : 'View as JSON'}
          </button>
        </div>
        {#if rawView && rawJsonHtml}
          <div class="flex-1 overflow-auto rounded border border-alabaster-grey-200 [&>pre]:p-4 [&>pre]:m-0 [&>pre]:min-h-full [&>pre]:text-sm">
            {@html rawJsonHtml}
          </div>
        {:else}
          <table class="w-full text-sm border-collapse">
            <thead class="bg-alabaster-grey-50 sticky top-0">
              <tr>
                <th class="text-left p-2 border-b">Member</th>
                <th class="text-left p-2 border-b w-24">Score</th>
              </tr>
            </thead>
            <tbody>
              {#each asZSet() as zitem}
                <tr class="border-b border-alabaster-grey-100 hover:bg-alabaster-grey-50">
                  <td class="p-2 font-mono">
                    <CollapsibleValue value={zitem.member} />
                  </td>
                  <td class="p-2 font-mono text-black-600">{zitem.score}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      </div>
    {:else if keyInfo.type === 'stream'}
      <div class="flex-1 flex flex-col gap-2 overflow-auto">
        <div class="flex items-center justify-between">
          <span class="text-sm text-black-400">
            {keyInfo.length} entries{keyInfo.length && keyInfo.length > 100 ? ' (showing first 100)' : ''}
          </span>
          <button
            type="button"
            onclick={() => rawView = !rawView}
            class="px-2 py-1 text-xs rounded bg-alabaster-grey-100 hover:bg-alabaster-grey-200"
          >
            {rawView ? 'View as cards' : 'View as JSON'}
          </button>
        </div>
        {#if rawView && rawJsonHtml}
          <div class="flex-1 overflow-auto rounded border border-alabaster-grey-200 [&>pre]:p-4 [&>pre]:m-0 [&>pre]:min-h-full [&>pre]:text-sm">
            {@html rawJsonHtml}
          </div>
        {:else}
          <div class="flex flex-col gap-2">
            {#each asStream() as entry}
              <div class="border border-alabaster-grey-200 rounded p-3">
                <div class="font-mono text-xs text-black-400 mb-2">{entry.id}</div>
                <div class="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm">
                  {#each Object.entries(entry.fields) as [field, val]}
                    <span class="font-mono text-black-600">{field}</span>
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
        <pre class="bg-alabaster-grey-50 p-4 rounded overflow-auto font-mono text-sm">{JSON.stringify(keyInfo.value, null, 2)}</pre>
      </div>
    {/if}

    <AlertDialog.Root bind:open={deleteDialogOpen}>
      <AlertDialog.Content>
        <AlertDialog.Header>
          <AlertDialog.Title>Delete Key</AlertDialog.Title>
          <AlertDialog.Description>
            Are you sure you want to delete <code class="font-mono bg-alabaster-grey-100 px-1 rounded">{key}</code>? This action cannot be undone.
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
