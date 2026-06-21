# HelixTrack API Reference — 2026-06-21

## Overview

HelixTrack Core Backend API reference documentation.

**Base URL:** `http://localhost:8080`
**Authentication:** JWT Bearer Token

---

## Authentication

### Login
```http
POST /api/v1/auth/login
Content-Type: application/json

{
  "username": "string",
  "password": "string"
}

Response:
{
  "token": "jwt_token",
  "user": {
    "id": "string",
    "username": "string",
    "email": "string"
  }
}
```

### Register
```http
POST /api/v1/auth/register
Content-Type: application/json

{
  "username": "string",
  "email": "string",
  "password": "string",
  "fullName": "string"
}
```

---

## Projects

### List Projects
```http
GET /api/v1/projects
Authorization: Bearer <token>
```

### Get Project
```http
GET /api/v1/projects/:id
Authorization: Bearer <token>
```

### Create Project
```http
POST /api/v1/projects
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "string",
  "key": "string",
  "description": "string",
  "lead": "string"
}
```

### Update Project
```http
PUT /api/v1/projects/:id
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "string",
  "description": "string"
}
```

### Delete Project
```http
DELETE /api/v1/projects/:id
Authorization: Bearer <token>
```

---

## Tickets

### List Tickets
```http
GET /api/v1/tickets?project_id=<id>&status=<status>&priority=<priority>
Authorization: Bearer <token>
```

### Get Ticket
```http
GET /api/v1/tickets/:id
Authorization: Bearer <token>
```

### Create Ticket
```http
POST /api/v1/tickets
Authorization: Bearer <token>
Content-Type: application/json

{
  "title": "string",
  "description": "string",
  "ticketTypeId": "string",
  "ticketStatusId": "string",
  "projectId": "string",
  "priority": "string",
  "assignee": "string"
}
```

### Update Ticket
```http
PUT /api/v1/tickets/:id
Authorization: Bearer <token>
Content-Type: application/json

{
  "title": "string",
  "description": "string",
  "status": "string",
  "assignee": "string"
}
```

### Delete Ticket
```http
DELETE /api/v1/tickets/:id
Authorization: Bearer <token>
```

---

## Comments

### List Comments
```http
GET /api/v1/comments?ticket_id=<id>
Authorization: Bearer <token>
```

### Create Comment
```http
POST /api/v1/comments
Authorization: Bearer <token>
Content-Type: application/json

{
  "ticketId": "string",
  "content": "string"
}
```

---

## Users

### List Users
```http
GET /api/v1/users
Authorization: Bearer <token>
```

### Get User
```http
GET /api/v1/users/:id
Authorization: Bearer <token>
```

---

## Teams

### List Teams
```http
GET /api/v1/teams?project_id=<id>
Authorization: Bearer <token>
```

### Create Team
```http
POST /api/v1/teams
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "string",
  "description": "string",
  "projectId": "string"
}
```

---

## WebSocket

### Connect
```ws
ws://localhost:8080/ws?token=<jwt_token>
```

### Events
- `ticket.created` — New ticket created
- `ticket.updated` — Ticket updated
- `comment.created` — New comment
- `user.online` — User came online
- `user.offline` — User went offline

---

## Error Codes

| Code | Description |
|------|-------------|
| 200 | Success |
| 201 | Created |
| 400 | Bad Request |
| 401 | Unauthorized |
| 403 | Forbidden |
| 404 | Not Found |
| 500 | Internal Server Error |

---

## Cross-references
- [Architecture](/Volumes/T7/Projects/helix_code/docs/helixtrack/ARCHITECTURE.md)
- [Implementation Plan](/Volumes/T7/Projects/helix_code/docs/helixtrack/IMPLEMENTATION_PLAN.md)
