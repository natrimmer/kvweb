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

<div class="app">
  <header>
    <h1>kvweb</h1>
    <nav>
      <button
        class:active={view === 'keys'}
        onclick={() => view = 'keys'}
      >
        Keys
      </button>
      <button
        class:active={view === 'info'}
        onclick={() => view = 'info'}
      >
        Server Info
      </button>
    </nav>
    <div class="status">
      {#if prefix}
        <span class="prefix-badge">{prefix}*</span>
      {/if}
      {#if readOnly}
        <span class="readonly-badge">READ-ONLY</span>
      {/if}
      <span class="indicator" class:connected></span>
      {connected ? `${dbSize} keys` : 'Disconnected'}
    </div>
  </header>

  <main>
    {#if view === 'keys'}
      <div class="keys-view">
        <aside>
          <KeyList
            onselect={handleKeySelect}
            selected={selectedKey}
            oncreated={handleKeyCreated}
            {readOnly}
            {prefix}
          />
        </aside>
        <section class="editor">
          {#if selectedKey}
            <KeyEditor
              key={selectedKey}
              ondeleted={handleKeyDeleted}
              {readOnly}
            />
          {:else}
            <div class="placeholder">
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

<style>
  .app {
    display: flex;
    flex-direction: column;
    height: 100vh;
  }

  header {
    display: flex;
    align-items: center;
    gap: 2rem;
    padding: 1rem 1.5rem;
    background: var(--bg-secondary);
    border-bottom: 1px solid var(--border);
  }

  h1 {
    font-size: 1.5rem;
    font-weight: 600;
    color: var(--accent);
  }

  nav {
    display: flex;
    gap: 0.5rem;
  }

  nav button {
    background: transparent;
    color: var(--text-secondary);
    padding: 0.5rem 1rem;
  }

  nav button:hover {
    color: var(--text-primary);
  }

  nav button.active {
    color: var(--accent);
    border-bottom: 2px solid var(--accent);
  }

  .status {
    margin-left: auto;
    display: flex;
    align-items: center;
    gap: 0.5rem;
    color: var(--text-secondary);
    font-size: 0.875rem;
  }

  .indicator {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: #dc2626;
  }

  .indicator.connected {
    background: var(--success);
  }

  .readonly-badge {
    padding: 0.25rem 0.5rem;
    background: #b45309;
    color: white;
    border-radius: 4px;
    font-size: 0.75rem;
    font-weight: 600;
  }

  .prefix-badge {
    padding: 0.25rem 0.5rem;
    background: var(--bg-tertiary);
    color: var(--text-secondary);
    border-radius: 4px;
    font-size: 0.75rem;
    font-family: var(--font-mono);
  }

  main {
    flex: 1;
    overflow: hidden;
  }

  .keys-view {
    display: flex;
    height: 100%;
  }

  aside {
    width: 320px;
    border-right: 1px solid var(--border);
    overflow: hidden;
    display: flex;
    flex-direction: column;
  }

  .editor {
    flex: 1;
    overflow: auto;
  }

  .placeholder {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 100%;
    color: var(--text-secondary);
  }
</style>
