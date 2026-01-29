<script lang="ts">
	import type { GeoMember } from '$lib/api';
	import L from 'leaflet';
	import 'leaflet/dist/leaflet.css';
	import { onMount } from 'svelte';

	interface Props {
		members: GeoMember[];
	}

	let { members }: Props = $props();

	let mapContainer: HTMLDivElement;
	let map: L.Map | null = null;
	let markersLayer: L.LayerGroup | null = null;

	onMount(() => {
		// Initialize map centered on first member or default to world view
		const center: L.LatLngExpression =
			members.length > 0 ? [members[0].latitude, members[0].longitude] : [20, 0];
		const zoom = members.length > 0 ? 4 : 2;

		map = L.map(mapContainer).setView(center, zoom);

		L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
			attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
		}).addTo(map);

		markersLayer = L.layerGroup().addTo(map);
		updateMarkers();

		return () => {
			map?.remove();
		};
	});

	function updateMarkers() {
		if (!map || !markersLayer) return;

		markersLayer.clearLayers();

		for (const member of members) {
			const marker = L.marker([member.latitude, member.longitude]);
			marker.bindPopup(
				`<strong>${member.member}</strong><br>${member.longitude.toFixed(6)}, ${member.latitude.toFixed(6)}`
			);
			markersLayer.addLayer(marker);
		}

		// Fit bounds if we have members
		if (members.length > 0) {
			const bounds = L.latLngBounds(members.map((m) => [m.latitude, m.longitude]));
			map.fitBounds(bounds, { padding: [50, 50], maxZoom: 12 });
		}
	}

	// Update markers when members change
	$effect(() => {
		members;
		updateMarkers();
	});
</script>

<div bind:this={mapContainer} class="h-full min-h-75 w-full rounded border border-border"></div>

<style>
	:global(.leaflet-container) {
		font-family: inherit;
	}
</style>
