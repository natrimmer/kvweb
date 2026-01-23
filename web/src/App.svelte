<script lang="ts">
  import { onMount } from 'svelte';
  import KeyEditor from './lib/KeyEditor.svelte';
  import KeyList from './lib/KeyList.svelte';
  import ServerInfo from './lib/ServerInfo.svelte';
  import { api } from './lib/api';

  let selectedKey = $state<string | null>(null)
  let view = $state<'keys' | 'info'>('keys')
  let dbSize = $state(0)
  let connected = $state(false)
  let readOnly = $state(false)
  let prefix = $state('')
  let disableFlush = $state(false)

  onMount(async () => {
    try {
      const [info, config] = await Promise.all([
        api.getInfo(),
        api.getConfig()
      ])
      dbSize = info.dbSize
      readOnly = config.readOnly
      prefix = config.prefix
      disableFlush = config.disableFlush
      connected = true
    } catch (e) {
      connected = false
    }
  })

  function handleKeySelect(key: string) {
    selectedKey = key
  }

  function handleKeyDeleted() {
    selectedKey = null
    dbSize = Math.max(0, dbSize - 1)
  }

  function handleKeyCreated() {
    dbSize += 1
  }
</script>

<div class="flex flex-col h-screen">
  <header class="flex items-center gap-8 px-6 py-4 bg-alabaster-grey-50 border-b border-black-700">
    <h1 class="text-2xl font-semibold text-crayola-blue-400">kvweb</h1>
    <nav class="flex gap-2">
      <button
        class="bg-transparent text-black-400 hover:text-black-100 {view === 'keys' ? 'text-crayola-blue-400 border-b-2 border-crayola-blue-400' : ''}"
        onclick={() => view = 'keys'}
      >
        Keys
      </button>
      <button
        class="bg-transparent text-black-400 hover:text-black-100 {view === 'info' ? 'text-crayola-blue-400 border-b-2 border-crayola-blue-400' : ''}"
        onclick={() => view = 'info'}
      >
        Server Info
      </button>
    </nav>
    <div class="ml-auto flex items-center gap-2 text-black-400 text-sm">
      {#if prefix}
        <span class="px-2 py-1 bg-black-800 text-black-300 rounded text-xs font-mono">{prefix}*</span>
      {/if}
      {#if readOnly}
        <span class="px-2 py-1 bg-golden-pollen-600 text-white rounded text-xs font-semibold">READ-ONLY</span>
      {/if}
      <span class="w-2 h-2 rounded-full {connected ? 'bg-crayola-blue-500' : 'bg-scarlet-rush-500'}"></span>
      {connected ? `${dbSize} keys` : 'Disconnected'}
    </div>
  </header>

  <main class="flex-1 overflow-hidden">
    {#if view === 'keys'}
      <div class="flex h-full">
        <aside class="w-80 border-r border-black-700 overflow-hidden flex flex-col">
          <KeyList
            onselect={handleKeySelect}
            selected={selectedKey}
            oncreated={handleKeyCreated}
            {readOnly}
            {prefix}
          />
        </aside>
        <section class="flex-1 overflow-auto">
          {#if selectedKey}
            <KeyEditor
              key={selectedKey}
              ondeleted={handleKeyDeleted}
              {readOnly}
            />
          {:else}
            <div class="flex items-center justify-center h-full text-black-400">
              Select a key to view/edit its value
            </div>
          {/if}
        </section>
      </div>
    {:else}
      <ServerInfo {readOnly} {disableFlush} />
    {/if}
  </main>
</div>
