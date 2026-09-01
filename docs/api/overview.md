# Backend REST API Overview

## 1. Design Principles

The Golang backend API serves as the core coordinator between the Telegram Bot, Browser Extension, and external KPI schedule services.

- **Base URL Prefix**: `/api/v1`
- **Format**: JSON (`Content-Type: application/json; charset=utf-8`)
- **Authentication**:
  - Internal bot-to-server communication: `X-Bot-Token` or JWT bearer token.
  - Browser extension pairing: 6-digit short-lived one-time pairing code.
- **HTTP Status Codes**:
  - `200 OK`: Request succeeded.
  - `400 Bad Request`: Invalid parameters or invalid pairing code.
  - `401 Unauthorized`: Session expired or invalid bot token.
  - `404 Not Found`: Group or user not found.
  - `500 Internal Server Error`: Unrecoverable scraping or merging failure (accompanied by a structured error code).

---

## 2. API Route Summary

| Method | Endpoint | Description | Auth Required |
| :--- | :--- | :--- | :--- |
| **POST** | `/api/v1/auth/pair-code` | Generate a 6-digit link code for Telegram User | Bot API Token |
| **POST** | `/api/v1/auth/sync-session` | Submit `my.kpi.ua` cookies from Browser Extension | Pair Code |
| **GET** | `/api/v1/auth/status/{telegramId}` | Check if user's session is active or expired | Bot API Token |
| **DELETE**| `/api/v1/auth/unlink/{telegramId}` | Remove stored cookies and unbind user | Bot API Token |
| **GET** | `/api/v1/schedule/today` | Fetch combined schedule for today | Telegram User Header / Bot Token |
| **GET** | `/api/v1/schedule/tomorrow` | Fetch combined schedule for tomorrow | Telegram User Header / Bot Token |
| **GET** | `/api/v1/schedule/week` | Fetch combined 2-week schedule | Telegram User Header / Bot Token |
| **GET** | `/api/v1/schedule/date` | Fetch combined schedule for a specific date | Telegram User Header / Bot Token |
| **GET** | `/api/v1/groups` | Search & list all academic groups | Public / Bot Token |
| **GET** | `/api/v1/time/current` | Query current academic week & day | Public / Bot Token |
