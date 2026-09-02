import {
  ScheduleSyncRequest,
  ScheduleSyncResponse,
  PairVerifyResponse,
} from '../types';

export class ApiClient {
  private baseUrl: string = '';

  constructor(baseUrl: string) {
    this.setBaseUrl(baseUrl);
  }

  setBaseUrl(url: string) {
    this.baseUrl = url.replace(/\/+$/, '');
  }

  /**
   * Verifies the 6-digit pairing code from Telegram Bot /link command
   * and obtains a persistent client authentication token.
   */
  async verifyPairCode(pairCode: string): Promise<PairVerifyResponse> {
    const url = `${this.baseUrl}/api/v1/auth/pair/verify`;
    const response = await fetch(url, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ pair_code: pairCode }),
    });

    if (!response.ok) {
      let errorMsg = `Помилка перевірки коду (HTTP ${response.status})`;
      try {
        const errJson = await response.json();
        if (errJson.message) errorMsg = errJson.message;
      } catch {
        // use default error message
      }
      throw new Error(errorMsg);
    }

    return (await response.json()) as PairVerifyResponse;
  }

  /**
   * Pushes the parsed schedule to the Go backend server.
   */
  async syncSchedule(payload: ScheduleSyncRequest): Promise<ScheduleSyncResponse> {
    const url = `${this.baseUrl}/api/v1/schedule/sync`;
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    };
    if (payload.auth_token) {
      headers['X-User-Token'] = payload.auth_token;
    }

    const response = await fetch(url, {
      method: 'POST',
      headers,
      body: JSON.stringify(payload),
    });

    if (!response.ok) {
      let errorMsg = `Помилка синхронізації розкладу (HTTP ${response.status})`;
      try {
        const errJson = await response.json();
        if (errJson.message) errorMsg = errJson.message;
      } catch {
        // use default error message
      }
      throw new Error(errorMsg);
    }

    return (await response.json()) as ScheduleSyncResponse;
  }
}
