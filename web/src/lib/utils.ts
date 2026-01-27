import { clsx, type ClassValue } from "clsx";
import { toast } from 'svelte-sonner';
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
	return twMerge(clsx(inputs));
}

/**
 * Extracts an error message from an unknown error value.
 */
export function getErrorMessage(e: unknown, fallback = 'An error occurred'): string {
	if (e instanceof Error) return e.message
	if (typeof e === 'string') return e
	return fallback
}

/**
 * Shows a toast error notification with a consistent format.
 */
export function toastError(e: unknown, fallback = 'An error occurred'): void {
	toast.error(getErrorMessage(e, fallback))
}

/**
 * Redis keyspace notification operations that indicate a key was deleted.
 * Used to filter WebSocket events for removing keys from UI.
 */
export const deleteOps = new Set(['del', 'expired'])

/**
 * Redis keyspace notification operations that indicate a key was created or modified.
 * Grouped by data type for clarity.
 */
export const modifyOps = new Set([
	'set',                                              // string
	'lpush', 'rpush', 'lpop', 'rpop', 'lset', 'ltrim',  // list
	'hset', 'hdel', 'hincrby', 'hincrbyfloat',          // hash
	'sadd', 'srem', 'spop',                             // set
	'zadd', 'zrem', 'zincrby',                          // sorted set
	'xadd', 'xtrim',                                    // stream
	'append', 'incr', 'decr', 'incrby', 'decrby',       // string modifications
	'setex', 'psetex', 'setnx',                         // string variants
])

/**
 * Formats a TTL value in seconds to a human-readable string.
 */
export function formatTtl(seconds: number): string {
	if (seconds < 0) return 'No expiry'
	if (seconds < 60) return `${seconds}s`
	if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`
	return `${Math.floor(seconds / 3600)}h ${Math.floor((seconds % 3600) / 60)}m`
}

/**
 * Copies text to clipboard and calls the callback with true, then false after a delay.
 * Useful for showing a "copied" indicator that auto-resets.
 */
export async function copyToClipboard(
	text: string,
	onCopied: (copied: boolean) => void,
	resetDelay = 2000
): Promise<void> {
	await navigator.clipboard.writeText(text)
	onCopied(true)
	setTimeout(() => onCopied(false), resetDelay)
}

/**
 * Syntax highlights a JSON string with optional pretty-printing.
 * Returns HTML with span elements for colors.
 */
export function highlightJson(str: string, format: boolean): string {
	try {
		const code = format ? JSON.stringify(JSON.parse(str), null, 2) : str
		// Escape HTML and apply syntax highlighting
		const escaped = code
			.replace(/&/g, '&amp;')
			.replace(/</g, '&lt;')
			.replace(/>/g, '&gt;')

		// Apply highlighting with regex
		const highlighted = escaped
			// Strings (including keys)
			.replace(/"([^"\\]|\\.)*"/g, (match) => {
				return `<span class="json-string">${match}</span>`
			})
			// Numbers
			.replace(/\b(-?\d+\.?\d*(?:[eE][+-]?\d+)?)\b/g, '<span class="json-number">$1</span>')
			// Booleans and null
			.replace(/\b(true|false|null)\b/g, '<span class="json-keyword">$1</span>')

		return `<pre class="json-highlight">${highlighted}</pre>`
	} catch {
		return ''
	}
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export type WithoutChild<T> = T extends { child?: any } ? Omit<T, "child"> : T;
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export type WithoutChildren<T> = T extends { children?: any } ? Omit<T, "children"> : T;
export type WithoutChildrenOrChild<T> = WithoutChildren<WithoutChild<T>>;
export type WithElementRef<T, U extends HTMLElement = HTMLElement> = T & { ref?: U | null };
