# Fly.io Scale-to-Zero & 15-Minute Idle Architecture

This document describes the hosting architecture on **Fly.io Fly Machines**, specifically how the server sleeps after 15 minutes of inactivity to minimize hosting costs, and wakes up automatically upon receiving any incoming HTTP request (API call, browser extension sync, or Telegram webhook).

---

## 1. Motivation & Cost Model

Fly.io charges for Fly Machine compute (CPU and RAM) only while a Machine is in the `started` state. When a Machine is in the `stopped` state, compute billing drops to **$0.00**. The only persistent cost is the attached Fly Volume for SQLite data, which costs ~$0.15/month for a 1 GB disk.

Because schedule queries and Telegram interactions occur in bursts (e.g. morning check-ins before classes or students syncing schedules), running the server continuously 24/7 is unnecessary. Putting the server to sleep after 15 minutes of inactivity reduces compute costs by 80–90% while keeping response latency sub-second when active.

---

## 2. Architecture Overview

```
                          [ Incoming HTTP Request ]
                   (Telegram Webhook / Extension Sync / API)
                                     │
                                     ▼
                           ┌──────────────────┐
                           │    Fly Proxy     │
                           │ (Anycast Edge)   │
                           └─────────┬────────┘
                                     │
                  Machine Stopped? ──┴── Machine Running?
                         │                      │
                  ┌──────▼──────┐               │
                  │ Start VM    │               │
                  │ (< 500ms)   │               │
                  └──────┬──────┘               │
                         └───────────┬──────────┘
                                     │
                                     ▼
                        ┌────────────────────────┐
                        │   Go Server Process    │
                        │   (internal_port 8080) │
                        └────────────┬───────────┘
                                     │
                        ┌────────────▼───────────┐
                        │     idle.Watcher       │
                        │ (Resets 15m countdown) │
                        └────────────┬───────────┘
                                     │
                 ┌───────────────────┴───────────────────┐
                 │                                       │
                 ▼                                       ▼
        [ Normal Requests ]                     [ Inactive for >= 15m ]
        - Resets idle timer                     - Triggers graceful shutdown
        - In-flight counter > 0                 - Flushes SQLite DB & closes
        - Excludes /healthz probes              - Process exits with code 0
                                                - VM enters "stopped" state
```

---

## 3. Why In-App Idle Shutdown Instead of Fly Proxy Autostop

Fly.io provides a built-in `auto_stop_machines` option in `fly.toml` (`"stop"` or `"suspend"`). However:
1. **Unconfigurable timer**: Fly Proxy's excess-capacity loop runs every few minutes and shuts down idle machines aggressively after approximately 3–5 minutes of silence. It does not provide an idle timeout setting in `fly.toml`.
2. **Premature termination**: A 3–5 minute timeout disrupts users who pause briefly during multi-step bot interactions (such as setting custom course URLs or pairing academic groups).

To enforce an **exact 15-minute idle threshold**, the architecture combines Fly Proxy and in-app management:
- **`auto_stop_machines = "off"`**: Disables Fly Proxy's aggressive 3–5 minute stoppage.
- **`auto_start_machines = true`**: Fly Proxy continues listening at the edge and wakes the Machine upon incoming requests.
- **`min_machines_running = 0`**: Permits zero active instances when idle.
- **`[[restart]] policy = "on-failure"`**: When the server cleanly exits (`exit 0`), the Machine supervisor transitions it to `stopped` without restarting.
- **`internal/idle.Watcher`**: In-process Go watcher tracking `lastActivity` and concurrent in-flight requests.

---

## 4. In-App Idle Watcher (`internal/idle`)

The `idle.Watcher` is mounted as an HTTP middleware on the root router:

1. **Activity Tracking**:
   - Maintains an atomic `activeRequests` counter and `lastActivity` timestamp.
   - Any client or webhook request increments the active counter on arrival, updates `lastActivity`, and decrements on completion.
   - The server will **never** shut down while a request is in flight.

2. **Health Check Exclusion**:
   - `/healthz` is excluded from activity tracking.
   - External monitoring probes or Fly Proxy health checks hitting `/healthz` do not reset the idle timer, ensuring health checks do not keep the server awake indefinitely.

3. **Graceful Shutdown**:
   - When `time.Since(lastActivity) >= IDLE_TIMEOUT` (default: `15m`) and `activeRequests == 0`, `idle.Watcher` signals its `Done()` channel.
   - `main.go` initiates a clean shutdown:
     1. Stops the Telegram bot dispatcher/updater (`tgBot.Stop()`).
     2. Calls `srv.Shutdown(ctx)` with a 10-second deadline to complete pending operations.
     3. Closes SQLite connection (`db.Close()`), ensuring all WAL pages are flushed.
     4. Process exits with status code `0`.

4. **Configurability**:
   - Controlled via `IDLE_TIMEOUT` (e.g. `15m`, `30m`).
   - If unset or `<= 0` (default in local development), the idle watcher is disabled, preventing disruption during active coding.

---

## 5. Wake-on-Request Flow

When a Machine is `stopped` and an HTTP request arrives:
1. **Fly Proxy Interception**: Fly Proxy holds the incoming TCP/TLS connection open.
2. **Cold Start**: Fly Proxy boots the Firecracker microVM from the mounted Fly Volume.
3. **Application Initialization**:
   - Distroless Go binary boots in ~200–400ms.
   - SQLite migrations verify schema in ~5ms.
   - Server binds `:8080`.
4. **Proxy Forwarding**: Fly Proxy routes the original HTTP request to `:8080`.
5. **Response**: The client or Telegram webhook receives a normal response with a minor (~500ms–1s) initial cold-start latency. Subsequent requests within the 15-minute window experience zero cold-start delay.

---

## 6. Cold-Start Telegram Webhook Optimization

During a cold boot triggered by a Telegram user message:
- Telegram dispatches an HTTP POST to `/api/v1/telegram/webhook` and waits for `200 OK`.
- **Previous Bottleneck**: The server previously executed 3 synchronous HTTPS requests to `api.telegram.org` (`SetMyCommands` x2, `SetWebhook`) before listening on `:8080`, adding 1–2 seconds of latency and risking timeout.
- **`DropPendingUpdates` Bug**: Setting `DropPendingUpdates: true` on every boot would discard the very message that triggered the wake-up.
- **Optimization**:
  - `internal/bot.RegisterWebhook` first checks SQLite cache (`campus_cache` table) for `telegram_webhook_registration`.
  - If the registered URL matches `TELEGRAM_WEBHOOK_URL`, external Telegram API calls are skipped entirely.
  - The HTTP server starts listening on `:8080` in **< 50ms**, immediately servicing the Telegram update.
  - `DropPendingUpdates` is set to `false`, guaranteeing zero lost messages.

---

## 7. SQLite Persistence on Scale-to-Zero

Because Fly Machines stop and restart frequently under scale-to-zero:
- **Fly Volumes**: The SQLite file lives at `/data/kpi.db` on a mounted persistent NVMe volume (`[mounts]` in `fly.toml`).
- **Data Safety**: Machine stops do not touch or destroy the volume.
- **WAL Checkpointing**: Graceful shutdown in `main.go` closes the database before exit, flushing dirty pages and checkpoints safely.
- **Disk Cache**: The `campus_cache` table (which caches Campus API responses) remains on disk across sleep cycles, avoiding cold-cache stampedes on `api.campus.kpi.ua` after waking.

---

## 8. Deployment Runbook

### Prerequisites
Install the Fly CLI:
```bash
curl -L https://fly.io/install.sh | sh
```

### 1. Launch & Volume Creation
From the `apps/server/` directory:
```bash
# 1. Create a persistent volume in the primary region (fra = Frankfurt)
fly volumes create kpi_data --size 1 --region fra

# 2. Configure production secrets
fly secrets set \
  INTERNAL_API_TOKEN="$(openssl rand -hex 32)" \
  TELEGRAM_BOT_TOKEN="<your-telegram-bot-token>" \
  TELEGRAM_WEBHOOK_URL="https://<your-app-name>.fly.dev/api/v1/telegram/webhook" \
  TELEGRAM_WEBHOOK_SECRET="$(openssl rand -hex 32)"

# 3. Deploy the server
fly deploy
```

### 2. Verify Scale-to-Zero
1. Check running status:
   ```bash
   fly status
   ```
2. Wait 15 minutes with no traffic. Observe logs:
   ```bash
   fly logs
   # Logs will output:
   # "shutting down due to idle timeout" timeout=15m0s
   ```
3. Check machine status; it will transition to `stopped`:
   ```bash
   fly machine list
   ```
4. Send `/today` to the Telegram bot or make a `curl https://<your-app-name>.fly.dev/api/v1/schedule/today` request.
5. The machine boots automatically, handles the request, and resets the 15-minute idle countdown.
