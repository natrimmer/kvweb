<script lang="ts">
	import type { GeoMember } from '$lib/api';
	import L from 'leaflet';
	import 'leaflet/dist/leaflet.css';
	import { onMount } from 'svelte';

	interface Props {
		members: GeoMember[];
		keyName: string;
	}

	let { members, keyName }: Props = $props();

	let mapContainer: HTMLDivElement;
	let map: L.Map | null = null;
	let markersLayer: L.LayerGroup | null = null;
	let previousKeyName = '';
	let previousMembersLength = 0;
	let shouldFitBounds = false;

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
		shouldFitBounds = true; // Fit bounds on initial load
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
			const container = document.createElement('div');
			const strong = document.createElement('strong');
			strong.textContent = member.member;
			container.appendChild(strong);
			container.appendChild(document.createElement('br'));
			container.appendChild(
				document.createTextNode(`${member.longitude.toFixed(6)}, ${member.latitude.toFixed(6)}`)
			);
			marker.bindPopup(container);
			markersLayer.addLayer(marker);
		}

		// Fit bounds if we've been told to (key changed and we now have the new data)
		if (shouldFitBounds && members.length > 0) {
			const bounds = L.latLngBounds(members.map((m) => [m.latitude, m.longitude]));
			map.fitBounds(bounds, { padding: [50, 50], maxZoom: 12 });
			shouldFitBounds = false;
		}
	}

	// Update markers when members change
	$effect(() => {
		const keyChanged = keyName !== previousKeyName && previousKeyName !== '';
		const membersChanged = members.length !== previousMembersLength;

		// When key changes, set flag to fit bounds but DON'T update markers yet (data is stale)
		if (keyChanged) {
			shouldFitBounds = true;
			previousKeyName = keyName;
			// Don't update markers yet - wait for new members data
			return;
		}

		if (previousKeyName === '') {
			previousKeyName = keyName;
		}

		// Update markers when members change
		if (membersChanged || shouldFitBounds) {
			previousMembersLength = members.length;
			if (map && markersLayer) {
				updateMarkers();
			}
		}
	});
</script>

<div bind:this={mapContainer} class="h-full min-h-75 w-full rounded border border-border"></div>

<style>
	:global(.leaflet-container) {
		font-family: inherit;
	}
</style>
