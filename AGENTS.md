# Agent Guidelines & Workflow Rules

## Source of Truth & Context Discovery (`docs/`)

The `docs/` directory is the single source of truth for all architectural decisions, API specifications, scraper selectors, authentication mechanisms, and research findings in this project.

- **Mandatory Reference**: Before implementing new features, modifying endpoints, scraping schedules, or debugging integrations, AI agents **must consult the relevant files in `docs/`** (start with [`docs/README.md`](docs/README.md)).
- **Decision History & Findings**: All details regarding external KPI systems (`api.campus.kpi.ua`, `my.kpi.ua`), cookie formats, browser extension pairing, and schedule merging logic are thoroughly documented in `docs/`. Never make unverified assumptions when the answers are already recorded in `docs/`.

---

## Documentation Maintenance (Mandatory)

All AI agents working on this codebase must adhere to the following documentation requirements:

1. **Keep Docs in Sync with Code**:
   - Whenever code is added, modified, refactored, or removed, the corresponding documentation files in the `docs/` directory **must be updated immediately** in the same task.
   - Any modifications to API endpoints, request/response models, database schemas, scraper selectors, authentication flows, or bot commands must be accurately reflected in their respective documentation files.

2. **Document Architectural Decisions**:
   - When new design choices, architectural changes, or third-party integrations are decided upon, document the rationale, tradeoffs, and specifications in `docs/architecture/` or relevant subfolders before or alongside implementation.

3. **Preserve Modular Organization**:
   - Maintain the existing modular structure of `docs/` (e.g. `docs/architecture/`, `docs/schedules/`, `docs/api/`, `docs/bot/`, `docs/extension/`).
   - Do not merge distinct topics into monolithic files. If introducing a new system component, create a dedicated markdown file in an appropriate logical subfolder and update `docs/README.md`.

4. **Zero Tolerance for Stale Docs**:
   - Outdated documentation is considered a defect. Always verify that code changes do not contradict existing documentation.
