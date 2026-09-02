import { ExtensionStorageState } from '../types';

const DEFAULT_BACKEND_URL = 'http://localhost:8080';

export async function getStorageState(): Promise<ExtensionStorageState> {
  const data = await chrome.storage.local.get([
    'backendUrl',
    'telegramId',
    'authToken',
    'groupName',
    'lastSyncedAt',
    'lastLessonCount',
  ]);

  return {
    backendUrl: data.backendUrl || DEFAULT_BACKEND_URL,
    telegramId: data.telegramId,
    authToken: data.authToken,
    groupName: data.groupName,
    lastSyncedAt: data.lastSyncedAt,
    lastLessonCount: data.lastLessonCount,
  };
}

export async function setStorageState(state: Partial<ExtensionStorageState>): Promise<void> {
  await chrome.storage.local.set(state);
}

export async function clearLinkedAccount(): Promise<void> {
  await chrome.storage.local.remove([
    'telegramId',
    'authToken',
    'groupName',
    'lastSyncedAt',
    'lastLessonCount',
  ]);
}
