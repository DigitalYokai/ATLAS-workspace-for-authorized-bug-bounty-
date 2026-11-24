# Phase 4 API

All endpoints are rooted at `/api/v1` and require an authenticated local session, except `POST /auth/login` and `/healthz`. State-changing requests require `X-CSRF-Token`, returned by login or `GET /auth/me`.

- `GET /targets/{targetId}/dashboard` returns target metrics, entity type counts, recent runs and activity.
- `GET /targets/{targetId}/graph?rootEntityId={id}&depth=1|2&limit=1..100` returns a bounded graph neighborhood.
- `GET,POST /targets/{targetId}/schedules` and `PATCH /targets/{targetId}/schedules/{scheduleId}` manage passive schedules.
- `GET /activity`, `GET /targets/{targetId}/activity`, `GET /notifications`, and `PATCH /notifications/{id}` expose investigation activity and notifications.
- `GET /targets/{targetId}/exports/json|csv|markdown|graph` downloads target intelligence in the selected representation.
- `GET,PATCH /settings` reads or updates scheduler pause, default concurrency, evidence retention days, and evidence size limit.

Graph requests never return more than 100 relationships. Schedules accept only `dns` and `certificate_transparency` collectors and a whole-hour interval of at least one hour.
