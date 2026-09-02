import { RawFullCalendarEvent } from '../types';

export interface CalendarAuthCheckResult {
  success: boolean;
  studentId?: number;
  reason?: 'UNAUTHENTICATED' | 'NETWORK_ERROR' | 'PARSING_ERROR';
  errorMessage?: string;
}

const CALENDAR_PAGE_URL = 'https://my.kpi.ua/room/student/calendar';
const EVENTS_BASE_URL = 'https://my.kpi.ua/calendar/studevents';

/**
 * Step 1: Fetches the calendar shell HTML from my.kpi.ua to verify authentication
 * and extract the student ID for the FullCalendar events source.
 */
export async function checkLoginAndExtractStudentId(): Promise<CalendarAuthCheckResult> {
  try {
    const response = await fetch(CALENDAR_PAGE_URL, {
      method: 'GET',
      credentials: 'include',
      headers: {
        Accept: 'text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8',
      },
    });

    // Check for 401/403 or redirect to login page
    if (response.status === 401 || response.status === 403) {
      return {
        success: false,
        reason: 'UNAUTHENTICATED',
        errorMessage: 'Отримано код 403/401: Користувач не авторизований у My KPI',
      };
    }

    if (response.redirected && response.url.includes('/user/login')) {
      return {
        success: false,
        reason: 'UNAUTHENTICATED',
        errorMessage: 'Перенаправлено на сторінку входу My KPI',
      };
    }

    const html = await response.text();

    // Check if the page is a login form
    if (html.includes('id="login-form"') || html.includes('user/login') && !html.includes('studevents')) {
      return {
        success: false,
        reason: 'UNAUTHENTICATED',
        errorMessage: 'Сесія My KPI відсутня або застаріла. Будь ласка, увійдіть у кабінет.',
      };
    }

    // Extract studentId from inline FullCalendar configuration
    // Matches patterns like "events":"/calendar/studevents?id=33101"
    const match = html.match(/\/calendar\/studevents\?id=(\d+)/i) ||
                  html.match(/["']events["']\s*:\s*["'][^"']*id=(\d+)/i);

    if (!match || !match[1]) {
      return {
        success: false,
        reason: 'PARSING_ERROR',
        errorMessage: 'Не вдалося знайти FullCalendar конфігурацію зі studentId у HTML сторінці календаря',
      };
    }

    const studentId = parseInt(match[1], 10);
    return {
      success: true,
      studentId,
    };
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err);
    return {
      success: false,
      reason: 'NETWORK_ERROR',
      errorMessage: `Помилка мережі при доступі до my.kpi.ua: ${msg}`,
    };
  }
}

/**
 * Step 2: Fetches the FullCalendar JSON events feed using the authenticated student ID
 * for the requested date window (defaults to 14 days back, 120 days forward).
 */
export async function fetchCalendarEvents(
  studentId: number,
  daysBack = 14,
  daysForward = 120
): Promise<RawFullCalendarEvent[]> {
  const now = new Date();
  const startDate = new Date(now.getTime() - daysBack * 24 * 60 * 60 * 1000);
  const endDate = new Date(now.getTime() + daysForward * 24 * 60 * 60 * 1000);

  const startStr = startDate.toISOString().split('T')[0];
  const endStr = endDate.toISOString().split('T')[0];

  const url = `${EVENTS_BASE_URL}?id=${studentId}&start=${startStr}&end=${endStr}`;

  const response = await fetch(url, {
    method: 'GET',
    credentials: 'include',
    headers: {
      Accept: 'application/json',
    },
  });

  if (!response.ok) {
    throw new Error(`Помилка отримання подій розкладу: HTTP ${response.status}`);
  }

  const events = (await response.json()) as RawFullCalendarEvent[];
  if (!Array.isArray(events)) {
    throw new Error('Отримано неочікуваний формат відповіді від /calendar/studevents (очікувався масив)');
  }

  return events;
}
