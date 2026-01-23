<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from './api';

  interface Props {
    readOnly: boolean
    disableFlush: boolean
  }

  let { readOnly, disableFlush }: Props = $props()

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

<div class="p-6 h-full flex flex-col gap-4">
  <div class="flex gap-2">
    <select bind:value={section} onchange={handleSectionChange} class="bg-black-900 border border-black-700 text-black-100 p-2 rounded">
      <option value="">All Sections</option>
      {#each sections.slice(1) as s}
        <option value={s}>{s}</option>
      {/each}
    </select>
    <button class="btn-secondary" onclick={loadInfo} disabled={loading}>
      Refresh
    </button>
    {#if !readOnly && !disableFlush}
      <button class="btn-danger" onclick={flushDb}>
        Flush Database
      </button>
    {/if}
  </div>

  <pre class="flex-1 bg-alabaster-grey-50 p-4 rounded overflow-auto font-mono text-sm whitespace-pre-wrap break-all">{loading ? 'Loading...' : info}</pre>
</div>
