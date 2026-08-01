export function isMac(): boolean {
	const uad = (navigator as Navigator & { userAgentData?: { platform: string } }).userAgentData;
	if (uad) {
		return uad.platform === 'macOS';
	}
	return /Mac|iPhone|iPad|iPod/.test(navigator.userAgent);
}

/** "Cmd" on macOS, "Ctrl" everywhere else. */
export function getModifierKey(): string {
	return isMac() ? 'Cmd' : 'Ctrl';
}

/**
 * `ctrl` matches the platform's primary modifier: Cmd on macOS, Ctrl elsewhere.
 * Modifiers must match exactly, so Ctrl+Shift+S does not satisfy a Ctrl+S shortcut.
 */
export function matchesShortcut(
	event: KeyboardEvent,
	key: string,
	ctrl = false,
	shift = false,
	alt = false
): boolean {
	const hasModifier = isMac() ? event.metaKey : event.ctrlKey;
	return (
		event.key.toLowerCase() === key.toLowerCase() &&
		hasModifier === ctrl &&
		event.shiftKey === shift &&
		event.altKey === alt
	);
}

/** Renders a shortcut for display: "Cmd+S", "Ctrl+Shift+S", "Delete". */
export function formatShortcut(key: string, ctrl = false, shift = false, alt = false): string {
	const parts: string[] = [];

	if (ctrl) {
		parts.push(getModifierKey());
	}
	if (shift) {
		parts.push('Shift');
	}
	if (alt) {
		parts.push('Alt');
	}

	parts.push(key);

	return parts.join('+');
}

/** True when focus is somewhere a keystroke is text, not a shortcut. */
export function isInputFocused(): boolean {
	const activeElement = document.activeElement;
	if (!activeElement) return false;

	const tagName = activeElement.tagName.toLowerCase();
	return (
		tagName === 'input' ||
		tagName === 'textarea' ||
		tagName === 'select' ||
		activeElement.getAttribute('contenteditable') === 'true'
	);
}
