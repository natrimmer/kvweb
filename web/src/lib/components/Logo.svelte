<script lang="ts">
	interface Props {
		size?: number;
		class?: string;
	}

	let { size = 24, class: className = '' }: Props = $props();
</script>

<svg
	width={size}
	height={size}
	viewBox="0 0 64 64"
	xmlns="http://www.w3.org/2000/svg"
	class={className}
>
	<defs>
		<clipPath id="cell-clip">
			<rect x="32" y="8" width="24" height="16" rx="3" />
		</clipPath>
	</defs>
	<rect
		x="8"
		y="8"
		width="48"
		height="48"
		rx="6"
		fill="none"
		stroke="currentColor"
		stroke-width="3"
	/>
	<line x1="32" y1="8" x2="32" y2="56" stroke="currentColor" stroke-width="3" />
	<line x1="8" y1="24" x2="56" y2="24" stroke="currentColor" stroke-width="3" />
	<line x1="8" y1="40" x2="56" y2="40" stroke="currentColor" stroke-width="3" />
	<rect x="32" y="8" width="24" height="16" fill="currentColor" clip-path="url(#cell-clip)" />
</svg>
