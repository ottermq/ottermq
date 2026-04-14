# VHost Switcher — Implementation Plan

## Problem

The web UI hardcodes the virtual host `/` in seven places across two stores and two pages. This means users cannot operate on any vhost other than the default, making the UI unusable in multi-tenant deployments.

**Affected files:**

| File | Hardcoded occurrences |
|------|-----------------------|
| `src/stores/exchanges.js` | 5 — `deleteExchange`, `fetchBindings`, `addBinding`, `deleteBinding`, `publish` |
| `src/stores/queues.js` | 3 — `addQueue`, `deleteQueue`, `get` |
| `src/pages/ExchangesPage.vue` | 2 — form default `vhost: '/'`, vhost dropdown `:options="['/']"` |
| `src/pages/QueuesPage.vue` | 2 — form default `vhost: '/'`, vhost dropdown `:options="['/']"` |

---

## Solution

Introduce a **global `vhosts` Pinia store** as the single source of truth for:
- The list of available vhosts (fetched from `GET /api/vhosts`)
- The currently selected vhost

Add a **vhost selector dropdown** to the main header in `MainLayout.vue`. All stores and pages read the selected vhost from this store instead of hardcoding `/`.

---

## Implementation Steps

### Step 1 — Create `src/stores/vhosts.js`

New Pinia store with:
- `state`: `{ items: [], selected: '/', loading: false, error: null }`
- `actions`:
  - `fetch()` — `GET /api/vhosts`, populates `items`, preserves `selected` if still valid, falls back to `/`
  - `setSelected(name)` — updates `selected`

### Step 2 — Add vhost selector to `MainLayout.vue`

Add a `q-select` dropdown in the toolbar between the broker info and the username display. It:
- Binds to `vhostsStore.items` (list of vhost names)
- Reads/writes `vhostsStore.selected`
- Fetches the vhosts list on mount

### Step 3 — Update `src/stores/exchanges.js`

- Import `useVHostsStore`
- Replace every hardcoded `'/'` with `useVHostsStore().selected`
- Remove the `vhost` fallback `exchangeData.vhost || '/'` — always use the active vhost from the store

### Step 4 — Update `src/stores/queues.js`

Same as Step 3 — replace all hardcoded `'/'` with `useVHostsStore().selected`.

### Step 5 — Update `ExchangesPage.vue` and `QueuesPage.vue`

- Replace `:options="['/']"` in the vhost dropdown with `:options="vhostsStore.names"` (computed list of vhost names)
- Replace `vhost: '/'` in form defaults with a computed ref that reads `vhostsStore.selected`
- Add a `watch` on `vhostsStore.selected` that calls `exchangesStore.fetch()` / `queuesStore.fetch()` to reload data when the user switches vhosts

---

## Files Changed

| File | Change |
|------|--------|
| `ottermq_ui/src/stores/vhosts.js` | **New** — global vhost store |
| `ottermq_ui/src/layouts/MainLayout.vue` | Add vhost selector dropdown to toolbar |
| `ottermq_ui/src/stores/exchanges.js` | Replace hardcoded `'/'` with active vhost from store |
| `ottermq_ui/src/stores/queues.js` | Replace hardcoded `'/'` with active vhost from store |
| `ottermq_ui/src/pages/ExchangesPage.vue` | Wire vhost selector options + watch for vhost change |
| `ottermq_ui/src/pages/QueuesPage.vue` | Wire vhost selector options + watch for vhost change |

---

## Behaviour After Implementation

- On login, the UI fetches available vhosts and defaults to `/`
- The header always shows the active vhost in a dropdown
- Switching the vhost immediately re-fetches exchanges and queues for the new vhost
- All create/delete/publish/consume operations use the active vhost
- If the active vhost is removed or becomes unavailable, the selector falls back to `/`
