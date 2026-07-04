# Donetick Testing Philosophy

Thin, robust tests that give confidence and stay out of the way. We test **user-facing behavior**, not implementation details. Quality and speed beat coverage numbers.

## Shape

Different shapes for the two halves of the stack:

- **Backend (Go API):** classic pyramid — a wide base of fast table-driven unit tests, a focused layer of HTTP + DB integration tests, almost no E2E.
- **Frontend (React):** testing trophy — *mostly* component/integration tests via React Testing Library. *"Write tests. Not too many. Mostly integration."*
- **E2E:** a thin cap over both, for **critical user flows only**.

## Critical flows (the ones worth E2E)

Only these get end-to-end coverage. More E2E is negative ROI (slow + flaky).

1. Sign in — local **and** OIDC
2. Create a task
3. Complete a task → points awarded + recurrence advances
4. Assignee rotation
5. Circle sharing

## Stack

| Layer | Tools |
|---|---|
| Go unit | stdlib `testing` + `testify`, **table-driven** with `t.Run` subtests |
| Go handlers | `httptest` against the Gin router — assert status + JSON |
| Go integration | **testcontainers-go against real Postgres** (we ship Postgres; GORM differs per driver) |
| React component | **Vitest + React Testing Library + user-event**, network mocked with **MSW** |
| E2E / responsive / a11y | **Playwright** + `@axe-core/playwright`; **Lighthouse CI** for PWA/perf |

## Rules that keep tests useful

- **Test behavior, not internals.** No snapshot tests, no reaching into component state, no asserting "request X was made" (assert what the user sees instead).
- **Prefer real dependencies** over mocks where cheap (real Postgres, real router).
- **Flaky = broken.** Quarantine or delete flaky tests immediately; never let them erode trust in the gate. Playwright runs with `retries: 1` in CI.
- **Keep CI fast** (~10 min target). Slow blocking checks are the biggest drain on CI value.

## When tests run — block vs. warn

Block on fast + reliable. Warn on slow + fuzzy.

| Stage | Runs | Gate |
|---|---|---|
| **pre-commit** (<5s, changed files) | `gofmt`/`goimports`, `eslint --fix`, `golangci-lint` | **block** |
| **pre-push** (<2 min) | unit tests, typecheck | block (optional) |
| **CI — required** | `go test ./...`, `vitest`, frontend build, lint, Playwright critical-flow smoke, axe (critical/serious) | **block merge** |
| **CI — advisory** | Lighthouse CI, coverage report, visual regression, mutation tests | **warn only** |

## Accessibility

- **Target WCAG 2.2 AA, not AAA.** W3C says AAA is not an appropriate blanket goal. Adopt specific AAA criteria where cheap (e.g. 7:1 contrast on primary text).
- Contrast: **AA = 4.5:1** normal text (non-negotiable, audit light **and** dark themes). AAA = 7:1.
- **Automation catches only ~30–57% of a11y issues** — necessary but not sufficient. Automated axe in CI (fail on critical/serious first) **plus** manual keyboard-only + real screen-reader (VoiceOver/NVDA) passes **quarterly or before major releases**.

## PWA / Capacitor / mobile

- Web E2E (Playwright) tests the **WebView, not the native app** — it can't touch native UI, iOS WKWebViews, or permission dialogs (NFC, push, social login).
- ~90% of logic is in the shared web layer, so **get most confidence there** with Playwright + device emulation.
- **PWA offline / service worker:** Lighthouse CI PWA audits + manual DevTools offline checks.
- **Native layer:** defer automated native E2E (Appium/WebdriverIO carries real operational cost; real devices mandatory). **Manually smoke-test NFC / push / social-login on a real device per release** until scale justifies more.

## Coverage

Measure, don't target. Coverage finds untested code but is a poor quality signal; a **% target is counterproductive** (Goodhart's law) and 100% is a smell. The real signal: **bugs rarely reach production and developers feel safe changing code.** Report on PRs (advisory), optionally ratchet-don't-drop. For high-value logic (scheduler/recurrence), **mutation testing** beats line coverage.

## Running the tests

```bash
# Backend (from be/)
go test ./...                       # all unit + SQLite tests (fast, no Docker)
go test ./internal/chore/repo/ -v   # one package, verbose
go test -tags integration ./...     # + Postgres integration tests (needs Docker)

# Frontend unit/component (from fe/)
npm test              # watch mode
npm run test:run      # single run (what CI runs)
npm run test:coverage # coverage report (advisory, never gated)

# Frontend E2E + a11y (from fe/) — needs the backend checked out at ../be
npm run test:e2e      # Playwright: desktop + mobile, starts both servers
npm run test:e2e:ui   # interactive UI mode for debugging

# Lighthouse (from fe/) — advisory PWA/a11y/perf, never blocks
npm run build && npx lhci autorun
```

E2E starts the Go backend (temp SQLite DB) and the Vite dev server automatically
(`playwright.config.js`). Point it at a different backend dir with `BE_DIR=...`.
Selectors use accessible names / roles / text so they survive the upcoming
**MUI Joy → shadcn** migration — don't couple E2E to component-library DOM.

Reference examples to copy when adding tests:
- Go pure unit (table-driven): `be/internal/auth/password_test.go`
- Go DB integration (in-memory SQLite + real migrations): `be/internal/chore/repo/repository_test.go`
- FE pure unit (table-driven): `fe/src/utils/DurationUtils.test.js`
- FE component + a11y (RTL + user-event + axe): `fe/src/components/common/DurationInput.test.jsx`
- FE network mocking: `fe/src/test/msw.js` + `fe/src/test/setup.js`
- Go Postgres integration (testcontainers, `//go:build integration`): `be/internal/chore/repo/repository_pg_integration_test.go`
- E2E critical flows (Playwright): `fe/e2e/auth.spec.js`, `fe/e2e/tasks.spec.js`
- E2E accessibility (axe-core, known-issue baseline): `fe/e2e/a11y.spec.js`
- Lighthouse config (advisory): `fe/lighthouserc.json`

**a11y baseline:** `fe/e2e/a11y.spec.js` quarantines the current MUI Joy critical/serious
violations (contrast, labels, button-name) so it catches NEW regressions today. As the
shadcn migration fixes each one, shrink the `KNOWN` lists; when a page's list is empty it
becomes fully gated.

## Don't over-invest in

Large E2E suites · automated native iOS/Android E2E (for now) · chasing a coverage number · snapshot / internals tests · asserting on outgoing requests · WCAG AAA sitewide.

---

*Derived from consensus guidance: Google Testing Blog, Martin Fowler (test shapes / CI / coverage), Kent C. Dodds & Testing Library, MSW docs, go.dev table-driven tests, W3C WCAG 2.2, Playwright & Capacitor/Ionic docs, WebAIM/Deque a11y data.*
