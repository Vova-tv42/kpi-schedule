import { DEFAULT_BACKEND_URL, getStorageState, setStorageState } from '../lib/storage';

// Service worker lifecycle handling
chrome.runtime.onInstalled.addListener(async () => {
  const state = await getStorageState();
  if (!state.backendUrl || state.backendUrl === 'http://localhost:8080') {
    await setStorageState({ backendUrl: DEFAULT_BACKEND_URL });
  }
});
