<script lang="ts">
  import * as AlertDialog from '$lib/components/ui/alert-dialog';
  import { Button } from '$lib/components/ui/button';
  import * as Select from '$lib/components/ui/select';
  import Info from "@lucide/svelte/icons/info";
  import { onMount } from 'svelte';
  import { toast } from 'svelte-sonner';
  import { api } from './api';
  import { toastError } from './utils';
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
  let flushDialogOpen = $state(false)

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
    try {
      await api.flushDb()
      toast.success('Database flushed')
    } catch (e) {
      toastError(e, 'Failed to flush database')
    } finally {
      flushDialogOpen = false
    }
  }

  async function enableNotifications() {
    enablingNotifications = true
    try {
      const result = await api.setNotifications(true)
      notificationsEnabled = result.enabled
      if (result.enabled) {
        toast.success('Live updates enabled')
      }
    } catch (e) {
      toastError(e, 'Failed to enable notifications')
    } finally {
      enablingNotifications = false
    }
  }
</script>

<div class="p-6 h-full flex flex-col gap-4">
    <div class="flex justify-between items-center">
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
            <Button variant="secondary" onclick={loadInfo} disabled={loading} class="cursor-pointer" title="Refresh server info">
            Refresh
            </Button>
            {#if !readOnly && !disableFlush}
            <AlertDialog.Root bind:open={flushDialogOpen}>
                <AlertDialog.Trigger>
                {#snippet child({ props })}
                    <Button variant="destructive" {...props} class="cursor-pointer" title="Delete all keys in database">Flush Database</Button>
                {/snippet}
                </AlertDialog.Trigger>
                <AlertDialog.Content>
                <AlertDialog.Header>
                    <AlertDialog.Title>Flush Database</AlertDialog.Title>
                    <AlertDialog.Description>
                    This will delete ALL keys in the current database. This action cannot be undone.
                    </AlertDialog.Description>
                </AlertDialog.Header>
                <AlertDialog.Footer>
                    <AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
                    <AlertDialog.Action onclick={flushDb}>Flush Database</AlertDialog.Action>
                </AlertDialog.Footer>
                </AlertDialog.Content>
            </AlertDialog.Root>
            {/if}
            {#if !readOnly && !notificationsEnabled}
            <Button variant="secondary" onclick={enableNotifications} disabled={enablingNotifications} class="cursor-pointer" title="Enable real-time key change notifications">
                {enablingNotifications ? 'Enabling...' : 'Enable Live Updates'}
            </Button>
            {/if}
        </div>
        <a
          href="/kvweb"
          class="text-sm text-muted-foreground flex gap-2 justify-between items-center hover:underline"
        >
          <span>learn more about kvweb</span>
          <Info size={20} />
        </a>
    </div>

  <pre class="flex-1 bg-muted p-4 rounded overflow-auto font-mono text-sm whitespace-pre-wrap break-all">{loading ? 'Loading...' : info}</pre>
</div>
