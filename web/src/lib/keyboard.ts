/**
 * Keyboard utility functions for handling keyboard shortcuts across platforms.
 */

/**
 * Detects if the current platform is macOS.
 */
export function isMac(): boolean {
	return /Mac|iPhone|iPad|iPod/.test(navigator.platform);
}

/**
 * Gets the appropriate modifier key name for the current platform.
 * @returns "Cmd" for macOS, "Ctrl" for other platforms
 */
export function getModifierKey(): string {
	return isMac() ? 'Cmd' : 'Ctrl';
}

/**
 * Checks if the keyboard event matches a specific shortcut.
 * @param event The keyboard event to check
 * @param key The key to match (e.g., 's', 'Enter', 'Escape')
 * @param ctrl Whether the Ctrl/Cmd modifier should be pressed
 * @param shift Whether the Shift modifier should be pressed
 * @param alt Whether the Alt modifier should be pressed
 * @returns true if the event matches the shortcut
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

/**
 * Formats a keyboard shortcut for display in the UI.
 * @param key The key (e.g., 'S', 'Delete', 'Escape')
 * @param ctrl Whether to include the Ctrl/Cmd modifier
 * @param shift Whether to include the Shift modifier
 * @param alt Whether to include the Alt modifier
 * @returns A formatted shortcut string (e.g., "Ctrl+S", "Cmd+S", "Delete")
 */
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

/**
 * Checks if the current active element is an input that should not be interrupted by shortcuts.
 * @returns true if an input element is focused
 */
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
