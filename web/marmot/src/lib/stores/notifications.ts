import { writable, derived, get } from 'svelte/store';
import { fetchApi } from '$lib/api';
import { browser } from '$app/environment';

export interface Notification {
	id: string;
	user_id: string;
	recipient_type: string;
	recipient_id: string;
	type: string;
	title: string;
	message: string;
	data?: Record<string, unknown>;
	read: boolean;
	read_at?: string;
	created_at: string;
}

export interface NotificationSummary {
	unread_count: number;
	total_count: number;
}

export interface NotificationsState {
	notifications: Notification[];
	summary: NotificationSummary;
	loading: boolean;
	error: string | null;
	nextCursor: string | null;
	hasMore: boolean;
}

const initialState: NotificationsState = {
	notifications: [],
	summary: { unread_count: 0, total_count: 0 },
	loading: false,
	error: null,
	nextCursor: null,
	hasMore: true
};

function createNotificationsStore() {
	const { subscribe, update, set } = writable<NotificationsState>(initialState);

	let pollInterval: ReturnType<typeof setInterval> | null = null;

	async function fetchSummary() {
		if (!browser) return;

		try {
			const response = await fetchApi('/notifications/summary');
			if (response.ok) {
				const summary = await response.json();
				update((state) => ({ ...state, summary }));
			}
		} catch (err) {
			console.error('Failed to fetch notification summary:', err);
		}
	}

	async function fetchNotifications(reset = false) {
		if (!browser) return;

		update((state) => ({ ...state, loading: true, error: null }));

		try {
			let url = '/notifications?limit=20';

			// Use cursor for pagination if not resetting
			let currentCursor: string | null = null;
			const unsubscribe = subscribe((state) => {
				currentCursor = state.nextCursor;
			});
			unsubscribe();

			if (!reset && currentCursor) {
				url += `&cursor=${encodeURIComponent(currentCursor)}`;
			}

			const response = await fetchApi(url);
			if (response.ok) {
				const data = await response.json();
				update((state) => ({
					...state,
					notifications: reset
						? data.notifications
						: [...state.notifications, ...data.notifications],
					nextCursor: data.next_cursor || null,
					hasMore: !!data.next_cursor,
					loading: false
				}));
			} else {
				update((state) => ({
					...state,
					loading: false,
					error: 'Failed to load notifications'
				}));
			}
		} catch (err) {
			console.error('Failed to fetch notifications:', err);
			update((state) => ({
				...state,
				loading: false,
				error: 'Failed to load notifications'
			}));
		}
	}

	async function markAsRead(id: string) {
		try {
			const response = await fetchApi(`/notifications/item/${id}/mark-read`, { method: 'POST' });
			if (response.ok) {
				update((state) => ({
					...state,
					notifications: state.notifications.map((n) =>
						n.id === id ? { ...n, read: true, read_at: new Date().toISOString() } : n
					),
					summary: {
						...state.summary,
						unread_count: Math.max(0, state.summary.unread_count - 1)
					}
				}));
			}
		} catch (err) {
			console.error('Failed to mark notification as read:', err);
		}
	}

	async function markAllAsRead() {
		try {
			const response = await fetchApi('/notifications/mark-all-read', { method: 'POST' });
			if (response.ok) {
				update((state) => ({
					...state,
					notifications: state.notifications.map((n) => ({
						...n,
						read: true,
						read_at: n.read_at || new Date().toISOString()
					})),
					summary: { ...state.summary, unread_count: 0 }
				}));
			}
		} catch (err) {
			console.error('Failed to mark all notifications as read:', err);
		}
	}

	async function deleteNotification(id: string) {
		try {
			const response = await fetchApi(`/notifications/item/${id}`, { method: 'DELETE' });
			if (response.ok) {
				update((state) => {
					const notification = state.notifications.find((n) => n.id === id);
					const wasUnread = notification && !notification.read;
					return {
						...state,
						notifications: state.notifications.filter((n) => n.id !== id),
						summary: {
							unread_count: wasUnread
								? Math.max(0, state.summary.unread_count - 1)
								: state.summary.unread_count,
							total_count: Math.max(0, state.summary.total_count - 1)
						}
					};
				});
			}
		} catch (err) {
			console.error('Failed to delete notification:', err);
		}
	}

	async function clearRead() {
		try {
			const response = await fetchApi('/notifications/clear-read', { method: 'POST' });
			if (response.ok) {
				update((state) => ({
					...state,
					notifications: state.notifications.filter((n) => !n.read),
					summary: {
						...state.summary,
						total_count: state.summary.unread_count
					}
				}));
			}
		} catch (err) {
			console.error('Failed to clear read notifications:', err);
		}
	}

	function startPolling(intervalMs = 30000) {
		if (pollInterval) return;

		// Initial fetch
		fetchSummary();

		pollInterval = setInterval(() => {
			fetchSummary();
		}, intervalMs);
	}

	function stopPolling() {
		if (pollInterval) {
			clearInterval(pollInterval);
			pollInterval = null;
		}
	}

	function reset() {
		stopPolling();
		set(initialState);
	}

	return {
		subscribe,
		fetchSummary,
		fetchNotifications,
		markAsRead,
		markAllAsRead,
		deleteNotification,
		clearRead,
		startPolling,
		stopPolling,
		reset,
		loadMore: () => fetchNotifications(false),
		refresh: () => {
			fetchSummary();
			return fetchNotifications(true);
		}
	};
}

export const notifications = createNotificationsStore();

// Derived store for unread count (useful for badge)
export const unreadCount = derived(
	notifications,
	($notifications) => $notifications.summary.unread_count
);
