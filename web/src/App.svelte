<script lang="ts">
  import { Badge } from '$lib/components/ui/badge';
  import { Button } from '$lib/components/ui/button';
  import * as Empty from '$lib/components/ui/empty';
  import * as Resizable from '$lib/components/ui/resizable';
  import { Toaster } from '$lib/components/ui/sonner';
  import DatabaseIcon from '@lucide/svelte/icons/database';
  import { onMount } from 'svelte';
  import KeyEditor from './lib/KeyEditor.svelte';
  import KeyList from './lib/KeyList.svelte';
  import ServerInfo from './lib/ServerInfo.svelte';
  import { api } from './lib/api';
  import { ws } from './lib/ws';

  let selectedKey = $state<string | null>(null)
  let view = $state<'keys' | 'info'>('keys')
  let dbSize = $state(0)
  let connected = $state(false)
  let readOnly = $state(false)
  let prefix = $state('')
  let disableFlush = $state(false)
  let liveUpdates = $state(false)

  onMount(() => {
    // Load initial data
    Promise.all([
      api.getInfo(),
      api.getConfig()
    ]).then(([info, config]) => {
      dbSize = info.dbSize
      readOnly = config.readOnly
      prefix = config.prefix
      disableFlush = config.disableFlush
      connected = true
    }).catch(() => {
      connected = false
    })

    // Connect WebSocket
    ws.connect()

    ws.onStatus((status) => {
      liveUpdates = status.live
    })

    ws.onStats((stats) => {
      dbSize = stats.dbSize
      liveUpdates = stats.notificationsOn
    })

    return () => ws.disconnect()
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
  <header class="flex items-center gap-8 px-6 py-4 bg-crayola-blue-50 border-b border-black-700">
    <h1 class="text-2xl font-semibold text-crayola-blue-400">kvweb</h1>
    <nav class="flex gap-2">
      <Button
        variant="outline"
        class="text-black-400 hover:text-crayola-blue-400 hover:border-crayola-blue-400 rounded-none {view === 'keys' ? 'text-crayola-blue-400 border-b-2 border-crayola-blue-400' : ''}"
        onclick={() => view = 'keys'}
      >
        Keys
      </Button>
      <Button
        variant="outline"
        class="text-black-400 hover:text-crayola-blue-400 hover:border-crayola-blue-400 rounded-none {view === 'info' ? 'text-crayola-blue-400 border-b-2 border-crayola-blue-400' : ''}"
        onclick={() => view = 'info'}
      >
        Server Info
      </Button>
    </nav>
    <div class="ml-auto flex items-center gap-3 text-black-400 text-sm">
      {#if liveUpdates}
        <span class="flex items-center gap-1.5 text-crayola-blue-500">
          <span class="w-2 h-2 rounded-full bg-crayola-blue-500 animate-pulse"></span>
          Live
        </span>
      {/if}
      {#if prefix}
        <Badge variant="secondary" class="bg-black-800 text-black-300 font-mono">{prefix}*</Badge>
      {/if}
      {#if readOnly}
        <Badge class="bg-golden-pollen-600 text-white hover:bg-golden-pollen-600">READ-ONLY</Badge>
      {/if}
      <span class="w-2 h-2 rounded-full {connected ? 'bg-crayola-blue-500' : 'bg-scarlet-rush-500'}"></span>
      {connected ? `${dbSize} keys` : 'Disconnected'}
    </div>
  </header>

  <main class="flex-1 overflow-hidden">
    {#if view === 'keys'}
      <Resizable.PaneGroup direction="horizontal" class="h-full">
        <Resizable.Pane defaultSize={25} minSize={15} maxSize={50}>
          <div class="h-full overflow-hidden flex flex-col border-r border-black-700">
            <KeyList
              onselect={handleKeySelect}
              selected={selectedKey}
              oncreated={handleKeyCreated}
              {readOnly}
              {prefix}
            />
          </div>
        </Resizable.Pane>
        <Resizable.Handle withHandle />
        <Resizable.Pane defaultSize={75}>
          <div class="h-full overflow-auto">
            {#if selectedKey}
              <KeyEditor
                key={selectedKey}
                ondeleted={handleKeyDeleted}
                {readOnly}
              />
            {:else}
              <Empty.Root class="h-full">
                <Empty.Header>
                  <Empty.Media variant="icon">
                    <DatabaseIcon />
                  </Empty.Media>
                  <Empty.Title>No Key Selected</Empty.Title>
                  <Empty.Description>
                    Select a key from the list to view or edit its value.
                  </Empty.Description>
                </Empty.Header>
              </Empty.Root>
            {/if}
          </div>
        </Resizable.Pane>
      </Resizable.PaneGroup>
    {:else}
      <ServerInfo {readOnly} {disableFlush} />
    {/if}
  </main>
</div>

<Toaster />
