export interface RawFullCalendarEvent {
  id?: number | string;
  title: string;
  start: string; // "YYYY-MM-DDTHH:MM:SS"
  end?: string;   // "YYYY-MM-DDTHH:MM:SS"
  description?: string;
  descriptionRAW?: string;
  extendedProps?: {
    type?: string;
    teachers?: string;
    longTitle?: string;
    location?: string;
    locationRAW?: string;
    locationPDF?: string;
    locationURL?: string | null;
    groups?: string;
    timegrid?: number;
    modularity?: number;
    [key: string]: unknown;
  };
}

export interface ParsedLesson {
  date: string;       // "YYYY-MM-DD"
  start_time: string; // "HH:MM:SS"
  end_time: string;   // "HH:MM:SS"
  subject: string;
  tag: string;        // "lec", "prac", "lab", or ""
  teacher_raw: string;
  location_raw: string;
}

export interface ScheduleSyncRequest {
  pair_code?: string;
  auth_token?: string;
  telegram_id?: number;
  group_name?: string;
  lessons: ParsedLesson[];
}

export interface ScheduleSyncResponse {
  success: boolean;
  lesson_count: number;
  group_name?: string;
  enrichment_status: 'full' | 'degraded' | 'none';
  synced_at: string;
}

export interface PairVerifyResponse {
  success: boolean;
  telegram_id: number;
  auth_token: string;
  status: string;
}

export interface ExtensionStorageState {
  backendUrl?: string;
  telegramId?: number;
  authToken?: string;
  groupName?: string;
  lastSyncedAt?: string;
  lastLessonCount?: number;
}
