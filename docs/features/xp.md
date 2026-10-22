# Points and categorized XP

XP is display terminology for the existing points currency, not an additional
balance. `users.xp_mode` defaults to false. Users can change their own preference
with `PUT /api/v1/xp/preference`, body `{"xpMode":true}`.

## Categories and task configuration

Active circle members can list `GET /api/v1/xp/categories`. Admins and managers
create (`POST`) and rename/archive/restore (`PUT /:id`) categories with
`{"name":"Learning","archived":false}`. Names are trimmed, 1–80 Unicode
characters, unique case-insensitively per circle. IDs are stable. Categories are
archived, never deleted. Archived categories remain in statistics and on existing
tasks; they cannot be newly assigned. Rename does not change integration keys.

Task create/edit requests use the existing `points` field (null or an integer
0–1000) and optional `xpCategoryId` (integer or null). Existing clients that omit
`xpCategoryId` on edit retain the current category. Explicit null clears it.
Task authorization is unchanged. A category from another circle is rejected.

## Statistics and integration

Both JWT and existing `secretkey` API-token authentication are supported through
the standard multi-auth middleware. Every request requires active membership in
the authenticated user's current circle, checked against the database. No
cross-circle queries are permitted. A circle member can read other active
members' aggregate XP, as with the existing points leaderboard. These endpoints
do not expose task names, descriptions, or history.

`GET /api/v1/users/{userID}/xp`:

```json
{"user_id":1,"total_xp":80,"categories":{"category_7":50,"uncategorized":30}}
```

`GET /api/v1/circles/{circleID}/xp`:

```json
{"circle_id":1,"leaderboard":[{"user_id":1,"total_xp":80,"categories":{"category_7":50,"uncategorized":30}}]}
```

Category keys are `category_<id>`; resolve display names through the categories
endpoint. `uncategorized` is reserved. All returned values are lifetime earned
points, including legacy points. Reward redemption does not reduce XP. Members
with no earned points receive zero and an uncategorized zero entry. The
leaderboard sorts by total descending, then user ID ascending. Category-specific
ranking can be derived from each member's category map (the frontend does this).
Statistics are read with one SQL statement for snapshot consistency. Awards are
scoped to the membership ID as well as user/circle IDs, so leaving and rejoining
a circle does not attach a previous membership’s awards to a reset balance.

## Persistence, undo, and compatibility

The existing AutoMigrate path adds nullable task category IDs, a default-false
user preference, category definitions, and an award ledger. It does not rewrite
balances or invent historical categories. Existing points without ledger entries
are reported as uncategorized, ensuring categories sum to the existing lifetime
total, even if old task histories have been removed.

Successful point-bearing completion/approval records category attribution in the
same transaction as the existing points credit. An award's history ID is unique.
Task/category edits and history date/note edits never recategorize earned XP.
Pending approval, rejection, skipping, and completions where points are disabled
do not earn XP. Approval uses the task configuration at approval time, matching
the existing points award behavior. Deleting a task or its displayed history
retains lifetime XP, matching existing points behavior. Undo deletes the award
and subtracts the immutable award amount from its original membership atomically.
A departed membership cannot debit a later rejoined membership. Legacy and
uncategorized completions have no category award; their existing history-based
reversal behavior is retained. Stale concurrent
completions, including pending-approval transitions, and approvals are rejected; a client must reload the task. Fresh subsequent
completions of an always task are distinct awards.

Validation: run `go test ./internal/chore/repo ./internal/xp ./internal/database`
and the broader existing suite. No external services or credentials are required
for these regression tests. PostgreSQL locking follows the existing dialect
conventions; SQLite tests do not substitute for a PostgreSQL integration run.
