<script lang="ts">
  import { onMount } from 'svelte'
  import { api } from './api'

  interface Props {
    selected: string | null
    onselect: (key: string) => void
    oncreated: () => void
  }

  let { selected, onselect, oncreated }: Props = $props()

  let keys = $state<string[]>([])
  let pattern = $state('*')
  let loading = $state(false)
  let cursor = $state(0)
  let hasMore = $state(false)
  let showNewKey = $state(false)
  let newKeyName = $state('')

  async function loadKeys(reset = false) {
    loading = true
    try {
      const c = reset ? 0 : cursor
      const result = await api.getKeys(pattern, c)
      if (reset) {
        keys = result.keys
      } else {
        keys = [...keys, ...result.keys]
      }
      cursor = result.cursor
      hasMore = result.cursor !== 0
    } catch (e) {
      console.error('Failed to load keys:', e)
    } finally {
      loading = false
    }
  }

  function handleSearch() {
    loadKeys(true)
  }

  async function createKey() {
    if (!newKeyName.trim()) return
    try {
      await api.setKey(newKeyName, '')
      newKeyName = ''
      showNewKey = false
      await loadKeys(true)
      onselect(newKeyName)
      oncreated()
    } catch (e) {
      console.error('Failed to create key:', e)
    }
  }

  onMount(() => {
    loadKeys(true)
  })
</script>

<div class="key-list">
  <div class="search">
    <input
      type="text"
      bind:value={pattern}
      placeholder="Pattern (e.g., user:*)"
      onkeydown={(e) => e.key === 'Enter' && handleSearch()}
    />
    <button class="btn-primary" onclick={handleSearch} disabled={loading}>
      Search
    </button>
  </div>

  <div class="actions">
    <button class="btn-secondary" onclick={() => showNewKey = !showNewKey}>
      + New Key
    </button>
  </div>

  {#if showNewKey}
    <div class="new-key">
      <input
        type="text"
        bind:value={newKeyName}
        placeholder="Key name"
        onkeydown={(e) => e.key === 'Enter' && createKey()}
      />
      <button class="btn-primary" onclick={createKey}>Create</button>
    </div>
  {/if}

  <ul class="keys">
    {#each keys as key (key)}
      <li>
        <button
          class="key-item"
          class:selected={key === selected}
          onclick={() => onselect(key)}
        >
          {key}
        </button>
      </li>
    {/each}
  </ul>

  {#if hasMore}
    <button
      class="btn-secondary load-more"
      onclick={() => loadKeys(false)}
      disabled={loading}
    >
      {loading ? 'Loading...' : 'Load more'}
    </button>
  {/if}

  {#if keys.length === 0 && !loading}
    <div class="empty">No keys found</div>
  {/if}
</div>

<style>
  .key-list {
    display: flex;
    flex-direction: column;
    height: 100%;
    padding: 1rem;
    gap: 0.75rem;
  }

  .search {
    display: flex;
    gap: 0.5rem;
  }

  .search input {
    flex: 1;
  }

  .actions {
    display: flex;
    gap: 0.5rem;
  }

  .new-key {
    display: flex;
    gap: 0.5rem;
    padding: 0.5rem;
    background: var(--bg-tertiary);
    border-radius: 4px;
  }

  .new-key input {
    flex: 1;
  }

  .keys {
    flex: 1;
    overflow-y: auto;
    list-style: none;
  }

  .key-item {
    width: 100%;
    text-align: left;
    padding: 0.5rem;
    background: transparent;
    color: var(--text-primary);
    font-family: var(--font-mono);
    font-size: 0.875rem;
    border-radius: 4px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .key-item:hover {
    background: var(--bg-tertiary);
  }

  .key-item.selected {
    background: var(--accent);
  }

  .load-more {
    width: 100%;
  }

  .empty {
    text-align: center;
    color: var(--text-secondary);
    padding: 2rem;
  }
</style>
