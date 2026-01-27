<script lang="ts">
  import { Badge } from '$lib/components/ui/badge';
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

  function resetToHome() {
    selectedKey = null
    view = 'keys'
  }

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
  <header class="flex items-center gap-6 px-6 py-4 border-b border-border">
    <button
      type="button"
      onclick={resetToHome}
      class="flex items-center gap-2 text-xl font-semibold text-foreground hover:text-primary transition-colors group"
    >
      <svg width="24" height="24" viewBox="0 0 64 64" xmlns="http://www.w3.org/2000/svg" class="text-primary group-hover:text-primary transition-colors">
        <rect x="8" y="8" width="48" height="48" rx="6" fill="none" stroke="currentColor" stroke-width="3"/>
        <line x1="32" y1="8" x2="32" y2="56" stroke="currentColor" stroke-width="3"/>
        <line x1="8" y1="24" x2="56" y2="24" stroke="currentColor" stroke-width="3"/>
        <line x1="8" y1="40" x2="56" y2="40" stroke="currentColor" stroke-width="3"/>
        <rect x="32" y="8" width="24" height="16" fill="currentColor"/>
      </svg>
      kvweb
    </button>

    <nav class="flex gap-1">
      <button
        type="button"
        onclick={() => view = 'keys'}
        class="px-4 py-1.5 text-sm rounded transition-colors {view === 'keys' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:text-foreground hover:bg-muted'}"
      >
        Keys
      </button>
      <button
        type="button"
        onclick={() => view = 'info'}
        class="px-4 py-1.5 text-sm rounded transition-colors {view === 'info' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:text-foreground hover:bg-muted'}"
      >
        Info
      </button>
    </nav>

    <div class="ml-auto flex items-center gap-3 text-sm">
      {#if readOnly}
        <Badge variant="default" class="bg-accent text-accent-foreground hover:bg-accent">READ-ONLY</Badge>
      {/if}
      <div class="flex items-center gap-2 text-muted-foreground">
        <span class="w-2 h-2 rounded-full {connected ? 'bg-primary' : 'bg-destructive'}"></span>
        <span>{connected ? `${dbSize} keys` : 'Disconnected'}</span>
      </div>
    </div>
  </header>

  <main class="flex-1 overflow-hidden">
    {#if view === 'keys'}
      <Resizable.PaneGroup direction="horizontal" class="h-full">
        <Resizable.Pane defaultSize={25} minSize={15} maxSize={50}>
          <div class="h-full overflow-hidden flex flex-col border-r border-border">
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
