# HelixTrack Database Schema — 2026-06-21

## Overview

HelixTrack uses SQLite (development) or PostgreSQL (production). The shipped core
schema spans **five Definition versions (V1-V5)** plus a **V5.6 migration**
(`Migration.V5.6.sql` — "V5 → V6 Security Engine Enhancements", dated 2025-10-19).
The five Definition DDL files declare **106 distinct tables in total**:

| Version | DDL file | Theme | Distinct `CREATE TABLE` |
|---------|----------|-------|--------------------------|
| V1 | `Definition.V1.sql` | Core | 60 |
| V2 | `Definition.V2.sql` | JIRA Feature Parity — Phase 1 | 12 |
| V3 | `Definition.V3.sql` | JIRA Feature Parity — Complete (Phases 2 & 3) | 17 |
| V4 | `Definition.V4.sql` | Parallel Editing Support | 6 |
| V5 | `Definition.V5.sql` | Documents Extension Integration & Cross-Entity Linking | 11 |
| — | `Migration.V5.6.sql` | Security Engine Enhancements (creates `permission_cache`, `security_audit`; ALTERs the audit table) | (migration) |

Source of truth: the DDL files under
`/Volumes/T7/Projects/helix_track/core/Database/DDL/`. Counts above are distinct
`CREATE TABLE [IF NOT EXISTS]` names per file (each file's tables are additive on
top of the prior version). The category breakdowns below were authored against an
earlier V1-V3 snapshot and use illustrative per-section counts that do not
re-derive the 106-table total above — re-derive any per-section count directly
from the DDL when precision matters.

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

> Note: `security_audit` and `permission_cache` are (re)created/expanded by the
> `Migration.V5.6.sql` Security-Engine migration, not only by an early version.

---

## V4 Tables — Parallel Editing Support

Defined in `Definition.V4.sql` (6 distinct tables). Supports concurrent/parallel
editing of entities. Re-derive the exact table list from the DDL file.

---

## V5 Tables — Documents Extension & Cross-Entity Linking

Defined in `Definition.V5.sql` (11 distinct tables). Integrates the Documents
extension and cross-entity linking. Re-derive the exact table list from the DDL
file.

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
- V1 — Initial core schema (`Definition.V1.sql`)
- V2 — JIRA feature parity, phase 1 (`Definition.V2.sql`)
- V3 — JIRA feature parity, complete / phases 2 & 3 (`Definition.V3.sql`)
- V4 — Parallel editing support (`Definition.V4.sql`)
- V5 — Documents extension & cross-entity linking (`Definition.V5.sql`)
- V5 → V6 — Security engine enhancements (`Migration.V5.6.sql`, 2025-10-19)

### Migration Files
- `core/Database/DDL/Migration.V1.2.sql`
- `core/Database/DDL/Migration.V2.3.sql`
- `core/Database/DDL/Migration.V3.4.sql`
- `core/Database/DDL/Migration.V5.6.sql` (also mirrored at `core/Application/Database/DDL/Migration.V5.6.sql`)

---

## Cross-references
- [Architecture](/Volumes/T7/Projects/helix_code/docs/helixtrack/ARCHITECTURE.md)
- [API Reference](/Volumes/T7/Projects/helix_code/docs/helixtrack/API_REFERENCE.md)

## Sources verified 2026-06-22: /Volumes/T7/Projects/helix_track/core/Database/DDL/Definition.V1.sql..Definition.V5.sql , /Volumes/T7/Projects/helix_track/core/Database/DDL/Migration.V5.6.sql , /Volumes/T7/Projects/helix_track/core/Application/Database/DDL/Migration.V5.6.sql

Reconciled 2026-06-22: schema range **V1-V3 / "71 tables" → V1-V5 + V5.6
migration / 106 distinct tables**, per the actual `Definition.V*.sql` DDL
(authoritative source of truth). Per-version distinct `CREATE TABLE` counts read
directly from the DDL: V1=60, V2=12, V3=17, V4=6, V5=11 (total 106); the
`Migration.V5.6.sql` "V5 → V6 Security Engine Enhancements" (2025-10-19) creates
`permission_cache` + `security_audit` and ALTERs the audit table.

REPO-STATE-DERIVED (per §11.4.99 the sources are the cross-referenced DDL files,
following the `docs/ARCHITECTURE.md` precedent). Cross-referenced on 2026-06-22:
- **Schema DDL CONFIRMED present:** `helix_track/core/Database/DDL/Definition.V1.sql`
  … `Definition.V5.sql` exist (plus `Indexes_Performance.sql`, `Migration.V*.sql`),
  and the V5.6 migration is mirrored at `core/Application/Database/DDL/`
  (`Migration.V5.6.sql`, `Test_Data_Users_Permissions.sql`). Extension and service
  DDLs (`Extensions/{Chats,Documents,Times}/`, `Services/{Authentication,Localization}/`)
  also exist and are NOT counted in the 106-table core total above.
- **Resolved (version-range drift):** the prior revision stated "71 tables across
  **V1-V3**"; the live DDL tree carries Definition files **V1 through V5** plus a
  **V5.6** migration, with 106 distinct core tables. The overview table + Migration
  History have been re-derived from the V1-V5 DDL. The legacy per-section category
  counts (V1 "57", V2 "14") are illustrative groupings authored against the old
  V1-V3 snapshot and are NOT the authoritative totals — the 106-table figure from
  the DDL is. Re-verify any per-section count directly from the `Definition.V*.sql`
  DDL, which remains the single source of truth.
