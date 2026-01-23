<script lang="ts">
  import { api, type KeyInfo } from './api'

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
      <span class="px-2 py-1 bg-black-800 rounded text-xs uppercase">{keyInfo.type}</span>
    </div>

    <div class="p-4 bg-black-900 rounded">
      <div class="flex items-center gap-4">
        <label class="flex items-center gap-2">
          TTL:
          {#if readOnly}
            <span class="text-black-400 text-sm">{formatTtl(keyInfo.ttl)}</span>
          {:else}
            <input
              type="number"
              bind:value={editTtl}
              placeholder="seconds (empty = no expiry)"
              class="w-[150px]"
            />
            <button class="btn-secondary" onclick={updateTtl}>Set TTL</button>
            <span class="text-black-400 text-sm">{formatTtl(keyInfo.ttl)}</span>
          {/if}
        </label>
      </div>
    </div>

    {#if keyInfo.type === 'string'}
      <div class="flex-1 flex flex-col gap-2">
        <label for="value-textarea">Value:</label>
        <textarea
          id="value-textarea"
          bind:value={editValue}
          rows="15"
          readonly={readOnly}
          class="flex-1 resize-none text-sm"
        ></textarea>
      </div>

      {#if !readOnly}
        <div class="flex gap-2">
          <button class="btn-primary" onclick={saveValue} disabled={saving}>
            {saving ? 'Saving...' : 'Save'}
          </button>
          <button class="btn-danger" onclick={deleteKey}>
            Delete
          </button>
        </div>
      {/if}
    {:else}
      <div class="flex flex-col gap-4">
        <p>Editing {keyInfo.type} values is not yet supported.</p>
        <pre class="bg-black-900 p-4 rounded overflow-auto font-mono text-sm">{JSON.stringify(keyInfo.value, null, 2)}</pre>
        {#if !readOnly}
          <button class="btn-danger" onclick={deleteKey}>
            Delete
          </button>
        {/if}
      </div>
    {/if}
  {/if}
</div>
