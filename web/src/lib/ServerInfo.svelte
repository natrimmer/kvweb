<script lang="ts">
  import { Button } from '$lib/components/ui/button';
  import * as Select from '$lib/components/ui/select';
  import { onMount } from 'svelte';
  import { api } from './api';
  import { ws, type Stats, type Status } from './ws';

  interface Props {
    readOnly: boolean
    disableFlush: boolean
  }

  let { readOnly, disableFlush }: Props = $props()

  let info = $state('')
  let loading = $state(false)
  let section = $state('')
  let notificationsEnabled = $state(false)
  let enablingNotifications = $state(false)

  const sections = [
    { value: '', label: 'All Sections' },
    { value: 'server', label: 'Server' },
    { value: 'clients', label: 'Clients' },
    { value: 'memory', label: 'Memory' },
    { value: 'stats', label: 'Stats' },
    { value: 'replication', label: 'Replication' },
    { value: 'cpu', label: 'CPU' },
    { value: 'keyspace', label: 'Keyspace' }
  ]

  const selectedLabel = $derived(
    sections.find((s) => s.value === section)?.label ?? 'All Sections'
  )

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

    // Load initial notifications status
    api.getNotifications().then((result) => {
      notificationsEnabled = result.enabled
    }).catch(() => {
      // Ignore if endpoint not available
    })

    // Subscribe to status updates for immediate notification changes
    const unsubStatus = ws.onStatus((status: Status) => {
      notificationsEnabled = status.live
    })

    // Subscribe to stats updates for live notifications status
    const unsubStats = ws.onStats((stats: Stats) => {
      notificationsEnabled = stats.notificationsOn
    })

    return () => {
      unsubStatus()
      unsubStats()
    }
  })

  function handleValueChange(value: string | undefined) {
    if (value !== undefined) {
      section = value
      loadInfo()
    }
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

  async function enableNotifications() {
    enablingNotifications = true
    try {
      const result = await api.setNotifications(true)
      notificationsEnabled = result.enabled
      if (result.enabled) {
        alert('Live updates enabled. The server will now broadcast key changes.')
      }
    } catch (e) {
      alert('Failed to enable notifications')
    } finally {
      enablingNotifications = false
    }
  }
</script>

<div class="p-6 h-full flex flex-col gap-4">
  <div class="flex gap-2">
    <Select.Root type="single" bind:value={section} onValueChange={handleValueChange}>
      <Select.Trigger class="w-50">
        {selectedLabel}
      </Select.Trigger>
      <Select.Content>
        {#each sections as sect}
          <Select.Item value={sect.value}>{sect.label}</Select.Item>
        {/each}
      </Select.Content>
    </Select.Root>
    <Button variant="secondary" onclick={loadInfo} disabled={loading}>
      Refresh
    </Button>
    {#if !readOnly && !disableFlush}
      <Button variant="destructive" onclick={flushDb}>
        Flush Database
      </Button>
    {/if}
    {#if !readOnly && !notificationsEnabled}
      <Button variant="secondary" onclick={enableNotifications} disabled={enablingNotifications}>
        {enablingNotifications ? 'Enabling...' : 'Enable Live Updates'}
      </Button>
    {/if}
  </div>

  <pre class="flex-1 bg-alabaster-grey-50 p-4 rounded overflow-auto font-mono text-sm whitespace-pre-wrap break-all">{loading ? 'Loading...' : info}</pre>
</div>
