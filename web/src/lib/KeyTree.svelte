<script lang="ts">
  import { Badge } from '$lib/components/ui/badge';
  import { Button } from '$lib/components/ui/button';
  import { Dot, Folder, House, List, MoveLeft } from '@lucide/svelte';
  import { api, type PrefixEntry } from './api';
  import { getErrorMessage } from './utils';

  interface Props {
    selected: string | null
    onselect: (key: string) => void
    onclose: () => void
  }

  let { selected, onselect, onclose }: Props = $props()

  let entries = $state<PrefixEntry[]>([])
  let loading = $state(false)
  let currentPrefix = $state('')
  let prefixStack = $state<string[]>([])
  let error = $state('')

  async function loadPrefixes(prefix: string) {
    loading = true
    error = ''
    try {
      const result = await api.getPrefixes(prefix)
      entries = result.entries
      currentPrefix = prefix
    } catch (e) {
      error = getErrorMessage(e, 'Failed to load')
      entries = []
    } finally {
      loading = false
    }
  }

  function navigateTo(prefix: string) {
    prefixStack = [...prefixStack, currentPrefix]
    loadPrefixes(prefix)
  }

  function navigateBack() {
    const prev = prefixStack.pop()
    prefixStack = prefixStack
    loadPrefixes(prev ?? '')
  }

  function navigateToRoot() {
    prefixStack = []
    loadPrefixes('')
  }

  function handleClick(entry: PrefixEntry) {
    if (entry.isLeaf && entry.fullKey) {
      onselect(entry.fullKey)
    } else {
      navigateTo(entry.prefix)
    }
  }

  function displayName(entry: PrefixEntry): string {
    // Show just the last segment
    const name = entry.prefix.slice(currentPrefix.length)
    return name || entry.prefix
  }

  // Load root on mount
  loadPrefixes('')
</script>

<div class="flex flex-col h-full p-4 gap-3">
  <div class="flex items-center gap-2">
    {#if currentPrefix}
      <Button variant="ghost" size="sm" onclick={navigateBack} class="px-2">
        <MoveLeft size={16}/>
      </Button>
      <Button variant="ghost" size="sm" onclick={navigateToRoot} class="px-2">
        <House size={16}/>
      </Button>
      <span class="font-mono text-sm text-black-600 truncate flex-1">{currentPrefix}</span>
    {:else}
      <span class="text-sm text-black-600 flex-1">Tree View</span>
    {/if}
    <button
      type="button"
      onclick={onclose}
      class="px-3 py-1 border border-alabaster-grey-200 rounded text-sm bg-white hover:bg-alabaster-grey-50 font-mono"
      title="Switch to list view"
    >
      <List size={18}/>
    </button>
  </div>

  {#if loading}
    <div class="flex items-center justify-center flex-1 text-black-400">Loading...</div>
  {:else if error}
    <div class="flex items-center justify-center flex-1 text-scarlet-rush-400">{error}</div>
  {:else}
    <ul class="flex-1 overflow-y-auto list-none">
      {#each entries as entry (entry.prefix)}
        <li>
          <Button
            variant="ghost"
            class="w-full justify-start p-2 text-black-950 font-mono text-sm rounded hover:bg-crayola-blue-200 {entry.isLeaf && entry.fullKey === selected ? 'bg-crayola-blue-100 hover:bg-crayola-blue-100' : ''}"
            onclick={() => handleClick(entry)}
          >
            <span class="text-black-400">
                {#if entry.isLeaf}
                    <Dot size={16}/>
                {:else if !entry.isLeaf}
                    <Folder size={16}/>
                {/if}
            </span>
            <span class="flex-1 overflow-hidden text-ellipsis text-left">{displayName(entry)}</span>
            {#if entry.isLeaf && entry.type}
              <Badge variant="secondary" class="ml-2 text-xs opacity-60">{entry.type}</Badge>
            {:else if !entry.isLeaf}
              <span class="ml-2 text-xs text-black-400">({entry.count})</span>
            {/if}
          </Button>
        </li>
      {/each}
    </ul>

    {#if entries.length === 0}
      <div class="text-center text-black-400 py-8">No keys found</div>
    {/if}
  {/if}
</div>
