<script lang="ts">
	import Logo from '$lib/components/Logo.svelte';
	import * as Dialog from '$lib/components/ui/dialog';
	import Separator from '$lib/components/ui/separator/separator.svelte';
	import { ExternalLink } from '@lucide/svelte/icons';

	const REPO = 'https://github.com/natrimmer/kvweb';

	interface Props {
		open: boolean;
		version: string;
		commit: string;
		dirty: boolean;
	}

	let { open = $bindable(), version, commit, dirty }: Props = $props();
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="max-w-sm">
		<Dialog.Header class="mb-4">
			<div class="flex items-center gap-2">
				<Logo size={32} class="text-primary" />
				<Dialog.Title class="text-xl">kvweb</Dialog.Title>
			</div>
			<Dialog.Description>
				A web-based GUI for browsing and editing Valkey/Redis databases.
			</Dialog.Description>
		</Dialog.Header>

		<div class="flex flex-col gap-4 text-sm">
			<div class="flex flex-col gap-1.5">
				<a
					href="https://kvweb.dev"
					target="_blank"
					rel="noopener"
					class="flex items-center gap-2 text-muted-foreground hover:text-foreground"
				>
					<ExternalLink class="size-3.5" />
					kvweb.dev
				</a>
				<a
					href={REPO}
					target="_blank"
					rel="noopener"
					class="flex items-center gap-2 text-muted-foreground hover:text-foreground"
				>
					<ExternalLink class="size-3.5" />
					GitHub
				</a>
			</div>
			<Separator />
			<div class="text-right text-xs text-muted-foreground">
				{#if version && version !== 'dev'}
					<a
						href="{REPO}/releases/tag/v{version}"
						target="_blank"
						rel="noopener"
						class="hover:text-foreground">{version}</a
					>
				{:else}
					{version || 'unknown'}
				{/if}
				{#if commit}
					<span>∘</span>
					<a
						href="{REPO}/commit/{commit}"
						target="_blank"
						rel="noopener"
						class="font-mono hover:text-foreground">{commit}</a
					>
				{/if}
				{#if dirty}
					<span>∘</span>
					<em>dirty</em>
				{/if}
			</div>
		</div>
	</Dialog.Content>
</Dialog.Root>
