import { getStorageState, setStorageState, clearLinkedAccount } from '../lib/storage';
import { ApiClient } from '../lib/api-client';
import { checkLoginAndExtractStudentId, fetchCalendarEvents } from '../lib/fetch-schedule';
import { parseScheduleEvents } from '../lib/parse-schedule';

// DOM Elements
const connectionBadge = document.getElementById('connection-badge') as HTMLElement;
const viewUnlinked = document.getElementById('view-unlinked') as HTMLElement;
const viewLinked = document.getElementById('view-linked') as HTMLElement;

const pairForm = document.getElementById('pair-form') as HTMLFormElement;
const pairCodeInput = document.getElementById('pair-code-input') as HTMLInputElement;
const btnPair = document.getElementById('btn-pair') as HTMLButtonElement;

const displayTelegramId = document.getElementById('display-telegram-id') as HTMLElement;
const displayGroupName = document.getElementById('display-group-name') as HTMLElement;
const displayLastSync = document.getElementById('display-last-sync') as HTMLElement;

const btnSync = document.getElementById('btn-sync') as HTMLButtonElement;
const syncProgress = document.getElementById('sync-progress') as HTMLElement;
const syncProgressText = document.getElementById('sync-progress-text') as HTMLElement;

const alertBox = document.getElementById('alert-box') as HTMLElement;
const alertTitle = document.getElementById('alert-title') as HTMLElement;
const alertMessage = document.getElementById('alert-message') as HTMLElement;
const alertIcon = document.getElementById('alert-icon') as HTMLElement;
const alertActions = document.getElementById('alert-actions') as HTMLElement;

const btnToggleSettings = document.getElementById('btn-toggle-settings') as HTMLButtonElement;
const settingsPanel = document.getElementById('settings-panel') as HTMLElement;
const inputBackendUrl = document.getElementById('input-backend-url') as HTMLInputElement;
const btnSaveSettings = document.getElementById('btn-save-settings') as HTMLButtonElement;
const btnUnlink = document.getElementById('btn-unlink') as HTMLButtonElement;

let apiClient = new ApiClient('http://localhost:8080');

// Helper: Show Alert Banner
function showAlert(
  type: 'warning' | 'error' | 'success',
  title: string,
  message: string,
  actions?: { label: string; onClick: () => void; isPrimary?: boolean }[]
) {
  alertBox.className = `alert-box alert-${type}`;
  alertIcon.textContent = type === 'warning' ? '⚠️' : type === 'error' ? '❌' : '✅';
  alertTitle.textContent = title;
  alertMessage.textContent = message;

  alertActions.innerHTML = '';
  if (actions && actions.length > 0) {
    alertActions.classList.remove('hidden');
    for (const action of actions) {
      const btn = document.createElement('button');
      btn.className = `btn ${action.isPrimary ? 'btn-primary' : 'btn-secondary'} btn-small`;
      btn.textContent = action.label;
      btn.addEventListener('click', action.onClick);
      alertActions.appendChild(btn);
    }
  } else {
    alertActions.classList.add('hidden');
  }

  alertBox.classList.remove('hidden');
}

function hideAlert() {
  alertBox.classList.add('hidden');
}

// Helper: Render View according to current linked state
async function renderState() {
  const state = await getStorageState();
  apiClient.setBaseUrl(state.backendUrl || 'http://localhost:8080');
  inputBackendUrl.value = state.backendUrl || 'http://localhost:8080';

  if (state.authToken && state.telegramId) {
    // User is linked
    connectionBadge.textContent = 'Зв\'язано';
    connectionBadge.className = 'badge badge-linked';
    viewUnlinked.classList.add('hidden');
    viewLinked.classList.remove('hidden');

    displayTelegramId.textContent = String(state.telegramId);
    displayGroupName.textContent = state.groupName || 'Визначається при синхронізації';

    if (state.lastSyncedAt) {
      const date = new Date(state.lastSyncedAt);
      const count = state.lastLessonCount ?? 0;
      displayLastSync.textContent = `${date.toLocaleDateString('uk-UA')} ${date.toLocaleTimeString('uk-UA', { hour: '2-digit', minute: '2-digit' })} (${count} занять)`;
    } else {
      displayLastSync.textContent = 'Ще не синхронізовано';
    }
  } else {
    // User is unlinked
    connectionBadge.textContent = 'Не зв\'язано';
    connectionBadge.className = 'badge badge-unlinked';
    viewUnlinked.classList.remove('hidden');
    viewLinked.classList.add('hidden');
  }
}

// Auto-format 6-digit pairing code as XXX-XXX
pairCodeInput.addEventListener('input', (e) => {
  const target = e.target as HTMLInputElement;
  const digits = target.value.replace(/\D/g, '').slice(0, 6);
  if (digits.length > 3) {
    target.value = `${digits.slice(0, 3)}-${digits.slice(3)}`;
  } else {
    target.value = digits;
  }
});

// Pairing Form Submit
pairForm.addEventListener('submit', async (e) => {
  e.preventDefault();
  hideAlert();

  const rawCode = pairCodeInput.value.replace(/\D/g, '');
  if (rawCode.length !== 6) {
    showAlert('warning', 'Некоректний код', 'Код повинен містити 6 цифр.');
    return;
  }

  btnPair.disabled = true;
  btnPair.textContent = 'Перевірка...';

  try {
    const res = await apiClient.verifyPairCode(rawCode);
    if (res.success && res.auth_token) {
      await setStorageState({
        telegramId: res.telegram_id,
        authToken: res.auth_token,
      });
      pairCodeInput.value = '';
      await renderState();
      showAlert('success', 'Акаунт зв\'язано!', 'Тепер ви можете синхронізувати ваш розклад.');
    } else {
      showAlert('error', 'Помилка', 'Не вдалося перевірити код зв\'язування.');
    }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err);
    showAlert('error', 'Помилка авторизації', msg);
  } finally {
    btnPair.disabled = false;
    btnPair.textContent = 'Зв\'язати акаунт';
  }
});

// Schedule Sync Click
btnSync.addEventListener('click', async () => {
  hideAlert();
  btnSync.disabled = true;
  syncProgress.classList.remove('hidden');

  try {
    const state = await getStorageState();
    if (!state.authToken) {
      showAlert('error', 'Помилка', 'Спочатку підключіть ваш Telegram акаунт.');
      return;
    }

    // Step 1: Check login & extract studentId
    syncProgressText.textContent = '1/4: Перевірка авторизації в My KPI...';
    const authCheck = await checkLoginAndExtractStudentId();

    if (!authCheck.success) {
      if (authCheck.reason === 'UNAUTHENTICATED') {
        showAlert(
          'warning',
          'Потрібен вхід у My KPI',
          'Ви не авторизовані у системі My KPI. Увійдіть у свій акаунт та спробуйте синхронізацію знову.',
          [
            {
              label: '🔑 Увійти в My KPI',
              isPrimary: true,
              onClick: () => {
                chrome.tabs.create({ url: 'https://my.kpi.ua/user/login' });
              },
            },
          ]
        );
        return;
      }

      showAlert('error', 'Помилка My KPI', authCheck.errorMessage || 'Не вдалося завантажити сторінку календаря.');
      return;
    }

    const studentId = authCheck.studentId!;

    // Step 2: Fetch calendar events JSON
    syncProgressText.textContent = '2/4: Завантаження розкладу...';
    const rawEvents = await fetchCalendarEvents(studentId);

    // Step 3: Parse and normalize events
    syncProgressText.textContent = '3/4: Обробка занять...';
    const { lessons, detectedGroup } = parseScheduleEvents(rawEvents);

    if (lessons.length === 0) {
      showAlert('warning', 'Порожній розклад', 'Не знайдено жодного заняття у календарі My KPI на поточний семестр.');
      return;
    }

    // Step 4: Push to backend
    syncProgressText.textContent = '4/4: Збереження на сервері...';
    const syncResult = await apiClient.syncSchedule({
      auth_token: state.authToken,
      group_name: detectedGroup || state.groupName,
      lessons,
    });

    // Save updated info in local storage
    await setStorageState({
      lastSyncedAt: new Date().toISOString(),
      lastLessonCount: syncResult.lesson_count,
      groupName: syncResult.group_name || detectedGroup || state.groupName,
    });

    await renderState();

    const enrichmentNote = syncResult.enrichment_status === 'full'
      ? ' (збагачено викладачами та аудиторіями Campus)'
      : '';

    showAlert(
      'success',
      'Розклад синхронізовано!',
      `Успішно збережено ${syncResult.lesson_count} занять${enrichmentNote}. Тепер розклад доступний у Telegram-боті.`
    );
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err);
    showAlert('error', 'Помилка синхронізації', msg);
  } finally {
    btnSync.disabled = false;
    syncProgress.classList.add('hidden');
  }
});

// Settings Toggle
btnToggleSettings.addEventListener('click', () => {
  settingsPanel.classList.toggle('hidden');
});

// Save Settings
btnSaveSettings.addEventListener('click', async () => {
  const newUrl = inputBackendUrl.value.trim();
  if (newUrl) {
    await setStorageState({ backendUrl: newUrl });
    apiClient.setBaseUrl(newUrl);
    settingsPanel.classList.add('hidden');
    showAlert('success', 'Збережено', 'Адресу сервера успішно оновлено.');
  }
});

// Unlink Account
btnUnlink.addEventListener('click', async () => {
  if (confirm('Ви дійсно бажаєте від\'єднати Telegram акаунт від цього браузера?')) {
    await clearLinkedAccount();
    settingsPanel.classList.add('hidden');
    hideAlert();
    await renderState();
    showAlert('warning', 'Від\'єднано', 'Telegram акаунт від\'єднано від розширення.');
  }
});

// Initial Render
renderState();
