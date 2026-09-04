export type ServerStatusState = 'awake' | 'sleeping' | 'transitioning' | 'down' | 'unconfigured' | 'loading';

export interface ServerStatusData {
	status: ServerStatusState;
	state?: string;
	machine_id?: string;
	machine_name?: string;
	region?: string;
	updated_at?: string;
	error?: string;
}

class ServerStatusStore {
	data = $state<ServerStatusData>({ status: 'loading' });
	isChecking = $state<boolean>(false);
	private pollInterval: any = null;

	async checkStatus() {
		this.isChecking = true;
		try {
			const res = await fetch('/api/server-status', {
				signal: AbortSignal.timeout(6000)
			});
			if (res.ok) {
				const json = await res.json();
				this.data = json;
			} else {
				this.data = { status: 'down', error: `HTTP ${res.status}` };
			}
		} catch (err: any) {
			this.data = { status: 'down', error: err?.message };
		} finally {
			this.isChecking = false;
		}
	}

	startPolling(intervalMs = 20000) {
		if (this.pollInterval) return;
		this.checkStatus();
		this.pollInterval = setInterval(() => {
			this.checkStatus();
		}, intervalMs);
	}

	stopPolling() {
		if (this.pollInterval) {
			clearInterval(this.pollInterval);
			this.pollInterval = null;
		}
	}
}

export const serverStatus = new ServerStatusStore();
