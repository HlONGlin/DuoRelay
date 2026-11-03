# DuoRelay UI v2 Refactor Acceptance

[简体中文](UI-v2-重构验收.md) | English

Date: 2026-11-01

This iteration refactored the existing web panel using the updated `DESIGN.md`, `DESIGN-dark.md`, and `提示池.md` supplied by the user. The scope covered five product pages and the shared design system. The marketing site, promotional pages, and optional components from the prompt pool were not implemented as new product features.

## Design Sources and Implementation

The SHA-256 hashes of the three source files match the project copies. The files are retained at the repository root as `DESIGN.md`, `DESIGN-dark.md`, and `docs/ui-prompts.md`.

| Specification | Implementation |
|---|---|
| Colors, type scales, radii, spacing, and motion for both themes | `web/scripts/sync-design.mjs` generates `tokens.css`; it runs before builds and checks that both theme structures match. |
| Light theme by default, manual switching, and persistence | The appearance store is shared by the top-bar button and appearance settings. |
| Product type scale, zero tracking for Chinese headings, tabular data | `main.css` and the `type-p-*` and `type-mono-*` utility classes. |
| Surface hierarchy and hairlines, without card shadows | Five pages use canvas, surface, and hairline tokens; shadows are reserved for overlays. |
| Linear icons | `UiIcon`, with a 1.5px stroke and 16 / 20 / 24px sizes. |
| Selected sidebar state | `primary-soft` background, `primary-deep` text, and a 6px radius. |
| Comfortable / compact density | Node rows at 44 / 36px; touch and narrow layouts use 44px automatically. |
| Latency bands | ≤100ms, ≤250ms, >250ms, and gray for unavailable; a 48px bar sits beside the value. |
| Charts | Solid primary-color upload line with 8% fill, dashed gray download line, based on recent real samples. |
| Responsive behavior | Icon sidebar at 1024px, hamburger navigation at 768px, bottom navigation at 480px; tables scroll horizontally. |
| Reduced motion | Global `prefers-reduced-motion` override. |

## Page Changes

- **Dashboard**: real node/subscription counts, runtime status, traffic accumulated during the current kernel session, current node, and a recent traffic chart. Without history, the UI shows waiting or unavailable states.
- **Node management**: paginated retrieval, virtual scrolling, filtering, sorting, multi-node tests, up to 4 concurrent tests, progress, and cancellation of pending work. Retains “Select and start” and “Stop and clear selection”; the desktop action column stays fixed on the right. Compact density is the default.
- **Subscriptions**: neutral rows, real node counts, thin usage bars, warnings above 85%, expiry notices, and refresh actions.
- **Kernel status**: unified status, traffic, logs, and connection tables; includes hit-rule information already returned by the API and right-aligns numeric columns.
- **Settings**: General / Kernel / Routing rules / Subscriptions / Appearance / Advanced sections; previews for both themes; a bottom-right save bar; rule values open an editor; unmatched traffic has a separate selectable outbound.
- **Development verification page**: `/design-system`, registered only in development, checks both type scales and themes, latency, buttons, inputs, dialogs, and toasts.

## Interaction Issues Also Fixed

- A successful speed test, a returned failure result, or a request error no longer clears the selected nodes.
- If a later page request fails, the previous complete node list remains visible instead of showing partial data.
- Switching routing groups preserves custom rules and only adds or removes exact matches of built-in rules.
- Restoring defaults edits the form draft first and does not prematurely mutate the configuration store.
- Mixed proxy ports for sing-box and mihomo are distinguished; `mixed_port` is no longer labeled as a Clash API port.
- Rule editor and theme buttons explicitly use `type=button`; forms prevent default submission so drafts are not lost to a page refresh.

## Verification Results

- `npm run type-check`: passed.
- `npm test -- --run`: 9 test files and 35 tests passed.
- `npm run build`: passed, including automatic `design:sync`; the development verification page was excluded from the production bundle.
- `git diff --check -- web`: passed.
- Orca browser: 20 states across five pages, 1440px / 390px viewports, and light/dark themes; `document.scrollWidth` did not exceed the viewport width.
- Also checked the 1024px icon sidebar, 768px hamburger navigation, and its expanded interaction.
- Opened the rule editor and checked its content; inputs and overlays had no light-theme leakage in dark mode.
- Appearance settings displayed light and dark previews while the page itself was dark.
- Confirmed the toast appears at the bottom right and that switching back to light theme persists.
- With 10,000 temporary browser nodes, the initial render showed 25 rows; scrolling to the end showed 19 rows and node 10,000.
- Simulated batch testing completed all 12 tasks with a maximum concurrency of 4; action buttons returned to their idle state.
- Temporary test data stayed in browser memory and was removed afterward. No real nodes, subscriptions, or kernel configuration were changed.

Local screenshots and matrix records are in ignored temporary artifacts under `build/`, including `ui-final-dashboard-light.png`, `ui-settings-390-dark-stable.png`, `ui-routing-dialog-dark.png`, `ui-audit-1440.json`, and `ui-audit-390.json`.

## Boundaries

- This acceptance covered the frontend UI and interactions. It did not restart real nodes or verify live proxy connectivity. The kernel was stopped during inspection, so the traffic area correctly showed unavailable.
- The traffic API provides recent samples and totals for the current run. It does not fabricate “today” statistics, 1h / 24h / 7d history, or an exit IP.
- The specified font fallback stack is used without downloading external fonts; rendered glyphs depend on fonts installed on the system.
- Actual “bypass mainland China” coverage still depends on existing backend rules and data. This UI refactor did not add a complete mainland IP/domain database.
