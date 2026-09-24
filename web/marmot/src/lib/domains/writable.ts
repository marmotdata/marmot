import { readable, type Readable } from 'svelte/store';
import { canWriteIn } from './api';
import type { DomainKind } from './types';

/**
 * Whether domains let the caller edit the entity. Starts allowed so nothing
 * flickers while enforcement is off; the server checks every write anyway.
 */
export function entityWritable(kind: DomainKind, id: string | undefined): Readable<boolean> {
	return readable(true, (set) => {
		if (!id) return;
		let stale = false;
		canWriteIn(kind, id).then((ok) => {
			if (!stale) set(ok);
		});
		return () => {
			stale = true;
		};
	});
}
