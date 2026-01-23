<script lang="ts">
  import { onMount } from 'svelte'
  import { api } from './api'

  let info = $state('')
  let loading = $state(false)
  let section = $state('')

  const sections = ['', 'server', 'clients', 'memory', 'stats', 'replication', 'cpu', 'keyspace']

  async function loadInfo() {
    loading = true
    try {
      const result = await api.getInfo(section)
      info = result.info
    } catch (e) {
      info = 'Failed to load server info'
    } finally {
      loading = false
    }
  }

  onMount(() => {
    loadInfo()
  })

  function handleSectionChange() {
    loadInfo()
  }

  async function flushDb() {
    if (!confirm('Are you sure you want to delete ALL keys in the current database?')) return
    try {
      await api.flushDb()
      alert('Database flushed successfully')
    } catch (e) {
      alert('Failed to flush database')
    }
  }
</script>

<div class="server-info">
  <div class="controls">
    <select bind:value={section} onchange={handleSectionChange}>
      <option value="">All Sections</option>
      {#each sections.slice(1) as s}
        <option value={s}>{s}</option>
      {/each}
    </select>
    <button class="btn-secondary" onclick={loadInfo} disabled={loading}>
      Refresh
    </button>
    <button class="btn-danger" onclick={flushDb}>
      Flush Database
    </button>
  </div>

  <pre class="info-content">{loading ? 'Loading...' : info}</pre>
</div>

<style>
  .server-info {
    padding: 1.5rem;
    height: 100%;
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .controls {
    display: flex;
    gap: 0.5rem;
  }

  select {
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    color: var(--text-primary);
    padding: 0.5rem;
    border-radius: 4px;
  }

  .info-content {
    flex: 1;
    background: var(--bg-secondary);
    padding: 1rem;
    border-radius: 4px;
    overflow: auto;
    font-family: var(--font-mono);
    font-size: 0.875rem;
    white-space: pre-wrap;
    word-break: break-all;
  }
</style>
