<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import { m } from '$lib/paraglide/messages';
	import { BTN_SECONDARY, INPUT } from '$lib/ui/classes';
	import { notificationTypeOptions, providerOptions } from '$lib/teams/webhooks';

	export interface PendingWebhook {
		key: string;
		name: string;
		provider: 'slack' | 'discord' | 'generic';
		webhook_url: string;
		notification_types: string[];
		enabled: boolean;
	}

	let { webhooks = $bindable([]) }: { webhooks?: PendingWebhook[] } = $props();

	const labelClass = 'block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1';
	const inputClass = INPUT;

	function addWebhook() {
		webhooks = [
			...webhooks,
			{
				key: crypto.randomUUID(),
				name: '',
				provider: 'slack',
				webhook_url: '',
				notification_types: [],
				enabled: true
			}
		];
	}

	function removeWebhook(key: string) {
		webhooks = webhooks.filter((w) => w.key !== key);
	}

	function toggleNotificationType(key: string, type: string) {
		webhooks = webhooks.map((w) => {
			if (w.key !== key) return w;
			const next = w.notification_types.includes(type)
				? w.notification_types.filter((t) => t !== type)
				: [...w.notification_types, type];
			return { ...w, notification_types: next };
		});
	}
</script>

{#if webhooks.length > 0}
	<div class="space-y-4">
		{#each webhooks as w, i (w.key)}
			<div
				class="overflow-hidden rounded-lg border border-gray-200 bg-gray-50/60 dark:border-gray-700 dark:bg-gray-900/30"
			>
				<div
					class="flex items-center justify-between border-b border-gray-200 bg-white px-4 py-2.5 dark:border-gray-700 dark:bg-gray-800/40"
				>
					<div class="flex items-center gap-2 text-sm font-medium text-gray-900 dark:text-gray-100">
						<IconifyIcon
							icon={providerOptions().find((p) => p.value === w.provider)?.icon ?? 'mdi:webhook'}
							class="h-4 w-4"
						/>
						{w.name || m.teams_integration_fallback_name({ number: i + 1 })}
					</div>
					<button
						type="button"
						onclick={() => removeWebhook(w.key)}
						class="-m-1 p-1 text-gray-400 hover:text-red-500 dark:hover:text-red-400"
						aria-label={m.teams_remove_integration_aria()}
					>
						<IconifyIcon icon="material-symbols:close" class="h-4 w-4" />
					</button>
				</div>
				<div class="space-y-4 p-4">
					<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
						<div>
							<label for="wh-name-{w.key}" class={labelClass}>
								{m.common_name()} <span class="text-red-500">*</span>
							</label>
							<input
								id="wh-name-{w.key}"
								type="text"
								bind:value={w.name}
								placeholder={m.teams_webhook_name_placeholder()}
								class={inputClass}
							/>
						</div>
						<div>
							<label for="wh-provider-{w.key}" class={labelClass}>
								{m.teams_webhook_provider_label()}
							</label>
							<select id="wh-provider-{w.key}" bind:value={w.provider} class={inputClass}>
								{#each providerOptions() as opt (opt.value)}
									<option value={opt.value}>{opt.label}</option>
								{/each}
							</select>
						</div>
					</div>

					<div>
						<label for="wh-url-{w.key}" class={labelClass}>
							{m.teams_webhook_url_label()} <span class="text-red-500">*</span>
						</label>
						<input
							id="wh-url-{w.key}"
							type="url"
							bind:value={w.webhook_url}
							placeholder="https://hooks.slack.com/services/..."
							class="{inputClass} font-mono"
						/>
					</div>

					<div>
						<div class={labelClass}>
							{m.teams_notification_types_label()} <span class="text-red-500">*</span>
						</div>
						<div class="grid grid-cols-2 gap-1.5 sm:grid-cols-3">
							{#each notificationTypeOptions() as opt (opt.type)}
								{@const selected = w.notification_types.includes(opt.type)}
								<button
									type="button"
									onclick={() => toggleNotificationType(w.key, opt.type)}
									class="flex items-center gap-1.5 rounded-md border px-2 py-1.5 text-left text-xs transition-colors
										{selected
										? 'border-earthy-terracotta-500 bg-earthy-terracotta-50 text-earthy-terracotta-700 dark:bg-earthy-terracotta-900/20 dark:text-earthy-terracotta-300'
										: 'border-gray-200 text-gray-600 hover:border-gray-300 dark:border-gray-600 dark:text-gray-400'}"
								>
									<IconifyIcon icon={opt.icon} class="h-3.5 w-3.5" />
									{opt.label}
								</button>
							{/each}
						</div>
					</div>

					<label
						class="inline-flex cursor-pointer items-center gap-2 text-xs text-gray-700 dark:text-gray-300"
					>
						<input
							type="checkbox"
							bind:checked={w.enabled}
							class="rounded border-gray-300 text-earthy-terracotta-600 focus:ring-earthy-terracotta-500 dark:border-gray-600"
						/>
						{m.common_enabled()}
					</label>
				</div>
			</div>
		{/each}
	</div>
{/if}

<button type="button" onclick={addWebhook} class="{BTN_SECONDARY} w-full">
	<IconifyIcon icon="material-symbols:add" class="h-4 w-4" />
	{m.teams_add_integration()}
</button>
