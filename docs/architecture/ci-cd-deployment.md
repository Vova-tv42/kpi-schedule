# CI/CD Deployment to Fly.io via GitHub Actions

This document outlines the Continuous Integration and Continuous Deployment (CI/CD) pipeline for the Go server component of the **KPI Personalized Schedule Platform**, targeting **Fly.io Fly Machines**.

---

## 1. Overview & Strategy

Fly.io operates on an API- and CLI-centric architecture rather than offering native Git repository triggers from its web dashboard. Automated deployments on code pushes are therefore orchestrated through **GitHub Actions**.

The workflow is defined at [`.github/workflows/fly-deploy.yml`](../../.github/workflows/fly-deploy.yml).

```
   [ Push to main ]
          │
          ▼
┌──────────────────┐
│  Path Filter     │ (apps/server/** or fly-deploy.yml)
└─────────┬────────┘
          │
          ▼
┌──────────────────┐
│   Job: test      │ ──► [ go test -v ./... ]
└─────────┬────────┘
          │ (Success)
          ▼
┌──────────────────┐
│   Job: deploy    │ ──► [ superfly/flyctl-actions ]
└─────────┬────────┘
          │
          ▼
┌──────────────────┐
│  Fly.io Remote   │ ──► Builds Docker image & deploys
│     Builder      │     to Fly Machines (scale-to-zero)
└──────────────────┘
```

---

## 2. Trigger Conditions & Monorepo Optimization

Because this repository is a monorepo containing both the backend Go server (`apps/server`) and the Chrome extension (`apps/extension`), deployments are scoped using GitHub Actions path filters:

- **Branches**: `main`
- **Paths**:
  - `apps/server/**` (server source code, `go.mod`, `go.sum`, `fly.toml`, `Dockerfile`)
  - `.github/workflows/fly-deploy.yml` (pipeline configuration changes)
- **Manual Trigger**: `workflow_dispatch` is enabled to allow manual redeployments from the GitHub Actions tab without requiring an empty commit.

Changes solely touching `apps/extension/**`, root documentation (`docs/**`), or root assets will **not** trigger redundant Fly.io deployments.

---

## 3. Pipeline Stages

### Stage 1: `test`
- **Runner**: `ubuntu-latest`
- **Go Setup**: Uses `actions/setup-go@v5` reading Go version directly from `apps/server/go.mod` with dependency caching enabled via `apps/server/go.sum`.
- **Command**: `go test -v ./...`
- If any unit tests fail, the deployment stage is skipped immediately.

### Stage 2: `deploy`
- **Requirement**: Depends on successful completion of the `test` stage (`needs: test`).
- **Flyctl Installation**: Uses `superfly/flyctl-actions/setup-flyctl@master`.
- **Execution**: Runs inside `apps/server/`:
  ```bash
  flyctl deploy --remote-only
  ```
- **Remote Builders**: The `--remote-only` flag delegates Docker container building to Fly's hosted remote builders. This avoids Docker daemon dependencies on the GitHub runner, leverages remote layer caching, and ensures native target architecture compatibility.
- **Concurrency**: Grouped under `fly-deploy` with `cancel-in-progress: false` to ensure deployments queue sequentially and prevent machine race conditions.

---

## 4. Setup Guide: Configuring `FLY_API_TOKEN`

To authorize GitHub Actions to deploy to your Fly.io organization/app:

### Step 1: Generate a Fly Deploy Token
From your terminal within the server directory:
```bash
cd apps/server
flyctl tokens create deploy
```
*Alternatively, specify the app explicitly:*
```bash
flyctl tokens create deploy -a kpi-schedule
```
Or generate one via the **Fly.io Web Dashboard** under your app's or organization's access tokens section.

### Step 2: Store in GitHub Repository Secrets
1. Navigate to your GitHub repository: `https://github.com/<owner>/kpi-schedule/settings/secrets/actions`.
2. Click **New repository secret**.
3. Name: `FLY_API_TOKEN`
4. Secret: Paste the deploy token generated in Step 1.
5. Click **Add secret**.

---

## 5. Rollback & Manual Deployments

### Manual Redeploy via GitHub Actions
1. Open the repository on GitHub.
2. Click the **Actions** tab.
3. Select **Deploy to Fly.io** in the left sidebar.
4. Click **Run workflow** -> Select branch `main` -> Click **Run workflow**.

### Direct CLI Deployment (Emergency Fallback)
If GitHub Actions is unavailable:
```bash
cd apps/server
flyctl deploy --remote-only
```
