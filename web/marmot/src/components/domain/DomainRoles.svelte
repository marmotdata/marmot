<script lang="ts">
	import Icon from '@iconify/svelte';
	import OwnerSelector from '$components/shared/OwnerSelector.svelte';
	import Button from '$components/ui/Button.svelte';
	import { toasts } from '$lib/stores/toast';
	import { m } from '$lib/paraglide/messages';
	import type { Owner } from '$lib/glossary/types';
	import type { DomainRole, RoleAssignment, SubjectType } from '$lib/domains/types';
	import { errorMessage, grantRole, listRoles, revokeRole } from '$lib/domains/api';

	let { domainId, canAdmin = false }: { domainId: string; canAdmin?: boolean } = $props();

	let roles = $state<RoleAssignment[] | null>(null);
	let subjects = $state<Owner[]>([]);
	let role = $state<DomainRole>('steward');
	let busy = $state(false);

	const roleOptions: { value: DomainRole; label: () => string; hint: () => string }[] = [
		{ value: 'steward', label: m.domains_role_steward, hint: m.domains_role_steward_hint },
		{
			value: 'domain_admin',
			label: m.domains_role_domain_admin,
			hint: m.domains_role_domain_admin_hint
		},
		{ value: 'reader', label: m.domains_role_reader, hint: m.domains_role_reader_hint }
	];

	const roleLabel = (r: DomainRole) => roleOptions.find((o) => o.value === r)?.label() ?? r;

	const subjectIcon: Record<SubjectType, string> = {
		user: 'material-symbols:person-outline',
		team: 'material-symbols:group-outline',
		service_account: 'material-symbols:smart-toy-outline'
	};

	const roleClass: Record<DomainRole, string> = {
		domain_admin:
			'bg-earthy-terracotta-100 text-earthy-terracotta-800 dark:bg-earthy-terracotta-900/40 dark:text-earthy-terracotta-100',
		steward: 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-200',
		reader: 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-200'
	};

	async function load(id: string) {
		try {
			roles = await listRoles(id);
		} catch (error) {
			roles = [];
			toasts.error(errorMessage(error));
		}
	}

	$effect(() => {
		roles = null;
		subjects = [];
		load(domainId);
	});

	async function grant() {
		if (subjects.length === 0) return;
		busy = true;
		let granted = 0;
		for (const subject of subjects) {
			try {
				await grantRole(domainId, { subject_type: subject.type, subject_id: subject.id, role });
				granted++;
			} catch (error) {
				toasts.error(`${subject.name}: ${errorMessage(error)}`);
			}
		}
		if (granted > 0) toasts.success(m.domains_roles_granted());
		subjects = [];
		busy = false;
		await load(domainId);
	}

	async function revoke(assignment: RoleAssignment) {
		busy = true;
		try {
			await revokeRole(domainId, assignment.id);
			toasts.success(m.domains_roles_revoked());
			await load(domainId);
		} catch (error) {
			toasts.error(errorMessage(error));
		} finally {
			busy = false;
		}
	}
</script>

<section
	class="space-y-4 border-t border-gray-200 pt-5 dark:border-gray-700"
	aria-labelledby="domain-roles-heading"
>
	<div>
		<h3 id="domain-roles-heading" class="text-base font-semibold text-gray-900 dark:text-gray-100">
			{m.domains_roles_heading()}
		</h3>
		<p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{m.domains_roles_hint()}</p>
	</div>

	{#if roles === null}
		<div class="h-5 w-5 animate-spin rounded-full border-b-2 border-earthy-terracotta-700"></div>
	{:else if roles.length === 0}
		<p class="text-sm italic text-gray-500 dark:text-gray-400">{m.domains_roles_empty()}</p>
	{:else}
		<ul
			class="divide-y divide-gray-100 rounded-md border border-gray-200 dark:divide-gray-700 dark:border-gray-700"
		>
			{#each roles as assignment (assignment.id)}
				{@const name = assignment.subject_missing
					? m.domains_roles_subject_missing()
					: (assignment.subject_name ?? assignment.subject_id)}
				<li class="flex items-center justify-between gap-3 px-4 py-2.5">
					<div class="flex min-w-0 items-center gap-3">
						<Icon
							icon={subjectIcon[assignment.subject_type]}
							class="h-5 w-5 shrink-0 text-gray-400"
						/>
						<div class="min-w-0">
							<p
								class="truncate text-sm text-gray-900 dark:text-gray-100 {assignment.subject_missing
									? 'italic text-gray-500'
									: ''}"
							>
								{name}
							</p>
							{#if assignment.inherited}
								<p class="text-xs text-gray-500 dark:text-gray-400">
									{m.domains_roles_inherited({ domain: assignment.domain_name })}
								</p>
							{/if}
						</div>
					</div>
					<div class="flex shrink-0 items-center gap-2">
						<span
							class="rounded-full px-2.5 py-0.5 text-xs font-medium {roleClass[assignment.role]}"
							>{roleLabel(assignment.role)}</span
						>
						{#if canAdmin && !assignment.inherited}
							<button
								type="button"
								class="rounded p-1 text-gray-400 hover:text-red-600 disabled:opacity-50 dark:hover:text-red-400"
								aria-label={m.domains_roles_revoke({ role: roleLabel(assignment.role), name })}
								title={m.domains_roles_revoke({ role: roleLabel(assignment.role), name })}
								disabled={busy}
								onclick={() => revoke(assignment)}
							>
								<Icon icon="material-symbols:delete-outline" class="h-4 w-4" />
							</button>
						{/if}
					</div>
				</li>
			{/each}
		</ul>
	{/if}

	{#if canAdmin}
		<div class="space-y-3 rounded-md bg-gray-50 p-4 dark:bg-gray-900/40">
			<div>
				<span class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
					>{m.domains_roles_subjects()}</span
				>
				<OwnerSelector selectedOwners={subjects} onChange={(owners) => (subjects = owners)} />
			</div>
			<fieldset>
				<legend class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
					{m.domains_roles_role()}
				</legend>
				<div class="grid gap-2 sm:grid-cols-3">
					{#each roleOptions as option (option.value)}
						<label
							class="flex cursor-pointer flex-col rounded-md border px-3 py-2 text-sm transition-colors {role ===
							option.value
								? 'border-earthy-terracotta-600 bg-white dark:bg-gray-800'
								: 'border-gray-200 bg-white hover:border-gray-300 dark:border-gray-700 dark:bg-gray-800'}"
						>
							<span class="flex items-center gap-2">
								<input
									type="radio"
									name="domain-role"
									value={option.value}
									bind:group={role}
									class="text-earthy-terracotta-700 focus:ring-earthy-terracotta-600"
								/>
								<span class="font-medium text-gray-900 dark:text-gray-100">{option.label()}</span>
							</span>
							<span class="mt-1 text-xs text-gray-500 dark:text-gray-400">{option.hint()}</span>
						</label>
					{/each}
				</div>
			</fieldset>
			<Button
				icon="material-symbols:person-add-outline"
				text={m.domains_roles_grant()}
				loading={busy}
				disabled={busy || subjects.length === 0}
				click={grant}
			/>
		</div>
	{/if}
</section>
