// Mirrors the Go parser in plugin-sdk/mrn so the UI reads MRNs the way the server writes them.

export const MRN_PREFIX = 'mrn://';

const HOST_HINT_PREFIX = 'host=';

export type MRNKind =
	'asset' | 'glossary-term' | 'data-product' | 'team' | 'user' | 'pipeline' | 'source';

// How many segments follow the kind for each kind.
const KIND_PARTS: Record<MRNKind, number> = {
	asset: 3,
	'glossary-term': 2,
	'data-product': 1,
	team: 1,
	user: 1,
	pipeline: 1,
	source: 1
};

/** The parts of an MRN. Type, provider and zone are set for assets only and source for glossary terms only. */
export interface ParsedMRN {
	kind: MRNKind;
	type: string;
	provider: string;
	zone: string;
	source: string;
	name: string;
	/** True when the MRN was written without a kind (mrn://type/provider/name). */
	legacy: boolean;
}

/** The parts the API sends beside an asset's mrn. */
export interface MRNParts {
	kind: string;
	type?: string;
	provider?: string;
	zone?: string;
	name: string;
	path?: string[];
}

function isKind(s: string): s is MRNKind {
	return Object.prototype.hasOwnProperty.call(KIND_PARTS, s);
}

function clean(part: string): string {
	return part.normalize('NFC').toLowerCase().replace(/[/\s]/g, '-');
}

function cleanSegment(part: string): string {
	return clean(part).replace(/@/g, '-');
}

function parseAsset(type: string, segment: string, name: string, legacy: boolean): ParsedMRN {
	const at = segment.indexOf('@');
	const provider = at >= 0 ? segment.slice(0, at) : segment;
	const rawZone = at >= 0 ? segment.slice(at + 1) : '';
	const zone = rawZone.toLowerCase().startsWith(HOST_HINT_PREFIX)
		? HOST_HINT_PREFIX + cleanSegment(rawZone.slice(HOST_HINT_PREFIX.length))
		: cleanSegment(rawZone);
	return {
		kind: 'asset',
		type: clean(type),
		provider: cleanSegment(provider),
		zone,
		source: '',
		name: clean(name),
		legacy
	};
}

/** Parses an MRN in either form, or returns null when it is not one. */
export function parseMRN(input: string | null | undefined): ParsedMRN | null {
	const s = (input ?? '').trim();
	if (!s.toLowerCase().startsWith(MRN_PREFIX)) return null;
	const parts = s.slice(MRN_PREFIX.length).split('/');
	if (parts.some((p) => p === '')) return null;

	const kind = parts[0].toLowerCase();
	if (isKind(kind) && parts.length - 1 === KIND_PARTS[kind]) {
		if (kind === 'asset') return parseAsset(parts[1], parts[2], parts[3], false);
		const base = { type: '', provider: '', zone: '', source: '', legacy: false };
		if (kind === 'glossary-term')
			return { ...base, kind, source: cleanSegment(parts[1]), name: clean(parts[2]) };
		return { ...base, kind, name: clean(parts[1]) };
	}
	if (parts.length === 3) return parseAsset(parts[0], parts[1], parts[2], true);
	return null;
}

/** The MRN's canonical (kinded) spelling, or the input unchanged when it does not parse. */
export function canonicalMRN(input: string): string {
	const m = parseMRN(input);
	if (!m) return input;
	switch (m.kind) {
		case 'asset':
			return `${MRN_PREFIX}asset/${m.type}/${providerSegment(m)}/${m.name}`;
		case 'glossary-term':
			return `${MRN_PREFIX}glossary-term/${m.source}/${m.name}`;
		default:
			return `${MRN_PREFIX}${m.kind}/${m.name}`;
	}
}

/** The provider with its zone, as it appears in the MRN and in asset URLs. */
export function providerSegment(m: Pick<ParsedMRN, 'provider' | 'zone'>): string {
	return m.zone ? `${m.provider}@${m.zone}` : m.provider;
}

/** The host a host= hint names, if the zone is one. */
export function hostHint(m: Pick<ParsedMRN, 'zone'>): string | null {
	return m.zone.startsWith(HOST_HINT_PREFIX) ? m.zone.slice(HOST_HINT_PREFIX.length) : null;
}

/** Splits a qualified name at dots outside double quotes and unquotes each part. */
export function splitPath(name: string): string[] {
	const parts: string[] = [];
	let cur = '';
	let quoted = false;
	for (let i = 0; i < name.length; i++) {
		const c = name[i];
		if (c === '"' && quoted && name[i + 1] === '"') {
			cur += '"';
			i++;
		} else if (c === '"') {
			quoted = !quoted;
		} else if (c === '.' && !quoted) {
			parts.push(cur);
			cur = '';
		} else {
			cur += c;
		}
	}
	parts.push(cur);
	return parts;
}

// Keeps @ readable in asset URLs; everything else the route needs is escaped.
function encodeSegment(s: string): string {
	return encodeURIComponent(s).replace(/%40/g, '@');
}

/** The route segments of an asset page, already encoded so they can go straight into a path. */
export interface AssetRouteParams {
	type: string;
	service: string;
	name: string;
}

/** Route params for an asset MRN, or null for anything that is not an asset (legacy MRNs included, they map to the same page). */
export function assetRouteParams(mrn: string | null | undefined): AssetRouteParams | null {
	const m = parseMRN(mrn);
	if (!m || m.kind !== 'asset') return null;
	return {
		type: encodeSegment(m.type),
		service: encodeSegment(providerSegment(m)),
		name: encodeSegment(m.name)
	};
}

export type DiscoverPath = `/discover/${string}/${string}/${string}`;

/** The asset page path for an asset MRN, /discover/<type>/<provider[@zone]>/<name>, or null when the MRN is not an asset's. */
export function discoverPath(mrn: string | null | undefined): DiscoverPath | null {
	const p = assetRouteParams(mrn);
	return p ? `/discover/${p.type}/${p.service}/${p.name}` : null;
}

/** The pieces of an asset a link can be built from when the MRN is missing or unreadable. */
export interface AssetLike {
	mrn?: string | null;
	name?: string;
	type?: string;
	providers?: string[];
}

/** Route params for an asset, from its MRN when it has a readable one and from its type, first provider and name otherwise. */
export function assetRouteParamsFor(asset: AssetLike): AssetRouteParams {
	return (
		assetRouteParams(asset.mrn) ?? {
			type: encodeSegment((asset.type ?? '').toLowerCase()),
			service: encodeSegment((asset.providers?.[0] ?? '').toLowerCase()),
			name: encodeSegment(asset.name ?? '')
		}
	);
}

/** The provider of an asset MRN, lowercase, or an empty string. This is what picks the provider logo. */
export function mrnProvider(mrn: string | null | undefined): string {
	const m = parseMRN(mrn);
	return m && m.kind === 'asset' ? m.provider : '';
}

/** The zone of an asset MRN, or an empty string for no zone. Host hints are not zones. */
export function mrnZone(mrn: string | null | undefined): string {
	const m = parseMRN(mrn);
	return m && m.kind === 'asset' && !hostHint(m) ? m.zone : '';
}

/** The last segment of an MRN of any kind: an asset's qualified name (analytics.public.orders) or the slug of another kind. */
export function mrnShortName(mrn: string | null | undefined): string {
	const m = parseMRN(mrn);
	return m ? m.name : ((mrn ?? '').split('/').pop() ?? '');
}

/** The qualified name of an asset MRN with its levels joined by dots, e.g. analytics.public.orders. */
export function mrnQualifiedName(mrn: string | null | undefined): string {
	const m = parseMRN(mrn);
	return m ? m.name : '';
}

/** The zone of an asset from the API's mrn_parts, falling back to its MRN. */
export function zoneOf(asset: { mrn?: string | null; mrn_parts?: MRNParts | null }): string {
	const fromParts = asset.mrn_parts?.zone;
	if (fromParts !== undefined) return fromParts.startsWith(HOST_HINT_PREFIX) ? '' : fromParts;
	return mrnZone(asset.mrn);
}

/** The qualified name when it says more than the asset's own name does (analytics.public.orders against orders), otherwise an empty string. */
export function qualifierOf(asset: {
	name?: string;
	mrn?: string | null;
	mrn_parts?: MRNParts | null;
}): string {
	const qualified = asset.mrn_parts?.name ?? mrnQualifiedName(asset.mrn);
	return qualified.toLowerCase() === (asset.name ?? '').toLowerCase() ? '' : qualified;
}
