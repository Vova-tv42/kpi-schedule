# Chrome Web Store Listing & Publication Metadata

**Project**: KPI Schedule Sync  
**Last Updated**: 2026-09-02  
**Manifest Version**: 3  

---

## 1. Store Listing Metadata

| Field | Content |
| :--- | :--- |
| **Extension Name** | KPI Schedule Sync |
| **Short Description** | Синхронізація персонального розкладу My KPI з Telegram-ботом |
| **Category** | Productivity / Education |
| **Default Language** | Ukrainian (`uk`) |
| **Version** | `1.0.0` |

### Detailed Description
```text
KPI Schedule Sync — це офіційне розширення для студентів КПІ ім. Ігоря Сікорського, яке дозволяє в один клік синхронізувати ваш персональний розклад занять з кабінету My KPI (my.kpi.ua) до Telegram-бота.

✨ Основні можливості:
- 🔒 Безпечна синхронізація без збереження та передачі паролів або сесійних cookie на сервер.
- 📱 Авторизація через захищений 6-значний одноразовий код Telegram-бота.
- 🎓 Автоматичне визначення академічної групи, дисциплін за вибором та підгруп.
- 🏢 Збагачення розкладу аудиторіями, посиланнями на мапи корпусів та ПІБ викладачів з Campus API.
- ⚡ Швидка перевірка сесії та своєчасне сповіщення, якщо потрібен вхід до кабінету My KPI.

Як користуватися:
1. Запустіть Telegram-бота @kpi_schedule_bot та надішліть команду /link.
2. Введіть 6-значний код у розширенні для зв'язування акаунту.
3. Увійдіть у свій кабінет на https://my.kpi.ua.
4. Натисніть "Синхронізувати розклад" у розширенні — і ваш актуальний розклад з'явиться в Telegram!
```

---

## 2. Permissions Justification

| Permission | Justification (Plain English for CWS Reviewers) |
| :--- | :--- |
| `storage` | Stores the user's linked Telegram ID, client auth token, backend URL, and last sync timestamp locally on the user's device. |
| `host_permissions: https://my.kpi.ua/*` | Required to fetch the student's personal calendar page and events JSON directly in the browser using the student's existing session. |
| `host_permissions: https://api.campus.kpi.ua/*` | Used to resolve academic group directories and timetable metadata. |
| `host_permissions: http://localhost:8080/*` | Required to communicate with the local/self-hosted backend ingestion server (`/api/v1/schedule/sync`). |

---

## 3. Privacy & Data Use Disclosure

- **Does this extension collect personal data?**  
  The extension only reads the student's timetable (subject names, dates, pair times, teacher names, and classroom numbers).
- **Are credentials stored or sent?**  
  No passwords, tokens, or session cookies from `my.kpi.ua` are ever stored or transmitted to our servers.
- **Where is data transmitted?**  
  Parsed timetable JSON is transmitted exclusively to the configured KPI Schedule Bot backend server over HTTPS/HTTP.
- **Third-party analytics / Tracking**:  
  None. No trackers, telemetry, or external third-party SDKs are included.

---

## 4. Version History

- **v1.0.0 (2026-09-02)**: Initial release. Client-side extraction from `my.kpi.ua`, Telegram 6-digit pairing code verification, 403 login detection, and schedule sync ingestion integration.
