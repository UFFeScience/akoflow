---
title: API request history
---

Open **Audit → API requests** to inspect completed HTTP requests to the daemon,
including authentication failures, unknown routes, installation controls and MCP
transport calls. Records show the completion sequence (newest first), start time,
HTTP method, path, status, elapsed milliseconds, peer IP and caller classification.
Long-lived streams appear when they close. History starts after this version is
installed; earlier calls cannot be reconstructed.

The current shared API token proves access to the installation, not a particular
person. The caller column distinguishes authenticated API-token requests from
anonymous or unauthenticated requests. The optional client label is reported by
the caller and must not be treated as verified identity. Peer IP is taken from the
connection, not forwarded headers; a reverse proxy may therefore appear as caller.
Request bodies, query strings, authorization headers and cookies are not stored.

The table fetches 50 records at a time, without automatic polling. **Refresh**
returns to the newest records. Cursor pagination prevents concurrent inserts from
shifting older pages. The API accepts `GET /akoflow-api/request-logs/?limit=50`
and a `before` cursor from the response's `nextCursor`; limits range from 1 to 100.
Each response contains `items` and `nextCursor` (empty on the final page).

Configure **Settings → Data → API request log retention** between 1 and 3650 days
(default: 30). `GET` and `PUT /akoflow-api/request-log-settings/` read or update
`{"retentionDays":30}`. Reducing retention permanently removes expired records.
Cleanup runs at startup and every minute, deleting up to 1000 expired rows per
pass to bound write transactions. Large backlogs take multiple passes. SQLite may
reuse the released space without shrinking the database file immediately.

History and settings live in `request-audit.db` beside the configured instance
database, independent of instance archives and their read-only state. Back up
this sidecar separately when request history is required. Existing operational
events remain available under **Audit → Operational events**. Audit persistence
errors are reported in daemon logs; request handling continues if a write fails.
