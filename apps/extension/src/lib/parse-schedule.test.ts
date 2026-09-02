import { describe, expect, it } from 'bun:test';
import { parseScheduleEvents, normalizeTag, cleanTeacher } from './parse-schedule';
import { RawFullCalendarEvent } from '../types';

describe('normalizeTag', () => {
  it('normalizes type tags correctly', () => {
    expect(normalizeTag('lec')).toBe('lec');
    expect(normalizeTag('лек')).toBe('lec');
    expect(normalizeTag('prc')).toBe('prac');
    expect(normalizeTag('прак')).toBe('prac');
    expect(normalizeTag('prac')).toBe('prac');
    expect(normalizeTag('lab')).toBe('lab');
    expect(normalizeTag('лаб')).toBe('lab');
    expect(normalizeTag('unknown')).toBe('');
    expect(normalizeTag(undefined)).toBe('');
  });
});

describe('cleanTeacher', () => {
  it('strips prefix and cleans HTML tags', () => {
    expect(cleanTeacher('Викладачі: Колумбет В. П.')).toBe('Колумбет В. П.');
    expect(cleanTeacher('<i><span title="test">Гуменний Д. О.</span></i>')).toBe('Гуменний Д. О.');
    expect(cleanTeacher('')).toBe('');
  });
});

describe('parseScheduleEvents', () => {
  it('correctly parses fullcalendar events into ParsedLesson array', () => {
    const fixture: RawFullCalendarEvent[] = [
      {
        id: 1019849,
        title: 'Технології DevOps',
        start: '2026-09-19T08:30:00',
        end: '2026-09-19T10:05:00',
        descriptionRAW: 'Викладачі: Колумбет В. П.',
        extendedProps: {
          type: 'lec',
          locationPDF: 'lec., Онлайн Zoom',
          groups: 'ТВ-41, ТВ-42, ТВ-43',
        },
      },
      {
        id: 1019850,
        title: 'Процеси розробки ПЗ',
        start: '2026-09-19T10:25:00',
        end: '2026-09-19T12:00:00',
        descriptionRAW: 'Викладачі: Гуменний Д. О.',
        extendedProps: {
          type: 'prc',
          locationPDF: '18-402',
          groups: 'ТВ-42',
        },
      },
    ];

    const { lessons, detectedGroup } = parseScheduleEvents(fixture);

    expect(lessons).toHaveLength(2);
    expect(lessons[0]).toEqual({
      date: '2026-09-19',
      start_time: '08:30:00',
      end_time: '10:05:00',
      subject: 'Технології DevOps',
      tag: 'lec',
      teacher_raw: 'Колумбет В. П.',
      location_raw: 'lec., Онлайн Zoom',
    });

    expect(lessons[1]).toEqual({
      date: '2026-09-19',
      start_time: '10:25:00',
      end_time: '12:00:00',
      subject: 'Процеси розробки ПЗ',
      tag: 'prac',
      teacher_raw: 'Гуменний Д. О.',
      location_raw: '18-402',
    });

    expect(detectedGroup).toBe('ТВ-42');
  });
});
