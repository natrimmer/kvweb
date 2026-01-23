<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from './api';

  interface Props {
    selected: string | null
    onselect: (key: string) => void
    oncreated: () => void
    readOnly: boolean
    prefix: string
  }

  let { selected, onselect, oncreated, readOnly, prefix }: Props = $props()

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
      const fullKeyName = prefix + newKeyName
      await api.setKey(fullKeyName, '')
      newKeyName = ''
      showNewKey = false
      await loadKeys(true)
      onselect(fullKeyName)
      oncreated()
    } catch (e) {
      console.error('Failed to create key:', e)
    }
  }

  onMount(() => {
    loadKeys(true)
  })
</script>

<div class="flex flex-col h-full p-4 gap-3">
  <div class="flex gap-2">
    <input
      type="text"
      bind:value={pattern}
      placeholder="Pattern (e.g., user:*)"
      onkeydown={(e) => e.key === 'Enter' && handleSearch()}
      class="flex-1"
    />
    <button class="btn-primary" onclick={handleSearch} disabled={loading}>
      Search
    </button>
  </div>

  {#if !readOnly}
    <div class="flex gap-2">
      <button class="btn-secondary" onclick={() => showNewKey = !showNewKey}>
        + New Key
      </button>
    </div>
  {/if}

  {#if showNewKey && !readOnly}
    <div class="flex gap-2 p-2 bg-black-800 rounded">
      {#if prefix}
        <span class="text-black-400 font-mono text-sm">{prefix}</span>
      {/if}
      <input
        type="text"
        bind:value={newKeyName}
        placeholder="Key name"
        onkeydown={(e) => e.key === 'Enter' && createKey()}
        class="flex-1"
      />
      <button class="btn-primary" onclick={createKey}>Create</button>
    </div>
  {/if}

  <ul class="flex-1 overflow-y-auto list-none">
    {#each keys as key (key)}
      <li>
        <button
          class="w-full text-left p-2 bg-transparent text-black-100 font-mono text-sm rounded overflow-hidden text-ellipsis whitespace-nowrap hover:bg-black-800 {key === selected ? 'bg-crayola-blue-600' : ''}"
          onclick={() => onselect(key)}
        >
          {key}
        </button>
      </li>
    {/each}
  </ul>

  {#if hasMore}
    <button
      class="btn-secondary w-full"
      onclick={() => loadKeys(false)}
      disabled={loading}
    >
      {loading ? 'Loading...' : 'Load more'}
    </button>
  {/if}

  {#if keys.length === 0 && !loading}
    <div class="text-center text-black-400 py-8">No keys found</div>
  {/if}
</div>
