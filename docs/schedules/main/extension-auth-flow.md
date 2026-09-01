# Browser Extension Authentication Flow

## 1. Overview

Because `my.kpi.ua` uses HTTP-only and session cookies without a public OAuth API, the **Browser Extension** provides a seamless, secure, and user-friendly mechanism to bridge the student's browser session with their Telegram Bot account.

---

## 2. Authentication Handshake Protocol

```mermaid
sequenceDiagram
    autonumber
    actor User as Student
    participant Bot as Telegram Bot
    participant Server as Golang Backend Server
    participant Ext as Browser Extension (MV3)
    participant MyKPI as my.kpi.ua Portal

    User->>Bot: Send /link command
    Bot->>Server: Request pairing code for Telegram User ID
    Server-->>Bot: Return 6-digit code (e.g. 742-918, TTL: 10 min)
    Bot-->>User: Display pairing code & instructions

    User->>MyKPI: Log in via browser (password or KPI ID SSO)
    User->>Ext: Click extension icon, enter code "742-918"
    User->>Ext: Click "Sync with Telegram Bot"

    Ext->>MyKPI: chrome.cookies.getAll({domain: "my.kpi.ua"})
    MyKPI-->>Ext: Return PHPSESSID, _identity, language cookies

    Ext->>Server: POST /api/v1/auth/sync-session<br/>{pair_code: "742918", cookies: [...]}

    Server->>MyKPI: Probe GET /room/student/calendar with cookies
    MyKPI-->>Server: HTTP 200 OK (Valid student calendar HTML)

    Server->>Server: Encrypt & store session cookies linked to Telegram User ID
    Server-->>Ext: Return {success: true, student_name: "..."}
    Ext-->>User: Display "Successfully linked! You can close this tab."

    Server->>Bot: Trigger push event (Session Linked)
    Bot-->>User: Send confirmation: "✅ Розклад успішно синхронізовано!"
```

---

## 3. Data Transferred from Extension

The extension collects only the necessary authentication parameters:

```json
{
  "pair_code": "742918",
  "domain": "my.kpi.ua",
  "cookies": {
    "PHPSESSID": "eb659e5b8a5f5a4ea1d4f20ef1443af9",
    "_identity": "39233868b6449f77b496598a9824806e3adf855c...",
    "language": "uk"
  },
  "user_agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64)..."
}
```

---

## 4. Security & Privacy Guarantees

1. **Short-Lived Pairing Codes**:
   - Pairing codes expire after 10 minutes and can only be used once.
   - Brute-force rate limiting: 5 attempts per IP / Telegram user before cooldown.

2. **At-Rest Encryption**:
   - Session cookies stored in the database are encrypted at rest using AES-256-GCM with a secret master key.

3. **No Password Stored**:
   - The user's actual password or SSO credentials are never seen or stored by the bot or server.

4. **Extension Isolation**:
   - The browser extension only requests access to `https://my.kpi.ua/*` cookies and domain. No access to other tabs or websites is requested.
