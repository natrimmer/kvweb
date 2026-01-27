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

{#if highlight}
  {#if expanded}
    <div class="[&>pre]:p-0 [&>pre]:m-0 [&>pre]:bg-transparent [&>pre]:text-sm">
      {@html highlight}
    </div>
  {:else}
    <div class="[&>pre]:p-0 [&>pre]:m-0 [&>pre]:bg-transparent [&>pre]:text-sm [&>pre]:overflow-hidden [&>pre]:text-ellipsis [&>pre]:whitespace-nowrap">
      {@html highlight}
    </div>
  {/if}
{:else}
  <span class="break-all">{displayValue}{#if needsCollapse && !expanded}…{/if}</span>
{/if}

{#if needsCollapse}
  <button
    type="button"
    onclick={() => expanded = !expanded}
    class="ml-1 text-xs text-primary hover:text-primary/80 hover:underline cursor-pointer"
    title={expanded ? 'Collapse value' : `Expand to show full value (${value.length} characters)`}
  >
    {expanded ? 'Show less' : `Show all (${value.length} chars)`}
  </button>
{/if}
