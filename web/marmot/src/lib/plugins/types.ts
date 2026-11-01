// Mirrors internal/plugin/updates.Status and its nested types: the running version of each core
// plugin, the versions it could move to, and any security advisories against it.

/** Who or what last chose the version a plugin runs. */
export type PluginVersionOrigin = 'release' | 'manual' | 'auto' | 'local';

/** How Marmot moves unpinned plugins on its own. */
export type PluginAutoUpdatePolicy = 'off' | 'security' | 'patch';

/** A security advisory against some versions of a plugin. */
export interface PluginAdvisory {
	id: string;
	severity: string;
	/** The semver constraint on the plugin versions the advisory applies to, e.g. "<0.3.1". */
	affected: string;
	fixed: string;
	summary: string;
	url?: string;
	published?: string;
}

/** A version published in the plugin index, as it applies to this Marmot. */
export interface PluginVersionOption {
	version: string;
	digest: string;
	/** Semver constraint on the Marmot releases this version works with; empty means every release. */
	marmot_version?: string;
	protocol_version?: number;
	released?: string;
	changelog_url?: string;
	yanked?: boolean;
	yanked_reason?: string;
	compatible: boolean;
	/** IDs of the advisories against this version; full detail is only available for the running version. */
	advisories: string[];
}

/** One core plugin's running version and the versions it could move to. */
export interface PluginVersionStatus {
	id: string;
	name: string;
	icon?: string;
	/** The version the plugin should run at; empty for a locally installed override. */
	version: string;
	/** Whether this replica has the plugin registered. */
	loaded: boolean;
	release_version: string;
	origin: PluginVersionOrigin;
	pinned: boolean;
	previous_version?: string;
	updated_by?: string;
	updated_at?: string;
	/** The newest version that supports this Marmot and has no advisory against it. */
	latest?: string;
	update_available: boolean;
	/** Advisories against the version the plugin runs. */
	advisories: PluginAdvisory[];
	versions: PluginVersionOption[];
	/** This replica's last failure switching the plugin's version. */
	error?: string;
}

/** The stored plugin index and the last check against it. */
export interface PluginIndexStatus {
	ref: string;
	version?: number;
	generated?: string;
	expires_at?: string;
	expired: boolean;
	fetched_at?: string;
	checked_at?: string;
	error?: string;
}

/** What the Plugins settings page and CLI show. */
export interface PluginUpdatesStatus {
	/** Whether Marmot checks the plugin index for updates. */
	enabled: boolean;
	auto_update: PluginAutoUpdatePolicy;
	marmot_version: string;
	index: PluginIndexStatus;
	plugins: PluginVersionStatus[];
}
