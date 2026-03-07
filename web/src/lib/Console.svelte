<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Kbd from '$lib/components/ui/kbd';
	import { Eraser } from '@lucide/svelte/icons';
	import { api, type ExecResult } from './api';

	export interface OutputLine {
		type: 'command' | 'result' | 'error';
		text: string;
	}

	interface Props {
		readOnly: boolean;
		prefix: string;
		outputHistory?: OutputLine[];
		commandHistory?: string[];
	}

	let {
		readOnly,
		prefix,
		outputHistory = $bindable([]),
		commandHistory = $bindable([])
	}: Props = $props();

	const isMac = /mac/i.test(
		(navigator as Navigator & { userAgentData?: { platform: string } }).userAgentData?.platform ??
			navigator.userAgent
	);

	let historyIndex = $state(-1);
	let input = $state('');
	let loading = $state(false);
	let outputEl: HTMLDivElement | undefined = $state();
	let inputEl: HTMLInputElement | undefined = $state();

	function scrollToBottom() {
		if (outputEl) {
			requestAnimationFrame(() => {
				outputEl!.scrollTop = outputEl!.scrollHeight;
			});
		}
	}

	function formatExecResult(result: ExecResult, indent = 0): string {
		const pad = '  '.repeat(indent);
		switch (result.type) {
			case 'string':
				return `${pad}"${result.value}"`;
			case 'integer':
				return `${pad}(integer) ${result.value}`;
			case 'nil':
				return `${pad}(nil)`;
			case 'error':
				return `${pad}(error) ${String(result.value).replace(/, with args beginning with:\s*$/, '')}`;
			case 'array': {
				const items = result.value as ExecResult[];
				if (items.length === 0) return `${pad}(empty array)`;
				return items
					.map((item, i) => {
						const num = `${pad}${i + 1}) `;
						const formatted = formatExecResult(item, indent + 1);
						return num + formatted.trimStart();
					})
					.join('\n');
			}
			default:
				return `${pad}${result.value}`;
		}
	}

	async function submit() {
		const cmd = input.trim();
		if (!cmd || loading) return;
		if (cmd == 'exit') {
			outputHistory = [
				...outputHistory,
				{ type: 'command', text: 'exit' },
				{
					type: 'result',
					text: 'you can close the console pane by clicking the same button you used to open it. thanks for using kvweb. i love you.'
				}
			];
			scrollToBottom();
			input = '';
			inputEl?.focus();
			return;
		}

		if (commandHistory[commandHistory.length - 1] !== cmd) {
			commandHistory = [...commandHistory, cmd];
		}
		historyIndex = -1;
		input = '';

		outputHistory = [...outputHistory, { type: 'command', text: cmd }];
		scrollToBottom();

		loading = true;
		try {
			const result = await api.exec(cmd);
			const text = formatExecResult(result);
			outputHistory = [
				...outputHistory,
				{ type: result.type === 'error' ? 'error' : 'result', text }
			];
		} catch (e) {
			outputHistory = [
				...outputHistory,
				{ type: 'error', text: `(error) ${e instanceof Error ? e.message : String(e)}` }
			];
		} finally {
			loading = false;
			scrollToBottom();
			inputEl?.focus();
		}
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter') {
			e.preventDefault();
			submit();
		} else if (e.key === 'ArrowUp') {
			e.preventDefault();
			if (commandHistory.length === 0) return;
			if (historyIndex === -1) {
				historyIndex = commandHistory.length - 1;
			} else if (historyIndex > 0) {
				historyIndex--;
			}
			input = commandHistory[historyIndex];
		} else if (e.key === 'ArrowDown') {
			e.preventDefault();
			if (historyIndex === -1) return;
			if (historyIndex < commandHistory.length - 1) {
				historyIndex++;
				input = commandHistory[historyIndex];
			} else {
				historyIndex = -1;
				input = '';
			}
		} else if (e.key === 'l' && (e.ctrlKey || e.metaKey)) {
			e.preventDefault();
			outputHistory = [];
		}
	}

	function clear() {
		outputHistory = [];
	}

	export function focus() {
		inputEl?.focus();
	}
</script>

<div class="flex h-full flex-col bg-background">
	<!-- Toolbar -->
	<div class="flex items-center gap-2 border-b border-border px-3 py-1.5">
		<span class="text-xs font-medium text-muted-foreground">Console</span>
		{#if readOnly}
			<span class="text-xs text-muted-foreground">(read-only)</span>
		{/if}
		{#if prefix}
			<span class="text-xs text-muted-foreground">(prefix: {prefix})</span>
		{/if}
		<div class="ml-auto flex items-center gap-1.5">
			<span class="text-xs opacity-40">
				<Kbd.Root>{isMac ? '⌃' : 'Ctrl'}</Kbd.Root><Kbd.Root>L</Kbd.Root>
			</span>
			<Button variant="ghost" size="sm" class="h-6 w-6 p-0" onclick={clear} title="Clear console">
				<Eraser class="h-3.5 w-3.5" />
			</Button>
		</div>
	</div>

	<!-- Output -->
	<div class="flex-1 overflow-auto p-3 font-mono text-sm" bind:this={outputEl}>
		{#each outputHistory as line}
			{#if line.type === 'command'}
				<div class="text-muted-foreground">
					<span class="text-primary">valkey&gt;</span>
					{' '}{line.text}
				</div>
			{:else if line.type === 'error'}
				<pre class="whitespace-pre-wrap text-destructive">{line.text}</pre>
			{:else}
				<pre class="whitespace-pre-wrap text-foreground">{line.text}</pre>
			{/if}
		{/each}
		{#if loading}
			<div class="animate-pulse text-muted-foreground">...</div>
		{/if}
	</div>

	<!-- Input -->
	<div class="flex items-center border-t border-border px-3 py-2 font-mono text-sm">
		<span class="mr-2 text-primary">valkey&gt;</span>
		<input
			bind:this={inputEl}
			bind:value={input}
			onkeydown={handleKeydown}
			class="flex-1 bg-transparent text-foreground outline-none placeholder:text-muted-foreground"
			placeholder="Enter command..."
			disabled={loading}
			spellcheck="false"
			autocomplete="off"
		/>
	</div>
</div>
