import { RawFullCalendarEvent, ParsedLesson } from '../types';

/**
 * Normalizes my.kpi.ua lesson type tags to the Campus API tag vocabulary.
 * Specifically handles "prc" -> "prac", "lec" -> "lec", "lab" -> "lab".
 */
export function normalizeTag(typeStr?: string): string {
  if (!typeStr) return '';
  const clean = typeStr.trim().toLowerCase();
  if (clean === 'lec' || clean === 'лек') return 'lec';
  if (clean === 'prc' || clean === 'прак' || clean === 'prac') return 'prac';
  if (clean === 'lab' || clean === 'лаб') return 'lab';
  return '';
}

/**
 * Strips "Викладачі: " label prefix and trims HTML or extra whitespace.
 */
export function cleanTeacher(raw?: string): string {
  if (!raw) return '';
  let cleaned = raw.replace(/^Викладач(?:і|я)?:\s*/i, '').trim();
  // Strip any accidental html tags if present
  cleaned = cleaned.replace(/<[^>]*>/g, '').trim();
  return cleaned;
}

/**
 * Cleans location string from extendedProps.locationPDF
 */
export function cleanLocation(raw?: string): string {
  if (!raw) return '';
  return raw.replace(/<[^>]*>/g, '').trim();
}

/**
 * Parses FullCalendar events into ParsedLesson array and extracts academic group name.
 */
export function parseScheduleEvents(events: RawFullCalendarEvent[]): {
  lessons: ParsedLesson[];
  detectedGroup?: string;
} {
  const lessons: ParsedLesson[] = [];
  const groupOccurrences: Map<string, number> = new Map();

  for (const ev of events) {
    if (!ev.start || !ev.title) continue;

    // ISO format: "YYYY-MM-DDTHH:MM:SS"
    const [datePart, timePart] = ev.start.split('T');
    if (!datePart || !timePart) continue;

    let endTime = '';
    if (ev.end && ev.end.includes('T')) {
      endTime = ev.end.split('T')[1] || '';
    }

    // Ensure standard HH:MM:SS format with 2-digit padded hour, minute, second
    const formatTime = (t: string) => {
      const parts = t.split(':');
      const hour = parts[0]?.padStart(2, '0') || '00';
      const minute = parts[1]?.padStart(2, '0') || '00';
      const second = parts[2]?.padStart(2, '0') || '00';
      return `${hour}:${minute}:${second}`;
    };

    const startTimeFormatted = formatTime(timePart);
    const endTimeFormatted = endTime ? formatTime(endTime) : '';

    const lesson: ParsedLesson = {
      date: datePart,
      start_time: startTimeFormatted,
      end_time: endTimeFormatted,
      subject: ev.title.trim(),
      tag: normalizeTag(ev.extendedProps?.type),
      teacher_raw: cleanTeacher(ev.descriptionRAW || ev.description),
      location_raw: cleanLocation(ev.extendedProps?.locationPDF || ev.extendedProps?.locationRAW),
    };

    lessons.push(lesson);

    // Track group occurrences to detect user's main group
    if (ev.extendedProps?.groups) {
      const rawGroups = ev.extendedProps.groups.split(',');
      for (const g of rawGroups) {
        const trimmed = g.trim();
        if (trimmed) {
          groupOccurrences.set(trimmed, (groupOccurrences.get(trimmed) || 0) + 1);
        }
      }
    }
  }

  // Find most frequent group
  let detectedGroup: string | undefined;
  let maxCount = 0;
  for (const [group, count] of groupOccurrences.entries()) {
    if (count > maxCount) {
      maxCount = count;
      detectedGroup = group;
    }
  }

  return { lessons, detectedGroup };
}
