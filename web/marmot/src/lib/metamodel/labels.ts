export interface MessageContext {
	locale: string;
	defaultLocale: string;
	/** Catalogues shipped with the profile, keyed by locale. */
	messages?: Record<string, Record<string, string>>;
	/** Marmot's own catalogue, for the keys native fields already use. */
	native?: (key: string) => string | undefined;
}

/**
 * Profile catalogue in the active locale, then in the profile's default locale, then Marmot's
 * catalogue. Undefined when nothing matches so callers pick their own fallback.
 */
export function resolveMessage(
	key: string | undefined,
	context: MessageContext
): string | undefined {
	if (!key) return undefined;
	const { messages, locale, defaultLocale, native } = context;
	return messages?.[locale]?.[key] ?? messages?.[defaultLocale]?.[key] ?? native?.(key);
}
