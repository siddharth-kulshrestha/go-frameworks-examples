# TaskForge — Database Developer Experience Lab

## Purpose

TaskForge is a deliberately realistic Go backend used to compare the developer experience of **sqlc, Bun, and Ent**.

The application is a small Jira/Linear-style project-management system.

The goal is **NOT** to determine a universal winner. Implement the same requirements three times and record:

- SQL required
- handwritten Go required
- generated code
- ease of joins and aggregations
- transactions and locking
- many-to-many relationships
- self-referencing relationships
- PostgreSQL-specific features
- schema evolution
- debugging/generated SQL
- IDE experience
- framework-specific knowledge required

Do not simplify requirements for any library.

---

## Table of Contents

- [1. Repository / Implementation Boundary](#1-repository--implementation-boundary)
- [2. Domain](#2-domain)
- [3. Database Schema](#3-database-schema)
  - [3.1 organizations](#31-organizations)
  - [3.2 users](#32-users)
  - [3.3 teams](#33-teams)
  - [3.4 team_members](#34-team_members)
  - [3.5 projects](#35-projects)
  - [3.6 issues](#36-issues)
  - [3.7 labels](#37-labels)
  - [3.8 issue_labels](#38-issue_labels)
  - [3.9 comments](#39-comments)
  - [3.10 audit_events](#310-audit_events)
- [4. Required Indexes](#4-required-indexes)
- [5. Seed Data](#5-seed-data)
- [6. Repository Methods](#6-repository-methods)
- [7. Exact SQL Requirements: Core Entities](#7-exact-sql-requirements-core-entities)
  - [Organizations](#organizations)
  - [Users](#users)
  - [Teams](#teams)
  - [Projects](#projects)
- [8. Issues](#8-issues)
  - [CreateIssue](#createissue)
  - [GetIssueDetails](#getissuedetails)
  - [ListProjectIssues](#listprojectissues)
  - [CountProjectIssues](#countprojectissues)
  - [SearchIssues](#searchissues)
  - [GetIssueForUpdate](#getissueforupdate)
  - [UpdateIssueStatus](#updateissuestatus)
  - [AssignIssue](#assignissue)
  - [DeleteIssue](#deleteissue)
- [9. Labels](#9-labels)
  - [CreateLabel](#createlabel)
  - [AddLabelToIssue](#addlabeltoissue)
  - [RemoveLabelFromIssue](#removelabelfromissue)
  - [ListIssueLabels](#listissuelabels)
  - [AddLabelsToIssue (Batch)](#addlabelstoissue-batch)
- [10. Comments](#10-comments)
  - [CreateComment](#createcomment)
  - [ListIssueComments](#listissuecomments)
- [11. Audit Events](#11-audit-events)
  - [CreateAuditEvent](#createauditevent)
  - [ListIssueActivity](#listissueactivity)
- [12. Analytical Queries](#12-analytical-queries)
  - [ProjectDashboard](#projectdashboard)
  - [UserWorkload](#userworkload)
- [13. Recursive CTE](#13-recursive-cte)
- [14. Window Function](#14-window-function)
- [15. Transaction: ChangeIssueStatus](#15-transaction-changeissuestatus)
- [16. Transaction: CreateIssue](#16-transaction-createissue)
- [17. Tenant Isolation](#17-tenant-isolation)
- [18. Repository Contract Tests](#18-repository-contract-tests)
- [19. Concurrency Test](#19-concurrency-test)
- [20. Error Behavior](#20-error-behavior)
- [21. Developer-Experience Experiment](#21-developer-experience-experiment)
- [22. Required Comparison Matrix](#22-required-comparison-matrix)
- [23. Implementation Order](#23-implementation-order)
- [24. Definition of Done](#24-definition-of-done)
- [25. AI-Agent Implementation Rules](#25-ai-agent-implementation-rules)
- [26. Final Architecture](#26-final-architecture)

---

## 1. Repository / Implementation Boundary

Use three independent Go modules:

```text
taskforge/
├── sqlc/
├── bun/
├── ent/
├── docs/
├── PRODUCTIVITY.md
└── go.work
```

- Each implementation must use PostgreSQL and must independently implement the same domain behavior.
- Do not share persistence implementation code between the three modules.

---

## 2. Domain

TaskForge is a multi-tenant project management system.

```text
Organization
├── Users
├── Teams
│   ├── Team Members
│   └── Projects
│       └── Issues
│           ├── Creator
│           ├── Assignee
│           ├── Parent / Child Issues
│           ├── Labels
│           ├── Comments
│           └── Audit Events
└── Labels
```

There are exactly **10 database tables**.

---

## 3. Database Schema

### 3.1 organizations

```sql
CREATE TABLE organizations (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 3.2 users

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id),
    name TEXT NOT NULL,
    email TEXT NOT NULL,
    avatar_url TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organization_id, email)
);
```

### 3.3 teams

```sql
CREATE TABLE teams (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id),
    name TEXT NOT NULL,
    key TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organization_id, key)
);
```

### 3.4 team_members

```sql
CREATE TABLE team_members (
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role TEXT NOT NULL DEFAULT 'member',
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (team_id, user_id)
);
```

### 3.5 projects

```sql
CREATE TABLE projects (
    id UUID PRIMARY KEY,
    team_id UUID NOT NULL REFERENCES teams(id),
    name TEXT NOT NULL,
    key TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (team_id, key)
);
```

Valid project status values:
```text
active
archived
```

### 3.6 issues

```sql
CREATE TABLE issues (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id),
    creator_id UUID NOT NULL REFERENCES users(id),
    assignee_id UUID REFERENCES users(id),
    parent_issue_id UUID REFERENCES issues(id),
    title TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'todo',
    priority TEXT NOT NULL DEFAULT 'medium',
    estimate_points INTEGER,
    due_date DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CHECK (estimate_points IS NULL OR estimate_points >= 0),

    CHECK (status IN (
        'backlog', 'todo', 'in_progress', 'done', 'cancelled'
    )),

    CHECK (priority IN (
        'low', 'medium', 'high', 'urgent'
    ))
);
```

### 3.7 labels

```sql
CREATE TABLE labels (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id),
    name TEXT NOT NULL,
    color TEXT NOT NULL,
    UNIQUE (organization_id, name)
);
```

### 3.8 issue_labels

```sql
CREATE TABLE issue_labels (
    issue_id UUID NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    label_id UUID NOT NULL REFERENCES labels(id) ON DELETE CASCADE,
    PRIMARY KEY (issue_id, label_id)
);
```

### 3.9 comments

```sql
CREATE TABLE comments (
    id UUID PRIMARY KEY,
    issue_id UUID NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    author_id UUID NOT NULL REFERENCES users(id),
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 3.10 audit_events

```sql
CREATE TABLE audit_events (
    id BIGSERIAL PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id),
    actor_id UUID NOT NULL REFERENCES users(id),
    issue_id UUID REFERENCES issues(id) ON DELETE SET NULL,
    event_type TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

---

## 4. Required Indexes

Execute the following index creation statements:

```sql
CREATE INDEX idx_users_organization_id ON users(organization_id);
CREATE INDEX idx_team_members_user_id ON team_members(user_id);
CREATE INDEX idx_projects_team_id ON projects(team_id);
CREATE INDEX idx_issues_project_id ON issues(project_id);
CREATE INDEX idx_issues_assignee_id ON issues(assignee_id);
CREATE INDEX idx_issues_parent_issue_id ON issues(parent_issue_id);
CREATE INDEX idx_issues_status ON issues(status);
CREATE INDEX idx_issues_priority ON issues(priority);
CREATE INDEX idx_issues_due_date ON issues(due_date);
CREATE INDEX idx_issues_updated_at ON issues(updated_at);
CREATE INDEX idx_labels_organization_id ON labels(organization_id);
CREATE INDEX idx_comments_issue_created ON comments(issue_id, created_at);
CREATE INDEX idx_audit_events_org_created ON audit_events(organization_id, created_at);
CREATE INDEX idx_audit_events_issue_created ON audit_events(issue_id, created_at);
```

> [!IMPORTANT]
> Do not add arbitrary indexes without documenting the specific query optimization reason.

---

## 5. Seed Data

Create deterministic seed data containing:

```text
2 organizations
10 users
4 teams
6 projects
100+ issues
15 labels
200+ issue-label relationships
200+ comments
300+ audit events
```

Seed data must cover:

- Users in different organizations
- Users in multiple teams
- Projects with varying issue counts
- Assigned and unassigned issues
- Overdue issues
- Completed issues
- Cancelled issues
- Parent/child issue hierarchies
- Issues with multiple labels
- Issues without labels
- Issues with multiple comments
- Multiple audit-event types

> [!TIP]
> Use fixed, deterministic UUIDs so integration and contract tests can refer to known records reliably.

---

## 6. Repository Methods

> [!CAUTION]
> Do NOT create generic CRUD repositories. Expose domain- and business-oriented persistence operations.

### Organizations
```text
CreateOrganization
GetOrganization
```

### Users
```text
CreateUser
GetUser
ListUsersByOrganization
```

### Teams
```text
CreateTeam
AddTeamMember
RemoveTeamMember
ListTeamMembers
```

### Projects
```text
CreateProject
GetProject
ListProjectsByTeam
```

### Issues
```text
CreateIssue
GetIssueDetails
ListProjectIssues
CountProjectIssues
SearchIssues
GetIssueForUpdate
UpdateIssueStatus
AssignIssue
DeleteIssue
GetIssueTree
```

### Labels
```text
CreateLabel
AddLabelToIssue
RemoveLabelFromIssue
ListIssueLabels
AddLabelsToIssue
```

### Comments
```text
CreateComment
ListIssueComments
```

### Audit
```text
CreateAuditEvent
ListIssueActivity
```

### Additional Analytical Queries
```text
ProjectDashboard
UserWorkload
RecentIssueRank
```

---

## 7. Exact SQL Requirements: Core Entities

The following query semantics are fixed. SQL syntax MAY differ slightly between libraries where an abstraction demands it, but the resulting behavior and result sets must remain completely equivalent.

### Organizations

#### CreateOrganization
```sql
INSERT INTO organizations (id, name, slug)
VALUES ($1, $2, $3)
RETURNING *;
```

#### GetOrganization
```sql
SELECT *
FROM organizations
WHERE id = $1;
```

---

### Users

#### CreateUser
```sql
INSERT INTO users (
    id, organization_id, name, email, avatar_url
)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;
```

#### GetUser
```sql
SELECT *
FROM users
WHERE id = $1;
```

#### ListUsersByOrganization
```sql
SELECT *
FROM users
WHERE organization_id = $1
ORDER BY created_at DESC;
```

---

### Teams

#### CreateTeam
```sql
INSERT INTO teams (id, organization_id, name, key)
VALUES ($1, $2, $3, $4)
RETURNING *;
```

#### AddTeamMember
Must use upsert:
```sql
INSERT INTO team_members (team_id, user_id, role)
VALUES ($1, $2, $3)
ON CONFLICT (team_id, user_id)
DO UPDATE SET role = EXCLUDED.role;
```

#### RemoveTeamMember
```sql
DELETE FROM team_members
WHERE team_id = $1 AND user_id = $2;
```

#### ListTeamMembers
Required `JOIN`:
```sql
SELECT
    u.id,
    u.name,
    u.email,
    tm.role,
    tm.joined_at
FROM team_members tm
JOIN users u ON u.id = tm.user_id
WHERE tm.team_id = $1
ORDER BY u.name;
```

---

### Projects

#### CreateProject
```sql
INSERT INTO projects (
    id, team_id, name, key, description, status
)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;
```

#### GetProject
```sql
SELECT *
FROM projects
WHERE id = $1;
```

#### ListProjectsByTeam
```sql
SELECT *
FROM projects
WHERE team_id = $1
ORDER BY created_at DESC;
```

---

## 8. Issues

### CreateIssue

```sql
INSERT INTO issues (
    id,
    project_id,
    creator_id,
    assignee_id,
    parent_issue_id,
    title,
    description,
    status,
    priority,
    estimate_points,
    due_date
)
VALUES (
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $10, $11
)
RETURNING *;
```

### GetIssueDetails

Required `JOIN` query:

```sql
SELECT
    i.id,
    i.project_id,
    i.creator_id,
    i.assignee_id,
    i.parent_issue_id,
    i.title,
    i.description,
    i.status,
    i.priority,
    i.estimate_points,
    i.due_date,
    i.created_at,
    i.updated_at,

    p.name AS project_name,
    p.key AS project_key,

    t.name AS team_name,
    t.key AS team_key,

    creator.name AS creator_name,
    creator.email AS creator_email,

    assignee.name AS assignee_name,
    assignee.email AS assignee_email

FROM issues i
JOIN projects p ON p.id = i.project_id
JOIN teams t ON t.id = p.team_id
JOIN users creator ON creator.id = i.creator_id
LEFT JOIN users assignee ON assignee.id = i.assignee_id
WHERE i.id = $1;
```

> [!WARNING]
> Do not generate N+1 queries. All project, team, creator, and assignee data must be retrieved in a single statement.

### ListProjectIssues

```sql
SELECT
    i.id,
    i.title,
    i.status,
    i.priority,
    i.estimate_points,
    i.due_date,
    i.created_at,
    i.updated_at,
    creator.name AS creator_name,
    assignee.name AS assignee_name,
    COUNT(DISTINCT c.id) AS comment_count,
    COUNT(DISTINCT il.label_id) AS label_count
FROM issues i
JOIN users creator ON creator.id = i.creator_id
LEFT JOIN users assignee ON assignee.id = i.assignee_id
LEFT JOIN comments c ON c.issue_id = i.id
LEFT JOIN issue_labels il ON il.issue_id = i.id
WHERE i.project_id = $1
GROUP BY
    i.id,
    creator.name,
    assignee.name
ORDER BY i.updated_at DESC
LIMIT $2 OFFSET $3;
```

This intentionally exercises:
```text
JOIN
LEFT JOIN
COUNT
DISTINCT
GROUP BY
ORDER BY
LIMIT
OFFSET
```

### CountProjectIssues

```sql
SELECT COUNT(*)
FROM issues
WHERE project_id = $1;
```

### SearchIssues

Inputs:
```text
organization_id
search_term
limit
offset
```

Initial implementation uses `ILIKE`:

```sql
SELECT
    i.id,
    i.title,
    i.status,
    i.priority,
    i.project_id,
    p.name AS project_name,
    t.name AS team_name,
    assignee.name AS assignee_name,
    i.created_at,
    i.updated_at
FROM issues i
JOIN projects p ON p.id = i.project_id
JOIN teams t ON t.id = p.team_id
LEFT JOIN users assignee ON assignee.id = i.assignee_id
WHERE t.organization_id = $1
  AND (
      i.title ILIKE '%' || $2 || '%'
      OR i.description ILIKE '%' || $2 || '%'
  )
ORDER BY i.updated_at DESC
LIMIT $3 OFFSET $4;
```

### GetIssueForUpdate

Must use row locking:

```sql
SELECT *
FROM issues
WHERE id = $1
FOR UPDATE;
```

> [!IMPORTANT]
> This query **MUST** run inside an active database transaction.

### UpdateIssueStatus

```sql
UPDATE issues
SET
    status = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;
```

### AssignIssue

```sql
UPDATE issues
SET
    assignee_id = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;
```

### DeleteIssue

```sql
DELETE FROM issues
WHERE id = $1;
```

---

## 9. Labels

### CreateLabel

```sql
INSERT INTO labels (
    id, organization_id, name, color
)
VALUES ($1, $2, $3, $4)
RETURNING *;
```

### AddLabelToIssue

```sql
INSERT INTO issue_labels (issue_id, label_id)
VALUES ($1, $2)
ON CONFLICT (issue_id, label_id)
DO NOTHING;
```

### RemoveLabelFromIssue

```sql
DELETE FROM issue_labels
WHERE issue_id = $1
  AND label_id = $2;
```

### ListIssueLabels

```sql
SELECT
    l.id,
    l.name,
    l.color
FROM issue_labels il
JOIN labels l ON l.id = il.label_id
WHERE il.issue_id = $1
ORDER BY l.name;
```

### AddLabelsToIssue (Batch)

Input:
```text
issue_id
label_ids[]
```

Requirements:
- One transaction
- Bulk insert
- `ON CONFLICT DO NOTHING`
- Never execute one transaction or one query per label

> [!TIP]
> The implementation should use the most idiomatic batch operation available in the chosen library (e.g. unnesting slices, bulk insert builders, or batch exec).

---

## 10. Comments

### CreateComment

```sql
INSERT INTO comments (
    id, issue_id, author_id, body
)
VALUES ($1, $2, $3, $4)
RETURNING *;
```

### ListIssueComments

```sql
SELECT
    c.id,
    c.body,
    c.created_at,
    c.updated_at,
    u.id AS author_id,
    u.name AS author_name,
    u.email AS author_email
FROM comments c
JOIN users u ON u.id = c.author_id
WHERE c.issue_id = $1
ORDER BY c.created_at ASC;
```

---

## 11. Audit Events

### CreateAuditEvent

```sql
INSERT INTO audit_events (
    organization_id,
    actor_id,
    issue_id,
    event_type,
    metadata
)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;
```

`metadata` must be represented as structured JSON/JSONB.

### ListIssueActivity

```sql
SELECT
    ae.id,
    ae.issue_id,
    ae.event_type,
    ae.metadata,
    ae.created_at,
    u.id AS actor_id,
    u.name AS actor_name
FROM audit_events ae
JOIN users u ON u.id = ae.actor_id
WHERE ae.issue_id = $1
ORDER BY ae.created_at DESC
LIMIT $2 OFFSET $3;
```

---

## 12. Analytical Queries

These queries are deliberately included to test non-trivial analytical database workloads beyond standard CRUD.

### ProjectDashboard

Return fields:
```text
total_issues
backlog_count
todo_count
in_progress_count
done_count
cancelled_count
total_estimate_points
completed_estimate_points
```

Required PostgreSQL concepts:
```text
COUNT
SUM
FILTER
COALESCE
```

Target SQL:

```sql
SELECT
    COUNT(*) AS total_issues,

    COUNT(*) FILTER (WHERE status = 'backlog') AS backlog_count,
    COUNT(*) FILTER (WHERE status = 'todo') AS todo_count,
    COUNT(*) FILTER (WHERE status = 'in_progress') AS in_progress_count,
    COUNT(*) FILTER (WHERE status = 'done') AS done_count,
    COUNT(*) FILTER (WHERE status = 'cancelled') AS cancelled_count,

    COALESCE(SUM(estimate_points), 0) AS total_estimate_points,

    COALESCE(
        SUM(estimate_points) FILTER (WHERE status = 'done'),
        0
    ) AS completed_estimate_points

FROM issues
WHERE project_id = $1;
```

---

### UserWorkload

Return aggregate summary:
```text
total_assigned
open_count
completed_count
total_estimate_points
completed_estimate_points
overdue_count
```

Also return a per-project breakdown:
```text
project_id
project_name
open_count
completed_count
total_points
completed_points
```

Requirements:
- Assigned to the requested user (`assignee_id = $1`)
- Optional date range filtering
- Overdue criteria: `due_date < current_date` and `status NOT IN ('done', 'cancelled')`
- Use `JOIN`
- Use `COUNT` and `SUM`
- Use `GROUP BY`
- Use `FILTER` or `CASE`

Target SQL:

```sql
-- Aggregate Summary
SELECT
    COUNT(*) AS total_assigned,
    COUNT(*) FILTER (WHERE status NOT IN ('done', 'cancelled')) AS open_count,
    COUNT(*) FILTER (WHERE status = 'done') AS completed_count,
    COALESCE(SUM(estimate_points), 0) AS total_estimate_points,
    COALESCE(SUM(estimate_points) FILTER (WHERE status = 'done'), 0) AS completed_estimate_points,
    COUNT(*) FILTER (
        WHERE due_date < CURRENT_DATE
          AND status NOT IN ('done', 'cancelled')
    ) AS overdue_count
FROM issues
WHERE assignee_id = $1
  AND ($2::TIMESTAMPTZ IS NULL OR created_at >= $2)
  AND ($3::TIMESTAMPTZ IS NULL OR created_at <= $3);

-- Per-Project Breakdown
SELECT
    p.id AS project_id,
    p.name AS project_name,
    COUNT(*) FILTER (WHERE i.status NOT IN ('done', 'cancelled')) AS open_count,
    COUNT(*) FILTER (WHERE i.status = 'done') AS completed_count,
    COALESCE(SUM(i.estimate_points), 0) AS total_points,
    COALESCE(SUM(i.estimate_points) FILTER (WHERE i.status = 'done'), 0) AS completed_points
FROM issues i
JOIN projects p ON p.id = i.project_id
WHERE i.assignee_id = $1
  AND ($2::TIMESTAMPTZ IS NULL OR i.created_at >= $2)
  AND ($3::TIMESTAMPTZ IS NULL OR i.created_at <= $3)
GROUP BY p.id, p.name
ORDER BY p.name;
```

---

## 13. Recursive CTE

Implement:
```text
GetIssueTree(project_id)
```

Issues have self-referential relationships:
```text
parent_issue_id → issues.id
```

Use a PostgreSQL recursive Common Table Expression (CTE).

Target SQL:

```sql
WITH RECURSIVE issue_tree AS (

    SELECT
        id,
        parent_issue_id,
        title,
        status,
        0 AS depth
    FROM issues
    WHERE project_id = $1
      AND parent_issue_id IS NULL

    UNION ALL

    SELECT
        child.id,
        child.parent_issue_id,
        child.title,
        child.status,
        parent.depth + 1
    FROM issues child
    JOIN issue_tree parent
        ON child.parent_issue_id = parent.id
)

SELECT *
FROM issue_tree
ORDER BY depth, title;
```

> [!CAUTION]
> Do **NOT** replace this with application-side recursion. The purpose is specifically to compare how each database abstraction handles recursive PostgreSQL SQL.

---

## 14. Window Function

Implement:
```text
RecentIssueRank
```

Return the three most recently updated issues per project.

Required SQL concept:
```sql
ROW_NUMBER() OVER (
    PARTITION BY project_id
    ORDER BY updated_at DESC
)
```

Target SQL:

```sql
WITH ranked AS (
    SELECT
        i.*,
        ROW_NUMBER() OVER (
            PARTITION BY project_id
            ORDER BY updated_at DESC
        ) AS row_num
    FROM issues i
)
SELECT *
FROM ranked
WHERE row_num <= 3;
```

---

## 15. Transaction: ChangeIssueStatus

This is a business operation, not just a standalone UPDATE statement.

Input:
```text
issue_id
actor_id
new_status
```

Required sequence:
```text
BEGIN

SELECT issue FOR UPDATE

Validate issue exists

Validate actor

Validate status transition

UPDATE issue

INSERT audit event

COMMIT
```

Audit event specification:
```text
event_type = issue.status_changed
metadata = {
    "from": old_status,
    "to": new_status
}
```

If anything fails:
```text
ROLLBACK
```

No partial state may remain in the database.

---

## 16. Transaction: CreateIssue

Creating an issue with labels must be atomic.

Required sequence:
```text
BEGIN

Validate project

Validate creator

Validate assignee

Validate parent issue

INSERT issue

INSERT issue_labels

INSERT audit event

COMMIT
```

If label insertion or audit insertion fails, the issue must not exist.

---

## 17. Tenant Isolation

Tenant isolation is mandatory.

Organization A must never retrieve Organization B's data across:
```text
users
teams
projects
issues
labels
audit events
```

- Test cases must include cross-tenant IDs.
- Search queries must enforce organization boundaries.
- Where appropriate, establish ownership through joins rather than trusting a caller-supplied organization ID.

---

## 18. Repository Contract Tests

The same logical test suite must run against all three implementations (`sqlc`, `Bun`, `Ent`).

Required tests include:

```text
CreateOrganization
GetOrganization

CreateUser
GetUser
ListUsersByOrganization

CreateTeam
AddTeamMember
RemoveTeamMember
ListTeamMembers

CreateProject
GetProject
ListProjectsByTeam

CreateIssue
GetIssueDetails
ListProjectIssues
CountProjectIssues
SearchIssues
AssignIssue
ChangeIssueStatus
DeleteIssue

CreateLabel
AddLabelToIssue
RemoveLabelFromIssue
ListIssueLabels
AddLabelsToIssue

CreateComment
ListIssueComments

CreateAuditEvent
ListIssueActivity

ProjectDashboard
UserWorkload
IssueTree
RecentIssueRank

TenantIsolation
TransactionRollback
ConcurrentStatusUpdate
```

---

## 19. Concurrency Test

Run two concurrent status changes against the same issue.

The implementation must demonstrate that the `FOR UPDATE` transaction prevents inconsistent state.

Test scenario:
```text
Transaction A:
    SELECT issue FOR UPDATE
    UPDATE status
    INSERT audit
    COMMIT

Transaction B:
    attempts update on same issue concurrently
```

Verify that the final issue state and audit log history are strictly consistent without race conditions.

---

## 20. Error Behavior

The repository/service boundary must distinguish the following error classes:

```text
not found
constraint violation
database failure
transaction failure
```

Exact error types may differ between `sqlc`, `Bun`, and `Ent`, but the classified behavior must be equivalent.

---

## 21. Developer-Experience Experiment

For every implementation, maintain a `PRODUCTIVITY.md`.

Record:
```text
setup time
dependency count
generated files
handwritten files
handwritten LOC
SQL LOC
framework-specific code
time per use case
time per migration
time debugging a failed query
```

For every difficult operation, record:
```text
Requirement
Implementation
Resulting SQL
Generated code
What was easy?
What was difficult?
What documentation/searching was required?
How good were compiler errors?
How good were runtime errors?
How easy was IDE navigation?
```

> [!NOTE]
> Do NOT assign an overall numeric winner. The objective is evidence-based developer experience comparison.

---

## 22. Required Comparison Matrix

Every implementation must support:

| Capability | sqlc | Bun | Ent |
|:---|:---:|:---:|:---:|
| Basic CRUD | ✓ | ✓ | ✓ |
| JOIN | ✓ | ✓ | ✓ |
| LEFT JOIN | ✓ | ✓ | ✓ |
| N:M Relationships | ✓ | ✓ | ✓ |
| Aggregation | ✓ | ✓ | ✓ |
| Dynamic Filters | ✓ | ✓ | ✓ |
| Pagination | ✓ | ✓ | ✓ |
| Transactions | ✓ | ✓ | ✓ |
| Row Locking (`FOR UPDATE`) | ✓ | ✓ | ✓ |
| Upsert (`ON CONFLICT`) | ✓ | ✓ | ✓ |
| Batch Insert | ✓ | ✓ | ✓ |
| JSONB Support | ✓ | ✓ | ✓ |
| Recursive CTE | ✓ | ✓ | ✓ |
| Window Functions | ✓ | ✓ | ✓ |
| Concurrent Update Testing | ✓ | ✓ | ✓ |

*The checkmarks describe required functionality to be tested, NOT an assessment of ease.*

---

## 23. Implementation Order

Do not implement everything at once. Follow this phased progression:

### Phase 1 — Foundation
```text
Organization
User
Team
Project
```

### Phase 2 — Issues
```text
CreateIssue
GetIssueDetails
DeleteIssue
AssignIssue
```

### Phase 3 — Relationships and Aggregation
```text
ListProjectIssues
CountProjectIssues
ListIssueLabels
ListIssueComments
```

### Phase 4 — Transactions
```text
GetIssueForUpdate
UpdateIssueStatus
CreateAuditEvent
ListIssueActivity
```

### Phase 5 — Search
```text
SearchIssues
```

### Phase 6 — PostgreSQL-Specific Queries
```text
ProjectDashboard
UserWorkload
IssueTree
RecentIssueRank
```

### Phase 7 — Batch
```text
AddLabelsToIssue
```

### Phase 8 — Contract & Integration Tests
```text
Tenant isolation
Rollback
Concurrency
All repository methods
```

> [!IMPORTANT]
> Sequence rule: Only after **sqlc** is complete should **Bun** implement the same requirements. Only after Bun is complete should **Ent** implement the same requirements.

---

## 24. Definition of Done

An implementation is complete only when:

```text
✓ Migrations run clean from an empty PostgreSQL database
✓ Seed data is deterministic
✓ Generated code compiles and works
✓ All repository methods exist
✓ All required SQL behavior exists
✓ Transactions work
✓ Rollback works
✓ Tenant isolation works
✓ Recursive CTE works
✓ Window function works
✓ JSONB works
✓ Upsert works
✓ Batch operations work
✓ Integration tests pass
✓ Concurrency test passes
```

---

## 25. AI-Agent Implementation Rules

The coding agent implementing this specification must **NOT**:

- Redesign the schema
- Remove or rename tables
- Simplify relationships
- Remove PostgreSQL-specific queries
- Replace recursive SQL with application-side recursion
- Introduce another ORM
- Implement Fiber or Go kit yet (focus on database DX first)
- Implement Kubernetes yet
- Change transaction boundaries
- Replace the required business-oriented repository methods with generic CRUD
- Optimize away a deliberately included query because another abstraction makes it easier

The purpose of the project is to expose the actual developer experience of the selected database library. Implement the requirements faithfully.

---

## 26. Final Architecture

Post-database experiment system architecture:

```text
                    TaskForge API
                         │
                 ┌───────┴────────┐
                 │                │
               Fiber           Go kit
                 │                │
                 └───────┬────────┘
                         │
                      Service
                         │
                 Repository Interface
                         │
          ┌──────────────┼──────────────┐
          │              │              │
         sqlc           Bun            Ent
          │              │              │
          └──────────────┼──────────────┘
                         │
                    PostgreSQL
```

The central learning objective is:

> **What does each database abstraction make easier, what does it make harder, and how much PostgreSQL knowledge remains necessary when building a non-trivial Go service?**
