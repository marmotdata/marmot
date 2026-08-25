import { writable } from 'svelte/store';

export type ToastVariant = 'success' | 'error' | 'info' | 'warning';

export interface Toast {
	id: string;
	message: string;
	variant: ToastVariant;
	duration: number;
}

function createToastStore() {
	const { subscribe, update } = writable<Toast[]>([]);

	function add(message: string, variant: ToastVariant = 'info', duration = 5000) {
		const id = crypto.randomUUID();
		const toast: Toast = { id, message, variant, duration };

		update((toasts) => [...toasts, toast]);

		if (duration > 0) {
			setTimeout(() => remove(id), duration);
		}

		return id;
	}

	function remove(id: string) {
		update((toasts) => toasts.filter((t) => t.id !== id));
	}

	function clear() {
		update(() => []);
	}

	return {
		subscribe,
		add,
		remove,
		clear,
		success: (message: string, duration?: number) => add(message, 'success', duration),
		error: (message: string, duration?: number) => add(message, 'error', duration ?? 8000),
		info: (message: string, duration?: number) => add(message, 'info', duration),
		warning: (message: string, duration?: number) => add(message, 'warning', duration ?? 6000)
	};
}

export const toasts = createToastStore();

/**
 * Helper to extract error message from unknown error
 */
export function getErrorMessage(error: unknown): string {
	if (error instanceof Error) {
		return error.message;
	}
	if (typeof error === 'string') {
		return error;
	}
	return 'An unexpected error occurred';
}

/**
 * Helper to handle API errors consistently
 */
export async function handleApiError(response: Response): Promise<string> {
	const parsed = await parseApiError(response);
	return parsed.message;
}

/**
 * Structured API error info. `code` is set for errors the server tagged
 * (e.g. `limit_exceeded` on a 403 when a resource cap is reached), so the
 * UI can show a distinct treatment without brittle string matching.
 */
export interface ApiErrorInfo {
	message: string;
	status: number;
	code?: string;
	resource?: string;
	current?: number;
	limit?: number;
}

export async function parseApiError(response: Response): Promise<ApiErrorInfo> {
	const info: ApiErrorInfo = {
		message: `Request failed with status ${response.status}`,
		status: response.status
	};
	try {
		const data = await response.json();
		if (typeof data.error === 'string') info.message = data.error;
		else if (typeof data.message === 'string') info.message = data.message;
		if (typeof data.code === 'string') info.code = data.code;
		if (typeof data.resource === 'string') info.resource = data.resource;
		if (typeof data.current === 'number') info.current = data.current;
		if (typeof data.limit === 'number') info.limit = data.limit;
	} catch {
		// non-JSON body; keep default message
	}
	return info;
}

export function isLimitExceeded(info: ApiErrorInfo | ApiError): boolean {
	return info.status === 403 && info.code === 'limit_exceeded';
}

/**
 * Error subclass carrying the parsed API error fields. `message` is set to
 * the server's friendly text so `catch (err) { err.message }` code paths keep
 * working; UI that wants richer handling can inspect `code`, `resource`, etc.
 */
export class ApiError extends Error {
	status: number;
	code?: string;
	resource?: string;
	current?: number;
	limit?: number;

	constructor(info: ApiErrorInfo) {
		super(info.message);
		this.name = 'ApiError';
		this.status = info.status;
		this.code = info.code;
		this.resource = info.resource;
		this.current = info.current;
		this.limit = info.limit;
	}
}

/**
 * Parse `response` as an API error and throw it. Never returns.
 */
export async function throwApiError(response: Response): Promise<never> {
	throw new ApiError(await parseApiError(response));
}
