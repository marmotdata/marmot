import { strict as assert } from 'node:assert';
import { test } from 'node:test';
import { loginRedirect } from '../src/lib/auth/loginRedirect.ts';

test('login redirect stays on the current origin', () => {
	const origin = 'https://catalog.example';
	assert.equal(loginRedirect('/assets?tab=docs#section', origin), '/assets?tab=docs#section');
	assert.equal(loginRedirect('https://catalog.example/profile', origin), '/profile');
	for (const unsafe of [
		'//attacker.example',
		'/\\attacker.example',
		'https://attacker.example',
		'javascript:alert(1)'
	]) {
		assert.equal(loginRedirect(unsafe, origin), '/');
	}
});
