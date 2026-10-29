import { fetchApi } from '$lib/api';

export interface AccountCapabilities {
	localAccount: boolean;
	totpAvailable: boolean;
}

let pending: Promise<AccountCapabilities> | null = null;

// Password and two-factor settings exist only for local accounts, and two-factor only when the
// server enables it; the Account layout and each of those pages ask once per page load.
export function accountCapabilities(): Promise<AccountCapabilities> {
	pending ??= (async () => {
		const result = { localAccount: false, totpAvailable: false };
		try {
			const user = await fetchApi('/users/me');
			if (user.ok) result.localAccount = !(await user.json()).identities?.length;
			const config = await fetch('/auth-providers');
			if (result.localAccount && config.ok && (await config.json()).totp_enabled) {
				const response = await fetchApi('/users/totp');
				if (response.ok) result.totpAvailable = (await response.json()).local;
			}
		} catch {
			// The account pages stay usable when the optional status is unavailable.
		}
		return result;
	})();
	return pending;
}
