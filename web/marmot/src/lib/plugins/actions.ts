// What each row action on the Plugins settings page does: the API call to make and the toasts to
// show around it. Kept apart from the page so the confirmations (which explain a rollback pins,
// and a reset clears a manual choice) are not tangled up with the page's own state.
import { MmConfirmDialog } from '@marmotdata/elements/classes/confirm-dialog.js';
import { m } from '$lib/paraglide/messages';
import {
	resetPluginVersion,
	rollbackPluginVersion,
	setPluginPinned,
	setPluginVersion
} from './api';
import type { PluginVersionStatus } from './types';

/** One version change: what to call, and what to say about it once it is done or fails. */
export interface PluginChange {
	run: () => Promise<void>;
	successMessage: string;
	errorMessage: string;
}

/** Moving straight to the newest compatible version without an advisory; null when there is none. */
export function updateToLatest(p: PluginVersionStatus): PluginChange | null {
	const latest = p.latest;
	if (!latest) return null;
	return {
		run: () => setPluginVersion(p.id, latest),
		successMessage: m.plugin_versions_update_success({ name: p.name, version: latest }),
		errorMessage: m.plugin_versions_update_error({ name: p.name })
	};
}

/** Moving to a version chosen from the picker dialog. */
export function chooseVersion(p: PluginVersionStatus, version: string): PluginChange {
	return {
		run: () => setPluginVersion(p.id, version),
		successMessage: m.plugin_versions_update_success({ name: p.name, version }),
		errorMessage: m.plugin_versions_update_error({ name: p.name })
	};
}

/** Pinning or unpinning the plugin at the version it currently runs. */
export function setPinned(p: PluginVersionStatus, pinned: boolean): PluginChange {
	return {
		run: () => setPluginPinned(p.id, pinned),
		successMessage: pinned
			? m.plugin_versions_pin_success({ name: p.name, version: p.version })
			: m.plugin_versions_unpin_success({ name: p.name }),
		errorMessage: m.plugin_versions_pin_error({ name: p.name })
	};
}

/** Asks for confirmation that rolling back pins the plugin; null when declined or there is nothing to roll back to. */
export async function confirmRollback(p: PluginVersionStatus): Promise<PluginChange | null> {
	const target = p.previous_version;
	if (!target) return null;
	const ok = await MmConfirmDialog.confirm({
		heading: m.plugin_versions_rollback_confirm_heading({ name: p.name, version: target }),
		body: m.plugin_versions_rollback_confirm_body({ name: p.name, version: target }),
		confirmLabel: m.plugin_versions_rollback_confirm_action(),
		confirmIcon: 'history',
		tone: 'default'
	});
	if (!ok) return null;
	return {
		run: () => rollbackPluginVersion(p.id),
		successMessage: m.plugin_versions_rollback_success({ name: p.name, version: target }),
		errorMessage: m.plugin_versions_rollback_error({ name: p.name })
	};
}

/** Asks for confirmation that resetting returns to the release version; null when declined. */
export async function confirmReset(p: PluginVersionStatus): Promise<PluginChange | null> {
	const ok = await MmConfirmDialog.confirm({
		heading: m.plugin_versions_reset_confirm_heading({ name: p.name }),
		body: m.plugin_versions_reset_confirm_body({ name: p.name, version: p.release_version }),
		confirmLabel: m.plugin_versions_reset_confirm_action(),
		confirmIcon: 'restart-alt',
		tone: 'default'
	});
	if (!ok) return null;
	return {
		run: () => resetPluginVersion(p.id),
		successMessage: m.plugin_versions_reset_success({ name: p.name }),
		errorMessage: m.plugin_versions_reset_error({ name: p.name })
	};
}
