import { json } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async () => {
	const flyToken = env.FLY_API_TOKEN || process.env.FLY_API_TOKEN;
	const appName = env.FLY_APP_NAME || process.env.FLY_APP_NAME || 'kpi-schedule';

	if (!flyToken) {
		return json({
			status: 'unconfigured',
			message: 'FLY_API_TOKEN is not set in environment variables',
			app_name: appName
		});
	}

	try {
		const res = await fetch(`https://api.machines.dev/v1/apps/${appName}/machines`, {
			headers: {
				Authorization: `Bearer ${flyToken}`,
				'Content-Type': 'application/json'
			},
			signal: AbortSignal.timeout(5000)
		});

		if (!res.ok) {
			const errorText = await res.text();
			return json(
				{
					status: 'unknown',
					error: `Fly API error (${res.status}): ${errorText}`
				},
				{ status: res.status }
			);
		}

		const machines = await res.json();
		if (!Array.isArray(machines) || machines.length === 0) {
			return json({
				status: 'down',
				message: 'No machines provisioned for this app',
				app_name: appName
			});
		}

		const primary = machines[0];
		const state = primary.state; // 'started', 'stopped', 'stopping', etc.

		let status: 'awake' | 'sleeping' | 'transitioning' | 'down' = 'down';
		if (state === 'started') {
			status = 'awake';
		} else if (state === 'stopped') {
			status = 'sleeping';
		} else if (state === 'stopping' || state === 'starting') {
			status = 'transitioning';
		}

		return json({
			status,
			state,
			machine_id: primary.id,
			machine_name: primary.name,
			region: primary.region,
			updated_at: primary.updated_at,
			app_name: appName
		});
	} catch (err: any) {
		return json(
			{
				status: 'down',
				error: err?.message || 'Failed to reach Fly Machines API'
			},
			{ status: 500 }
		);
	}
};
