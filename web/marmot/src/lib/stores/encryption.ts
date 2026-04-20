import { writable } from 'svelte/store';

export const encryptionConfigured = writable(true);
export const allowUnencrypted = writable(false);
