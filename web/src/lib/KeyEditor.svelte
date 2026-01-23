<script lang="ts">
  import { api, type KeyInfo } from './api'

  interface Props {
    key: string
    ondeleted: () => void
  }

  let { key, ondeleted }: Props = $props()

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

<div class="editor">
  {#if loading}
    <div class="loading">Loading...</div>
  {:else if error}
    <div class="error">{error}</div>
  {:else if keyInfo}
    <div class="header">
      <h2>{key}</h2>
      <span class="type-badge">{keyInfo.type}</span>
    </div>

    <div class="meta">
      <div class="ttl-section">
        <label>
          TTL:
          <input
            type="number"
            bind:value={editTtl}
            placeholder="seconds (empty = no expiry)"
          />
          <button class="btn-secondary" onclick={updateTtl}>Set TTL</button>
        </label>
        <span class="ttl-display">{formatTtl(keyInfo.ttl)}</span>
      </div>
    </div>

    {#if keyInfo.type === 'string'}
      <div class="value-editor">
        <label for="value-textarea">Value:</label>
        <textarea
          id="value-textarea"
          bind:value={editValue}
          rows="15"
        ></textarea>
      </div>

      <div class="actions">
        <button class="btn-primary" onclick={saveValue} disabled={saving}>
          {saving ? 'Saving...' : 'Save'}
        </button>
        <button class="btn-danger" onclick={deleteKey}>
          Delete
        </button>
      </div>
    {:else}
      <div class="unsupported">
        <p>Editing {keyInfo.type} values is not yet supported.</p>
        <pre>{JSON.stringify(keyInfo.value, null, 2)}</pre>
        <button class="btn-danger" onclick={deleteKey}>
          Delete
        </button>
      </div>
    {/if}
  {/if}
</div>

<style>
  .editor {
    padding: 1.5rem;
    height: 100%;
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .loading, .error {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 100%;
    color: var(--text-secondary);
  }

  .error {
    color: var(--accent);
  }

  .header {
    display: flex;
    align-items: center;
    gap: 1rem;
  }

  h2 {
    font-family: var(--font-mono);
    font-size: 1.25rem;
    word-break: break-all;
  }

  .type-badge {
    padding: 0.25rem 0.5rem;
    background: var(--bg-tertiary);
    border-radius: 4px;
    font-size: 0.75rem;
    text-transform: uppercase;
  }

  .meta {
    padding: 1rem;
    background: var(--bg-secondary);
    border-radius: 4px;
  }

  .ttl-section {
    display: flex;
    align-items: center;
    gap: 1rem;
  }

  .ttl-section label {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .ttl-section input {
    width: 150px;
  }

  .ttl-display {
    color: var(--text-secondary);
    font-size: 0.875rem;
  }

  .value-editor {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }

  .value-editor textarea {
    flex: 1;
    resize: none;
    font-size: 0.875rem;
  }

  .actions {
    display: flex;
    gap: 0.5rem;
  }

  .unsupported {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .unsupported pre {
    background: var(--bg-secondary);
    padding: 1rem;
    border-radius: 4px;
    overflow: auto;
    font-family: var(--font-mono);
    font-size: 0.875rem;
  }
</style>
