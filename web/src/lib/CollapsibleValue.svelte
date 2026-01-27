<script lang="ts">
  interface Props {
    value: string
    maxLength?: number
    highlight?: string  // Pre-rendered HTML for JSON highlighting
  }

  let { value, maxLength = 200, highlight }: Props = $props()
  
  let expanded = $state(false)
  
  let needsCollapse = $derived(value.length > maxLength)
  let displayValue = $derived(
    needsCollapse && !expanded 
      ? value.slice(0, maxLength) 
      : value
  )
</script>

{#if highlight && expanded}
  <div class="[&>pre]:p-0 [&>pre]:m-0 [&>pre]:bg-transparent [&>pre]:text-sm">
    {@html highlight}
  </div>
{:else}
  <span class="break-all">{displayValue}{#if needsCollapse && !expanded}…{/if}</span>
{/if}

{#if needsCollapse}
  <button
    type="button"
    onclick={() => expanded = !expanded}
    class="ml-1 text-xs text-crayola-blue-500 hover:text-crayola-blue-700 hover:underline"
  >
    {expanded ? 'Show less' : `Show all (${value.length} chars)`}
  </button>
{/if}
