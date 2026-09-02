import { getStorageState, setStorageState } from '../lib/storage';

// Service worker lifecycle handling
chrome.runtime.onInstalled.addListener(async (details) => {
  if (details.reason === 'install') {
    const state = await getStorageState();
    if (!state.backendUrl) {
      await setStorageState({ backendUrl: 'http://localhost:8080' });
    }
  }
});
