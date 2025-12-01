import { Centrifuge, type Subscription } from 'centrifuge';
import { browser } from '$app/environment';

export type JobRunEvent = {
	type:
		| 'job_run_created'
		| 'job_run_updated'
		| 'job_run_claimed'
		| 'job_run_started'
		| 'job_run_progress'
		| 'job_run_completed'
		| 'job_run_cancelled';
	payload: any;
	timestamp: string;
};

type EventCallback = (event: JobRunEvent) => void;

class WebSocketService {
	private centrifuge: Centrifuge | null = null;
	private jobRunsSubscription: Subscription | null = null;
	private callbacks: Set<EventCallback> = new Set();
	private isConnected = false;

	constructor() {
		if (browser) {
			this.connect();
		}
	}

	private connect() {
		// In development, connect directly to the backend server
		// In production, use the same host as the app
		const isDev = import.meta.env.DEV;
		const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';

		let wsUrl: string;
		if (isDev) {
			// Development: connect directly to backend on port 8080
			wsUrl = 'ws://localhost:8080/api/v1/ingestion/ws';
		} else {
			// Production: use same host as app
			wsUrl = `${protocol}//${window.location.host}/api/v1/ingestion/ws`;
		}

		console.log('[WebSocket] Connecting to:', wsUrl, '(dev mode:', isDev, ')');

		this.centrifuge = new Centrifuge(wsUrl, {
			// Retry configuration for better resilience during startup
			minReconnectDelay: 100,
			maxReconnectDelay: 5000,
			maxServerPingDelay: 10000,
			timeout: 5000
		});

		this.centrifuge.on('connecting', (ctx) => {
			console.log('[WebSocket] Connecting...', ctx);
		});

		this.centrifuge.on('connected', (ctx) => {
			console.log('[WebSocket] Connected', ctx);
			this.isConnected = true;
			this.subscribeToJobRuns();
		});

		this.centrifuge.on('disconnected', (ctx) => {
			console.log('[WebSocket] Disconnected', ctx);
			this.isConnected = false;
		});

		this.centrifuge.on('error', (ctx) => {
			console.error('[WebSocket] Error', ctx);
		});

		this.centrifuge.connect();
	}

	private subscribeToJobRuns() {
		if (!this.centrifuge) return;

		this.jobRunsSubscription = this.centrifuge.newSubscription('job_runs');

		this.jobRunsSubscription.on('publication', (ctx) => {
			try {
				const event = ctx.data as JobRunEvent;
				console.log('[WebSocket] Job run event received:', event.type, event.payload);
				// Notify all callbacks with individual error handling
				this.callbacks.forEach((callback) => {
					try {
						callback(event);
					} catch (callbackError) {
						console.error('[WebSocket] Error in event callback:', callbackError);
					}
				});
			} catch (error) {
				console.error('[WebSocket] Error processing event:', error);
			}
		});

		this.jobRunsSubscription.on('subscribing', (ctx) => {
			console.log('[WebSocket] Subscribing to job_runs...', ctx);
		});

		this.jobRunsSubscription.on('subscribed', (ctx) => {
			console.log('[WebSocket] Subscribed to job_runs', ctx);
		});

		this.jobRunsSubscription.on('unsubscribed', (ctx) => {
			console.log('[WebSocket] Unsubscribed from job_runs', ctx);
		});

		this.jobRunsSubscription.on('error', (ctx) => {
			console.error('[WebSocket] Subscription error:', ctx);
		});

		this.jobRunsSubscription.subscribe();
	}

	/**
	 * Subscribe to job run events
	 * Returns an unsubscribe function
	 */
	public subscribe(callback: EventCallback): () => void {
		this.callbacks.add(callback);
		return () => {
			this.callbacks.delete(callback);
		};
	}

	/**
	 * Disconnect from websocket
	 */
	public disconnect() {
		if (this.jobRunsSubscription) {
			this.jobRunsSubscription.unsubscribe();
			this.jobRunsSubscription.removeAllListeners();
			this.jobRunsSubscription = null;
		}

		if (this.centrifuge) {
			this.centrifuge.disconnect();
			this.centrifuge = null;
		}

		this.isConnected = false;
		this.callbacks.clear();
	}

	/**
	 * Get connection status
	 */
	public connected(): boolean {
		return this.isConnected;
	}
}

// Singleton instance
export const websocketService = new WebSocketService();
