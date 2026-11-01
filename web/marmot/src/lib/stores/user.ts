import { writable } from 'svelte/store';
import { browser } from '$app/environment';
import { fetchApi } from '$lib/api';
import { auth } from '$lib/stores/auth';

export interface UserProfile {
	id: string;
	username: string;
	name: string;
	email?: string;
	display_name?: string;
	profile_picture?: string;
	job_title?: string;
	teams?: { id: string; name: string }[];
}

const CACHE_KEY = 'user-profile';

function readCache(): UserProfile | null {
	if (!browser) return null;
	try {
		const cached = localStorage.getItem(CACHE_KEY);
		return cached ? (JSON.parse(cached) as UserProfile) : null;
	} catch {
		return null;
	}
}

function createUserProfileStore() {
	// Seeded from the last known profile so a refresh shows the real name right
	// away instead of a placeholder while /users/me is still in flight.
	const { subscribe, set } = writable<UserProfile | null>(readCache());
	let request: Promise<UserProfile | null> | null = null;

	function store(profile: UserProfile | null) {
		set(profile);
		if (!browser) return;
		if (profile) {
			localStorage.setItem(CACHE_KEY, JSON.stringify(profile));
		} else {
			localStorage.removeItem(CACHE_KEY);
		}
	}

	return {
		subscribe,
		// Every caller on a page shares one request.
		load: (): Promise<UserProfile | null> => {
			// Anonymous mode has no profile, and asking for one gets us bounced to /login.
			if (!auth.isAuthenticated()) return Promise.resolve(null);

			request ??= fetchApi('/users/me')
				.then((res) => (res.ok ? (res.json() as Promise<UserProfile>) : null))
				.then((profile) => {
					if (profile) store(profile);
					return profile;
				})
				.catch(() => null);
			return request;
		},
		// Applies a change the caller has already saved, so the header reflects it without another request
		patch: (changes: Partial<UserProfile>) => {
			let current: UserProfile | null = null;
			subscribe((value) => (current = value))();
			if (current) store({ ...(current as UserProfile), ...changes });
		},
		clear: () => {
			request = null;
			store(null);
		}
	};
}

export const userProfile = createUserProfileStore();
