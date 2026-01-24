<script lang="ts">
  import { Badge } from '$lib/components/ui/badge';
  import { Button } from '$lib/components/ui/button';
  import { Input } from '$lib/components/ui/input';
  import { Textarea } from '$lib/components/ui/textarea';
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

  $effect(() => {
    loadKey(key)
  })

  async function loadKey(k: string) {
    loading = true
    error = ''
    try {
      keyInfo = await api.getKey(k)
      editValue = typeof keyInfo.value === 'string' ? keyInfo.value : JSON.stringify(keyInfo.value, null, 2)
      editTtl = keyInfo.ttl > 0 ? String(keyInfo.ttl) : ''
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
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to save'
    } finally {
      saving = false
    }
  }

  async function deleteKey() {
    if (!confirm(`Delete key "${key}"?`)) return
    try {
      await api.deleteKey(key)
      ondeleted()
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to delete'
    }
  }

  async function updateTtl() {
    if (!keyInfo) return
    try {
      const ttl = editTtl ? parseInt(editTtl, 10) : 0
      await api.expireKey(key, ttl)
      await loadKey(key)
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to update TTL'
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
      <h2 class="font-mono text-xl break-all">{key}</h2>
      <Badge variant="secondary" class="uppercase">{keyInfo.type}</Badge>
    </div>

    <div class="p-4 bg-alabaster-grey-50 rounded">
      <div class="flex items-center gap-4">
        <label class="flex items-center gap-2">
          TTL:
          {#if readOnly}
            <span class="text-black-400 text-sm">{formatTtl(keyInfo.ttl)}</span>
          {:else}
            <Input
              type="number"
              bind:value={editTtl}
              placeholder="seconds (empty = no expiry)"
              class="w-37.5"
            />
            <Button variant="secondary" onclick={updateTtl}>Set TTL</Button>
            <span class="text-black-400 text-sm">{formatTtl(keyInfo.ttl)}</span>
          {/if}
        </label>
      </div>
    </div>

    {#if keyInfo.type === 'string'}
      <div class="flex-1 flex flex-col gap-2">
        <label for="value-textarea">Value:</label>
        <Textarea
          id="value-textarea"
          bind:value={editValue}
          readonly={readOnly}
          class="flex-1 resize-none text-sm min-h-75"
        />
      </div>

      {#if !readOnly}
        <div class="flex gap-2">
          <Button onclick={saveValue} disabled={saving}>
            {saving ? 'Saving...' : 'Save'}
          </Button>
          <Button variant="destructive" onclick={deleteKey}>
            Delete
          </Button>
        </div>
      {/if}
    {:else if keyInfo.type === 'list'}
      <div class="flex-1 flex flex-col gap-2 overflow-auto">
        <div class="text-sm text-black-400">
          {keyInfo.length} items{keyInfo.length && keyInfo.length > 100 ? ' (showing first 100)' : ''}
        </div>
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
                <td class="p-2 text-black-400 font-mono">{i}</td>
                <td class="p-2 font-mono break-all">{item}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
      {#if !readOnly}
        <Button variant="destructive" onclick={deleteKey}>Delete</Button>
      {/if}
    {:else if keyInfo.type === 'set'}
      <div class="flex-1 flex flex-col gap-2 overflow-auto">
        <div class="text-sm text-black-400">
          {keyInfo.length} members{keyInfo.length && keyInfo.length > 100 ? ' (showing first 100)' : ''}
        </div>
        <div class="flex flex-wrap gap-2">
          {#each asArray() as member}
            <span class="px-2 py-1 bg-alabaster-grey-100 rounded font-mono text-sm">{member}</span>
          {/each}
        </div>
      </div>
      {#if !readOnly}
        <Button variant="destructive" onclick={deleteKey}>Delete</Button>
      {/if}
    {:else if keyInfo.type === 'hash'}
      <div class="flex-1 flex flex-col gap-2 overflow-auto">
        <div class="text-sm text-black-400">
          {keyInfo.length} fields{keyInfo.length && keyInfo.length > 100 ? ' (showing first 100)' : ''}
        </div>
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
                <td class="p-2 font-mono text-black-600">{field}</td>
                <td class="p-2 font-mono break-all">{val}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
      {#if !readOnly}
        <Button variant="destructive" onclick={deleteKey}>Delete</Button>
      {/if}
    {:else if keyInfo.type === 'zset'}
      <div class="flex-1 flex flex-col gap-2 overflow-auto">
        <div class="text-sm text-black-400">
          {keyInfo.length} members{keyInfo.length && keyInfo.length > 100 ? ' (showing first 100)' : ''}
        </div>
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
                <td class="p-2 font-mono break-all">{zitem.member}</td>
                <td class="p-2 font-mono text-black-600">{zitem.score}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
      {#if !readOnly}
        <Button variant="destructive" onclick={deleteKey}>Delete</Button>
      {/if}
    {:else if keyInfo.type === 'stream'}
      <div class="flex-1 flex flex-col gap-2 overflow-auto">
        <div class="text-sm text-black-400">
          {keyInfo.length} entries{keyInfo.length && keyInfo.length > 100 ? ' (showing first 100)' : ''}
        </div>
        <div class="flex flex-col gap-2">
          {#each asStream() as entry}
            <div class="border border-alabaster-grey-200 rounded p-3">
              <div class="font-mono text-xs text-black-400 mb-2">{entry.id}</div>
              <div class="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm">
                {#each Object.entries(entry.fields) as [field, val]}
                  <span class="font-mono text-black-600">{field}</span>
                  <span class="font-mono break-all">{val}</span>
                {/each}
              </div>
            </div>
          {/each}
        </div>
      </div>
      {#if !readOnly}
        <Button variant="destructive" onclick={deleteKey}>Delete</Button>
      {/if}
    {:else}
      <div class="flex flex-col gap-4">
        <p>Unknown type: {keyInfo.type}</p>
        <pre class="bg-alabaster-grey-50 p-4 rounded overflow-auto font-mono text-sm">{JSON.stringify(keyInfo.value, null, 2)}</pre>
        {#if !readOnly}
          <Button variant="destructive" onclick={deleteKey}>
            Delete
          </Button>
        {/if}
      </div>
    {/if}
  {/if}
</div>
