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

export type SearchReindexEvent = {
	type:
		| 'search_reindex_started'
		| 'search_reindex_progress'
		| 'search_reindex_completed'
		| 'search_reindex_failed';
	payload: {
		indexed?: number;
		errors?: number;
		total?: number;
		error?: string;
	};
	timestamp: string;
};

type EventCallback = (event: JobRunEvent) => void;
type ReindexCallback = (event: SearchReindexEvent) => void;

class WebSocketService {
	private centrifuge: Centrifuge | null = null;
	private jobRunsSubscription: Subscription | null = null;
	private searchReindexSubscription: Subscription | null = null;
	private callbacks: Set<EventCallback> = new Set();
	private reindexCallbacks: Set<ReindexCallback> = new Set();
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

		this.centrifuge = new Centrifuge(wsUrl, {
			// Retry configuration for better resilience during startup
			minReconnectDelay: 100,
			maxReconnectDelay: 5000,
			maxServerPingDelay: 10000,
			timeout: 5000
		});

		this.centrifuge.on('connected', () => {
			this.isConnected = true;
			this.subscribeToJobRuns();
			if (this.reindexCallbacks.size > 0) {
				this.subscribeToSearchReindex();
			}
		});

		this.centrifuge.on('disconnected', () => {
			this.isConnected = false;
		});

		this.centrifuge.connect();
	}

	private subscribeToJobRuns() {
		if (!this.centrifuge) return;

		this.jobRunsSubscription = this.centrifuge.newSubscription('job_runs');

		this.jobRunsSubscription.on('publication', (ctx) => {
			try {
				const event = ctx.data as JobRunEvent;
				this.callbacks.forEach((callback) => {
					try {
						callback(event);
					} catch {
						// Silently ignore callback errors
					}
				});
			} catch {
				// Silently ignore processing errors
			}
		});

		this.jobRunsSubscription.subscribe();
	}

	private subscribeToSearchReindex() {
		if (!this.centrifuge || this.searchReindexSubscription) return;

		this.searchReindexSubscription = this.centrifuge.newSubscription('search_reindex');

		this.searchReindexSubscription.on('publication', (ctx) => {
			try {
				const event = ctx.data as SearchReindexEvent;
				this.reindexCallbacks.forEach((callback) => {
					try {
						callback(event);
					} catch {
						// Silently ignore callback errors
					}
				});
			} catch {
				// Silently ignore processing errors
			}
		});

		this.searchReindexSubscription.subscribe();
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
	 * Subscribe to search reindex events
	 * Lazily subscribes to the search_reindex channel on first call.
	 * Centrifuge handles pending subscriptions if not yet connected.
	 * Returns an unsubscribe function
	 */
	public subscribeToReindex(callback: ReindexCallback): () => void {
		this.reindexCallbacks.add(callback);
		this.subscribeToSearchReindex();
		return () => {
			this.reindexCallbacks.delete(callback);
		};
	}

	/**
	 * Disconnect from websocket
	 */
	public disconnect() {
		if (this.searchReindexSubscription) {
			this.searchReindexSubscription.unsubscribe();
			this.searchReindexSubscription.removeAllListeners();
			this.searchReindexSubscription = null;
		}

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
		this.reindexCallbacks.clear();
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
