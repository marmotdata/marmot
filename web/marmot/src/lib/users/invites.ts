import { fetchApi } from '$lib/api';
import { parseApiError } from '$lib/stores/toast';
import type { Invitation, InviteInfo } from './types';

/** An invitation link the person can open, resolved against this page when the server does not know its own address. */
export function inviteLink(inv: Pick<Invitation, 'invite_url'>): string {
	return new URL(inv.invite_url, window.location.origin).toString();
}

export async function inviteMember(input: {
	email: string;
	name?: string;
	role_names: string[];
	team_ids?: string[];
}): Promise<Invitation> {
	const res = await fetchApi('/users/invites', { method: 'POST', body: JSON.stringify(input) });
	if (!res.ok) throw new Error((await parseApiError(res)).message);
	return res.json();
}

/** A new link for a pending member; the old one stops working. */
export async function renewInvite(userId: string): Promise<Invitation> {
	const res = await fetchApi(`/users/invites/${userId}/renew`, { method: 'POST' });
	if (!res.ok) throw new Error((await parseApiError(res)).message);
	return res.json();
}

/** Withdraws the invitation and the pending account it created. */
export async function withdrawInvite(userId: string): Promise<void> {
	const res = await fetchApi(`/users/invites/${userId}`, { method: 'DELETE' });
	if (!res.ok) throw new Error((await parseApiError(res)).message);
}

export class InviteError extends Error {
	constructor(
		message: string,
		readonly status: number
	) {
		super(message);
	}
}

/** Reads an invitation without signing in: the token is the credential. */
export async function lookupInvite(token: string): Promise<InviteInfo> {
	const res = await fetchApi(`/invites/${encodeURIComponent(token)}`, { skipAuth: true });
	if (!res.ok) throw new InviteError((await parseApiError(res)).message, res.status);
	return res.json();
}

/** Sets the password and returns a session token for the new member. */
export async function acceptInvite(
	token: string,
	password: string,
	name: string
): Promise<{ access_token: string }> {
	const res = await fetchApi(`/invites/${encodeURIComponent(token)}/accept`, {
		method: 'POST',
		skipAuth: true,
		body: JSON.stringify({ password, name })
	});
	if (!res.ok) throw new InviteError((await parseApiError(res)).message, res.status);
	return res.json();
}
