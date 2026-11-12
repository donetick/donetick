# Categorized XP

In Circle Settings, an admin or manager can enable **Use XP instead of Points**
and create, rename, archive or restore categories. This changes the displayed
terminology; it does not reset balances. Task creation and editing let members
choose an award amount and category. The Points/XP page shows each member's
available category balances, manual adjustments and rewards.

XP is spendable. `total_xp` is the existing earned counter minus redeemed XP.
`lifetime_xp` exposes the earned counter, including reversals when a completion
is undone. Category identities remain stable after renaming or archiving.
Archived categories cannot receive new manual awards or task assignments, but
existing task assignments and pending awards retain their original category.

Existing balances remain **Uncategorized** (category ID `0`). The migration adds
columns and tables without inferring past categories from mutable task history.
Deleting history retains its award, matching existing Points behavior. Undoing
a completion reverses the original recipient, amount and category. Submitting
for approval awards nothing until approved; edits made while awaiting approval
do not change the submitted award. An old pending operation cannot charge or
credit a member who left and rejoined.

## API

All paths below are under `/api/v1` and require the existing authentication.
Reads are restricted to active members of the current circle. Category and
terminology updates require an admin or manager; manual adjustments require an
admin. A user ID from another circle is not accessible through these routes.

| Method | Path | Purpose |
| --- | --- | --- |
| GET / PUT | `/xp/settings` | Read/update `{"xpMode": true}` |
| GET / POST | `/xp/categories` | List/create categories |
| PUT | `/xp/categories/{id}` | Rename/archive/restore a category |
| GET | `/users/{id}/xp` | Spendable total and category breakdown |
| GET | `/circles/{id}/xp` | Active members, ordered by spendable total |
| POST | `/users/{id}/xp/adjust` | Categorized manual addition/deduction |

Category writes accept `{"name":"Housework","archived":false}`. Names are
trimmed and unique, without case distinctions, within the circle.

An adjustment body is `{"action":"add","points":25,"categoryId":1}` or
`{"action":"remove","points":10,"categoryId":1}`. Both require a selected
category. Additions require an active named category; deductions can select
Uncategorized or an archived category with a remaining balance. Supply an
`Idempotency-Key` header of 1–128 characters and reuse it when retrying the same
adjustment. Reusing it for a different adjustment returns `409`.

The user response contains `user_id`, `circle_id`, `total_xp`, `lifetime_xp`,
`redeemed_xp`, `categories` and `category_details`. `categories` maps stable
category IDs, serialized as strings, to available balances. `category_details`
contains the IDs, current names, archive state and earned/redeemed/balance
figures. The circle response is `{"circle_id":1,"users":[...]}` using the same
user response shape.

Task create/edit accepts `xpCategoryId` alongside `points`. Omission on edit
preserves the existing category; `null` or `0` clears it to Uncategorized.
Reward requests accept `categoryId` alongside the existing optional `note`.
The selected category and original cost are retained until approval. New
XP-mode reward requests must select a category. A pending request originally
made in Points mode remains approvable after switching modes: it consumes
Uncategorized first, then positive named balances in ID order.

New task/manual amounts are whole numbers up to `1000000000`. Existing signed
earned/redeemed counters remain within their common 32-bit range; aggregate
statistics use 64-bit values. Reward costs can be up to `2147483647`, subject
to both total and selected-category funds. Transactional member/task locks,
state checks and unique event keys keep accounting changes together and reject
stale duplicate completions.

## Validation

Feature regressions can be run with:

```sh
go test ./internal/points/... ./internal/chore/repo ./internal/reward/... ./internal/database -run 'XP|RewardCostBounds'
```

These cover authorization, old balances, category isolation, concurrent spends,
manual retry handling, original completion snapshots, approval/undo, stale
completion requests, rejoined memberships and large counter values. The
matching frontend includes XP translation tests and category-selection flows.
