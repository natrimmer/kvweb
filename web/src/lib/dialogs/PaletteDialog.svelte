<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as ButtonGroup from '$lib/components/ui/button-group';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Moon, Sun } from '@lucide/svelte';
	import { mode, toggleMode } from 'mode-watcher';
	import { onMount } from 'svelte';

	interface Props {
		open: boolean;
	}

	let { open = $bindable() }: Props = $props();

	// All semantic tokens to resolve from CSS custom properties
	const semanticTokens = [
		'background',
		'foreground',
		'card',
		'card-foreground',
		'popover',
		'popover-foreground',
		'primary',
		'primary-foreground',
		'secondary',
		'secondary-foreground',
		'muted',
		'muted-foreground',
		'accent',
		'accent-foreground',
		'destructive',
		'destructive-foreground',
		'border',
		'input',
		'ring',
		'sidebar',
		'sidebar-foreground',
		'sidebar-accent',
		'sidebar-accent-foreground',
		'sidebar-border',
		'sidebar-ring',
		'sidebar-primary',
		'sidebar-primary-foreground'
	];

	// Palette names and tones — hex values read from CSS custom properties at runtime
	const paletteNames = ['black', 'crayola-blue', 'scarlet-rush', 'golden-pollen', 'alabaster-grey'];
	const tones = [50, 100, 200, 300, 400, 500, 600, 700, 800, 900, 950];

	let semanticColors = $state<Record<string, string>>({});
	let paletteColors = $state<Record<string, string>>({});

	// Reverse lookup: hex → semantic token names using it
	let hexToTokens = $derived.by(() => {
		const map: Record<string, string[]> = {};
		for (const [token, hex] of Object.entries(semanticColors)) {
			const lower = hex.toLowerCase();
			if (!map[lower]) map[lower] = [];
			map[lower].push(token);
		}
		return map;
	});

	function rgbToHex(rgb: string): string {
		const match = rgb.match(/rgb\((\d+),\s*(\d+),\s*(\d+)\)/);
		if (!match) return rgb;
		const r = parseInt(match[1]).toString(16).padStart(2, '0');
		const g = parseInt(match[2]).toString(16).padStart(2, '0');
		const b = parseInt(match[3]).toString(16).padStart(2, '0');
		return `#${r}${g}${b}`;
	}

	function resolveVar(style: CSSStyleDeclaration, name: string): string {
		const raw = style.getPropertyValue(name).trim();
		return raw.startsWith('#') ? raw : rgbToHex(raw);
	}

	function readColors() {
		const style = getComputedStyle(document.documentElement);

		const sc: Record<string, string> = {};
		for (const token of semanticTokens) {
			sc[token] = resolveVar(style, `--${token}`);
		}
		semanticColors = sc;

		const pc: Record<string, string> = {};
		for (const name of paletteNames) {
			for (const tone of tones) {
				const key = `${name}-${tone}`;
				pc[key] = resolveVar(style, `--color-${key}`);
			}
		}
		paletteColors = pc;
	}

	let isDark = $derived(mode.current === 'dark');

	onMount(() => {
		readColors();
		const observer = new MutationObserver(() => readColors());
		observer.observe(document.documentElement, {
			attributes: true,
			attributeFilter: ['class']
		});
		return () => observer.disconnect();
	});
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="flex max-h-[85vh] flex-col overflow-y-auto p-8">
		<Dialog.Header>
			<Dialog.Title>Color Palette</Dialog.Title>
		</Dialog.Header>

		<div class="min-h-0 flex-1 space-y-4">
			{#each paletteNames as name}
				<div>
					<div class="mb-1 font-mono text-xs text-muted-foreground">{name}</div>
					<div class="flex gap-1">
						{#each tones as tone}
							{@const hex = paletteColors[`${name}-${tone}`] ?? ''}
							{@const usedBy = hexToTokens[hex.toLowerCase()] ?? []}
							<div class="flex flex-1 flex-col items-center gap-0.5">
								<div
									class="h-8 w-full rounded-sm {usedBy.length > 0
										? 'ring-2 ring-ring ring-offset-1 ring-offset-background'
										: ''}"
									style="background-color: {hex}"
									title="{name}-{tone}: {hex}{usedBy.length > 0 ? `\n${usedBy.join('\n')}` : ''}"
								></div>
								<span class="text-[9px] text-muted-foreground">{tone}</span>
								<span class="font-mono text-[8px] text-muted-foreground">{hex}</span>
							</div>
						{/each}
					</div>
				</div>
			{/each}
		</div>
		<div class="flex justify-end">
			<ButtonGroup.Root>
				<Button
					size="sm"
					variant="outline"
					onclick={() => isDark && toggleMode()}
					class={!isDark ? 'bg-accent' : ''}
					title="Switch to light theme"
					aria-label="Switch to light theme"
				>
					<Sun class="h-4 w-4" />
				</Button>
				<Button
					size="sm"
					variant="outline"
					onclick={() => !isDark && toggleMode()}
					class={isDark ? 'bg-accent' : ''}
					title="Switch to dark theme"
					aria-label="Switch to dark theme"
				>
					<Moon class="h-4 w-4" />
				</Button>
			</ButtonGroup.Root>
		</div>
	</Dialog.Content>
</Dialog.Root>
