<script lang="ts">
  import { Badge } from '$lib/components/ui/badge';
  import { Button } from '$lib/components/ui/button';
  import { Input } from '$lib/components/ui/input';
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
  let debounceTimer: ReturnType<typeof setTimeout> | null = null
  let showHistory = $state(false)
  let searchHistory = $state<string[]>([])

  const HISTORY_KEY = 'kvweb:search-history'
  const MAX_HISTORY = 20

  function loadHistory() {
    try {
      const stored = localStorage.getItem(HISTORY_KEY)
      searchHistory = stored ? JSON.parse(stored) : []
    } catch {
      searchHistory = []
    }
  }

  function saveHistory() {
    localStorage.setItem(HISTORY_KEY, JSON.stringify(searchHistory))
  }

  function addToHistory(p: string) {
    if (!p || p === '*') return
    searchHistory = [p, ...searchHistory.filter(h => h !== p)].slice(0, MAX_HISTORY)
    saveHistory()
  }

  function removeFromHistory(p: string) {
    searchHistory = searchHistory.filter(h => h !== p)
    saveHistory()
  }

  function clearHistory() {
    searchHistory = []
    saveHistory()
  }

  function selectHistory(p: string) {
    pattern = p
    showHistory = false
  }

  // Load history on init
  loadHistory()

  // Debounced search when pattern changes
  $effect(() => {
    pattern  // track dependency
    if (debounceTimer) clearTimeout(debounceTimer)
    debounceTimer = setTimeout(() => {
      loadKeys(true)
      addToHistory(pattern)
    }, 300)
    return () => {
      if (debounceTimer) clearTimeout(debounceTimer)
    }
  })

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
</script>

<div class="flex flex-col h-full p-4 gap-3">
  <div class="relative">
    <Input
      type="text"
      bind:value={pattern}
      placeholder="Pattern (e.g., user:*)"
      onfocus={() => showHistory = true}
      onblur={() => setTimeout(() => showHistory = false, 150)}
    />
    {#if showHistory && searchHistory.length > 0}
      <div class="absolute top-full left-0 right-0 mt-1 bg-white border border-alabaster-grey-200 rounded shadow-lg z-10 max-h-60 overflow-auto">
        <div class="flex items-center justify-between px-3 py-2 border-b border-alabaster-grey-100">
          <span class="text-xs text-black-400">Recent searches</span>
          <button
            type="button"
            class="text-xs text-black-400 hover:text-scarlet-rush-500"
            onmousedown={() => clearHistory()}
          >
            Clear all
          </button>
        </div>
        {#each searchHistory as h}
          <div class="flex items-center group hover:bg-alabaster-grey-50">
            <button
              type="button"
              class="flex-1 px-3 py-2 text-left font-mono text-sm"
              onmousedown={() => selectHistory(h)}
            >
              {h}
            </button>
            <button
              type="button"
              class="px-2 py-1 text-black-300 hover:text-scarlet-rush-500 opacity-0 group-hover:opacity-100"
              onmousedown={() => removeFromHistory(h)}
            >
              ×
            </button>
          </div>
        {/each}
      </div>
    {/if}
  </div>

  {#if !readOnly}
    <div class="flex gap-2">
      <Button variant="secondary" onclick={() => showNewKey = !showNewKey}>
        + New Key
      </Button>
    </div>
  {/if}

  {#if showNewKey && !readOnly}
    <div class="flex gap-2 p-2 bg-black-800 rounded">
      {#if prefix}
        <Badge variant="secondary" class="text-black-400 font-mono">{prefix}</Badge>
      {/if}
      <Input
        type="text"
        bind:value={newKeyName}
        placeholder="Key name"
        onkeydown={(e) => e.key === 'Enter' && createKey()}
        class="flex-1"
      />
      <Button onclick={createKey}>Create</Button>
    </div>
  {/if}

  <ul class="flex-1 overflow-y-auto list-none">
    {#each keys as key (key)}
      <li>
        <Button
          variant="ghost"
          class="w-full justify-start p-2 text-black-950 font-mono text-sm rounded overflow-hidden text-ellipsis whitespace-nowrap hover:bg-crayola-blue-200 {key === selected ? 'bg-crayola-blue-100 hover:bg-crayola-blue-100' : ''}"
          onclick={() => onselect(key)}
        >
          {key}
        </Button>
      </li>
    {/each}
  </ul>

  {#if hasMore}
    <Button
      variant="secondary"
      class="w-full"
      onclick={() => loadKeys(false)}
      disabled={loading}
    >
      {loading ? 'Loading...' : 'Load more'}
    </Button>
  {/if}

  {#if keys.length === 0 && !loading}
    <div class="text-center text-black-400 py-8">No keys found</div>
  {/if}
</div>
