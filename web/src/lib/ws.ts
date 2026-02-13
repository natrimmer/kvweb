// WebSocket message types

export type KeyEvent = {
	op: 'set' | 'del' | 'expire' | 'expired' | 'rename_from' | 'rename_to';
	key: string;
};

export type Stats = {
	dbSize: number;
	usedMemory: number;
	usedMemoryHuman: string;
	notificationsOn: boolean;
};

export type Status = {
	live: boolean;
	msg?: string;
};

type Message =
	| { type: 'key_event'; data: KeyEvent }
	| { type: 'stats'; data: Stats }
	| { type: 'status'; data: Status };

type Handler<T> = (data: T) => void;

class WebSocketManager {
	private ws: WebSocket | null = null;
	private keyHandlers = new Set<Handler<KeyEvent>>();
	private statsHandlers = new Set<Handler<Stats>>();
	private statusHandlers = new Set<Handler<Status>>();
	private reconnectDelay = 1000;
	private shouldReconnect = true;
	private url: string = '';

	connect() {
		if (this.ws?.readyState === WebSocket.OPEN) return;

		const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
		this.url = `${protocol}//${location.host}/ws`;
		this.ws = new WebSocket(this.url);

		this.ws.onmessage = (e) => {
			try {
				const msg: Message = JSON.parse(e.data);
				if (msg.type === 'key_event') {
					this.keyHandlers.forEach((h) => h(msg.data));
				} else if (msg.type === 'stats') {
					this.statsHandlers.forEach((h) => h(msg.data));
				} else if (msg.type === 'status') {
					this.statusHandlers.forEach((h) => h(msg.data));
				}
			} catch {
				// Ignore parse errors
			}
		};

		this.ws.onclose = () => {
			this.ws = null;
			if (this.shouldReconnect) {
				setTimeout(() => this.connect(), this.reconnectDelay);
				this.reconnectDelay = Math.min(this.reconnectDelay * 2, 30000);
			}
		};

		this.ws.onerror = () => {
			// Will trigger onclose
		};

		this.ws.onopen = () => {
			this.reconnectDelay = 1000;
		};
	}

	disconnect() {
		this.shouldReconnect = false;
		this.ws?.close();
		this.ws = null;
	}

	onKeyEvent(handler: Handler<KeyEvent>): () => void {
		this.keyHandlers.add(handler);
		return () => this.keyHandlers.delete(handler);
	}

	onStats(handler: Handler<Stats>): () => void {
		this.statsHandlers.add(handler);
		return () => this.statsHandlers.delete(handler);
	}

	onStatus(handler: Handler<Status>): () => void {
		this.statusHandlers.add(handler);
		return () => this.statusHandlers.delete(handler);
	}

	isConnected(): boolean {
		return this.ws?.readyState === WebSocket.OPEN;
	}
}

export const ws = new WebSocketManager();
