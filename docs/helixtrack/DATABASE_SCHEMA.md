# HelixTrack Database Schema — 2026-06-21

## Overview

HelixTrack uses SQLite (development) or PostgreSQL (production) with 71 tables across V1-V3.

---

## V1 Core Tables (57)

### Business Entities

| Table | Purpose | Status |
|-------|---------|--------|
| project | Projects/workspaces | ✅ Implemented |
| ticket | Issues/tasks | ✅ Implemented |
| user | Users | ✅ Implemented |
| comment | Comments | ✅ Implemented |
| team | Teams | ✅ Implemented |
| attachment | File attachments | ⚠️ Partial |
| label | Labels/tags | ❌ Missing |
| priority | Priorities | ❌ Missing |
| resolution | Resolutions | ❌ Missing |
| component | Components | ❌ Missing |
| cycle | Sprints/iterations | ❌ Missing |

### Workflow Entities

| Table | Purpose | Status |
|-------|---------|--------|
| workflow | Workflow definitions | ❌ Missing |
| workflow_step | Workflow steps | ❌ Missing |
| ticket_type | Ticket types | ❌ Missing |
| ticket_status | Ticket statuses | ❌ Missing |

### Board Entities

| Table | Purpose | Status |
|-------|---------|--------|
| board | Kanban/Scrum boards | ❌ Missing |
| board_column | Board columns | ❌ Missing |
| board_ticket | Board-ticket mapping | ❌ Missing |

### User Entities

| Table | Purpose | Status |
|-------|---------|--------|
| user_role | User roles | ❌ Missing |
| permission | Permissions | ❌ Missing |
| user_preference | User preferences | ❌ Missing |

### System Entities

| Table | Purpose | Status |
|-------|---------|--------|
| system_info | System metadata | ❌ Missing |
| audit_log | Audit trail | ❌ Missing |
| notification | Notifications | ❌ Missing |

---

## V2 Extension Tables (14)

### Enhanced Features

| Table | Purpose | Status |
|-------|---------|--------|
| ticket_relationship | Ticket relationships | ⚠️ Partial |
| ticket_link | Ticket links | ⚠️ Partial |
| ticket_watchers | Ticket watchers | ⚠️ Partial |
| ticket_time_entry | Time tracking | ⚠️ Partial |

### Workflow Extensions

| Table | Purpose | Status |
|-------|---------|--------|
| workflow_transition | Workflow transitions | ❌ Missing |
| workflow_condition | Workflow conditions | ❌ Missing |
| workflow_action | Workflow actions | ❌ Missing |

---

## V3 Advanced Tables

### Security

| Table | Purpose | Status |
|-------|---------|--------|
| security_audit | Security audit log | ✅ Implemented |
| permission_cache | Permission cache | ✅ Implemented |
| account | Account management | ✅ Implemented |

---

## Table Relationships

### Core Relationships
```
project (1) ──── (N) ticket
project (1) ──── (N) board
project (1) ──── (N) cycle
project (1) ──── (N) component
ticket (1) ──── (N) comment
ticket (1) ──── (N) attachment
ticket (N) ──── (N) label
user (1) ──── (N) ticket (assignee)
user (1) ──── (N) comment
team (N) ──── (N) user
```

---

## Indexes

### Performance Indexes
- `idx_ticket_project_id` — Ticket lookup by project
- `idx_ticket_status` — Ticket lookup by status
- `idx_ticket_assignee` — Ticket lookup by assignee
- `idx_comment_ticket_id` — Comment lookup by ticket
- `idx_audit_created` — Audit log by date
- `idx_audit_user_id` — Audit log by user

---

## Migrations

### Migration History
- V1.0 — Initial schema
- V2.0 — Extended features
- V3.0 — Advanced features
- V5.6 — Security engine enhancement

### Migration Files
- `core/Application/Database/DDL/Migration.V5.6.sql`

---

## Cross-references
- [Architecture](/Volumes/T7/Projects/helix_code/docs/helixtrack/ARCHITECTURE.md)
- [API Reference](/Volumes/T7/Projects/helix_code/docs/helixtrack/API_REFERENCE.md)

## Sources verified 2026-06-22: /Volumes/T7/Projects/helix_track/core/Database/DDL/Definition.V1..V5.sql , /Volumes/T7/Projects/helix_track/core/Application/Database/DDL/ , https://github.com/Helix-Track/Everything

REPO-STATE-DERIVED (per §11.4.99 the sources are the cross-referenced DDL files,
following the `docs/ARCHITECTURE.md` precedent). Cross-referenced on 2026-06-22:
- **Schema DDL CONFIRMED present:** `helix_track/core/Database/DDL/Definition.V1.sql`
  … `Definition.V5.sql` exist (plus `Indexes_Performance.sql`, `Migration.V*.sql`),
  and the migration set also lives at `core/Application/Database/DDL/`
  (`Migration.V5.6.sql`, `Test_Data_Users_Permissions.sql`) — the doc's
  `core/Application/Database/DDL/...` migration path is real. SQLite (dev) source
  `Definition.sqlite` is present.
- **Negative finding (version-range drift):** this doc states "71 tables across
  **V1-V3**", but the live DDL tree carries Definition files **V1 through V5** (and
  a V5.6 migration). The table count and the "V1-V3" range should be re-derived
  from the V1-V5 DDL on the next revision — the doc's range understates the
  shipped schema versions. Re-verify table counts directly from the `Definition.V*.sql`
  DDL, which is the source of truth.
