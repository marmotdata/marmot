export function loginRedirect(raw: string | null, origin: string): string {
	if (!raw) return '/';
	try {
		const target = new URL(raw, origin);
		return target.origin === origin ? `${target.pathname}${target.search}${target.hash}` : '/';
	} catch {
		return '/';
	}
}
