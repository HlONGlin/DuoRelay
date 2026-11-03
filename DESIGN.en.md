---
# [简体中文](DESIGN.md) | English
version: beta
name: Iceflow-linear-route-light-default
description: "Project-authored design system, light by default. It uses Linear's system discipline (layered surfaces, hairlines, one accent, screenshot-led composition, restrained motion) but reworks the visual language for this project: a light-blue accent scale, blue-gray neutrals, marketing/product type tracks, Chinese typography rules, comfortable/compact density, semantic data colors, and native proxy-panel components (node rows, latency bars, subscription cards, kernel switch). This is not a recolored Linear theme."
source: "Methodology referenced from VoltAgent/awesome-design-md — design-md/linear.app (MIT License). All token values, type scales, and components in this file are re-authored for this project."

colors:
  primary: "#2f6bff"
  on-primary: "#ffffff"
  primary-hover: "#1f5af0"
  primary-active: "#1a4cd4"
  primary-focus: "#2f6bff"
  primary-soft: "#e9effe"
  primary-deep: "#173a8f"
  ink: "#0a0f1c"
  ink-muted: "#3c4659"
  ink-subtle: "#68738a"
  ink-tertiary: "#9aa4b8"
  canvas: "#ffffff"
  surface-1: "#f5f8fc"
  surface-2: "#edf2f9"
  surface-3: "#e2eaf4"
  surface-4: "#d3dfef"
  hairline: "#e2eaf3"
  hairline-strong: "#c9d6e6"
  hairline-tertiary: "#b3c3da"
  inverse-canvas: "#05070d"
  inverse-surface-1: "#0b0f16"
  inverse-surface-2: "#10151f"
  inverse-ink: "#f4f7fb"
  semantic-success: "#1f9d55"
  semantic-warning: "#d97706"
  semantic-danger: "#dc2626"
  latency-good: "#1f9d55"
  latency-mid: "#d97706"
  latency-bad: "#dc2626"
  latency-dead: "#9aa4b8"
  chart-line: "#2f6bff"
  chart-line-2: "#9aa4b8"
  chart-fill: "#2f6bff"
  semantic-overlay: "#0a0f1c"

typography:
  display-xl:
    fontFamily: Inter
    fontSize: 76px
    fontWeight: 600
    lineHeight: 1.06
    letterSpacing: -0.038em
  display-lg:
    fontFamily: Inter
    fontSize: 52px
    fontWeight: 600
    lineHeight: 1.10
    letterSpacing: -0.032em
  display-md:
    fontFamily: Inter
    fontSize: 38px
    fontWeight: 600
    lineHeight: 1.15
    letterSpacing: -0.026em
  headline:
    fontFamily: Inter
    fontSize: 27px
    fontWeight: 600
    lineHeight: 1.22
    letterSpacing: -0.02em
  card-title:
    fontFamily: Inter
    fontSize: 21px
    fontWeight: 500
    lineHeight: 1.28
    letterSpacing: -0.015em
  subhead:
    fontFamily: Inter
    fontSize: 19px
    fontWeight: 400
    lineHeight: 1.45
    letterSpacing: -0.008em
  body-lg:
    fontFamily: Inter
    fontSize: 17px
    fontWeight: 400
    lineHeight: 1.55
    letterSpacing: 0
  body:
    fontFamily: Inter
    fontSize: 16px
    fontWeight: 400
    lineHeight: 1.55
    letterSpacing: 0
  body-sm:
    fontFamily: Inter
    fontSize: 14px
    fontWeight: 400
    lineHeight: 1.50
    letterSpacing: 0
  caption:
    fontFamily: Inter
    fontSize: 12px
    fontWeight: 400
    lineHeight: 1.45
    letterSpacing: 0
  button:
    fontFamily: Inter
    fontSize: 14px
    fontWeight: 500
    lineHeight: 1.20
    letterSpacing: 0
  eyebrow:
    fontFamily: Inter
    fontSize: 13px
    fontWeight: 500
    lineHeight: 1.30
    letterSpacing: 0.06em
  p-display:
    fontFamily: Inter
    fontSize: 30px
    fontWeight: 600
    lineHeight: 1.15
    letterSpacing: -0.02em
  p-headline:
    fontFamily: Inter
    fontSize: 20px
    fontWeight: 600
    lineHeight: 1.30
    letterSpacing: -0.01em
  p-card-title:
    fontFamily: Inter
    fontSize: 15px
    fontWeight: 500
    lineHeight: 1.35
    letterSpacing: 0
  p-body:
    fontFamily: Inter
    fontSize: 14px
    fontWeight: 400
    lineHeight: 1.45
    letterSpacing: 0
  p-body-sm:
    fontFamily: Inter
    fontSize: 13px
    fontWeight: 400
    lineHeight: 1.45
    letterSpacing: 0
  p-caption:
    fontFamily: Inter
    fontSize: 12px
    fontWeight: 400
    lineHeight: 1.40
    letterSpacing: 0
  p-micro:
    fontFamily: Inter
    fontSize: 11px
    fontWeight: 500
    lineHeight: 1.30
    letterSpacing: 0.02em
  mono:
    fontFamily: JetBrains Mono
    fontSize: 13px
    fontWeight: 400
    lineHeight: 1.50
    letterSpacing: 0
  mono-data:
    fontFamily: JetBrains Mono
    fontSize: 13px
    fontWeight: 500
    lineHeight: 1.40
    letterSpacing: 0

rounded:
  xs: 4px
  sm: 6px
  md: 8px
  lg: 10px
  xl: 14px
  xxl: 20px
  pill: 9999px
  full: 9999px

spacing:
  xxs: 4px
  xs: 8px
  sm: 12px
  md: 16px
  lg: 24px
  xl: 32px
  xxl: 48px
  section: 96px

motion:
  duration-instant: 80ms
  duration-fast: 120ms
  duration-base: 180ms
  duration-slow: 280ms
  easing-out: "cubic-bezier(0.2, 0, 0, 1)"
  easing-standard: "cubic-bezier(0.4, 0, 0.2, 1)"
  easing-in: "cubic-bezier(0.4, 0, 1, 1)"

components:
  button-primary:
    backgroundColor: "{colors.primary}"
    textColor: "{colors.on-primary}"
    typography: "{typography.button}"
    rounded: "{rounded.md}"
    padding: 8px 14px
  button-primary-hover:
    backgroundColor: "{colors.primary-hover}"
    textColor: "{colors.on-primary}"
    typography: "{typography.button}"
    rounded: "{rounded.md}"
  button-primary-active:
    backgroundColor: "{colors.primary-active}"
    textColor: "{colors.on-primary}"
    typography: "{typography.button}"
    rounded: "{rounded.md}"
  button-secondary:
    backgroundColor: "{colors.canvas}"
    textColor: "{colors.ink}"
    typography: "{typography.button}"
    rounded: "{rounded.md}"
    padding: 8px 14px
  button-tertiary:
    backgroundColor: "{colors.surface-1}"
    textColor: "{colors.ink}"
    typography: "{typography.button}"
    rounded: "{rounded.md}"
    padding: 8px 14px
  button-inverse:
    backgroundColor: "{colors.inverse-canvas}"
    textColor: "{colors.inverse-ink}"
    typography: "{typography.button}"
    rounded: "{rounded.md}"
    padding: 8px 14px
  feature-card:
    backgroundColor: "{colors.surface-1}"
    textColor: "{colors.ink}"
    typography: "{typography.body}"
    rounded: "{rounded.lg}"
    padding: 24px
  pricing-card:
    backgroundColor: "{colors.surface-1}"
    textColor: "{colors.ink}"
    typography: "{typography.body}"
    rounded: "{rounded.lg}"
    padding: 24px
  pricing-card-featured:
    backgroundColor: "{colors.canvas}"
    textColor: "{colors.ink}"
    typography: "{typography.body}"
    rounded: "{rounded.lg}"
    padding: 24px
  product-screenshot-card:
    backgroundColor: "{colors.surface-1}"
    textColor: "{colors.ink}"
    typography: "{typography.body}"
    rounded: "{rounded.xl}"
    padding: 20px
  text-input:
    backgroundColor: "{colors.canvas}"
    textColor: "{colors.ink}"
    typography: "{typography.body}"
    rounded: "{rounded.md}"
    padding: 8px 12px
  pricing-tab-default:
    backgroundColor: "{colors.canvas}"
    textColor: "{colors.ink-subtle}"
    typography: "{typography.button}"
    rounded: "{rounded.pill}"
    padding: 6px 14px
  pricing-tab-selected:
    backgroundColor: "{colors.surface-2}"
    textColor: "{colors.ink}"
    typography: "{typography.button}"
    rounded: "{rounded.pill}"
    padding: 6px 14px
  cta-banner:
    backgroundColor: "{colors.inverse-canvas}"
    textColor: "{colors.inverse-ink}"
    typography: "{typography.headline}"
    rounded: "{rounded.lg}"
    padding: 48px
  changelog-row:
    backgroundColor: "{colors.canvas}"
    textColor: "{colors.ink}"
    typography: "{typography.body}"
    rounded: "{rounded.xs}"
    padding: 24px 0
  status-badge:
    backgroundColor: "{colors.surface-2}"
    textColor: "{colors.ink-muted}"
    typography: "{typography.caption}"
    rounded: "{rounded.pill}"
    padding: 2px 8px
  top-nav:
    backgroundColor: "{colors.canvas}"
    textColor: "{colors.ink}"
    typography: "{typography.body-sm}"
    rounded: "{rounded.xs}"
    height: 56px
  footer:
    backgroundColor: "{colors.canvas}"
    textColor: "{colors.ink-subtle}"
    typography: "{typography.caption}"
    rounded: "{rounded.xs}"
    padding: 64px 32px
  sidebar-nav:
    backgroundColor: "{colors.surface-1}"
    textColor: "{colors.ink-muted}"
    typography: "{typography.p-body}"
    rounded: "{rounded.xs}"
    padding: 16px 12px
  sidebar-nav-item-active:
    backgroundColor: "{colors.primary-soft}"
    textColor: "{colors.primary-deep}"
    typography: "{typography.p-body}"
    rounded: "{rounded.sm}"
    padding: 6px 10px
  stat-card:
    backgroundColor: "{colors.surface-1}"
    textColor: "{colors.ink}"
    typography: "{typography.p-body}"
    rounded: "{rounded.lg}"
    padding: 16px
  node-row:
    backgroundColor: "{colors.canvas}"
    textColor: "{colors.ink}"
    typography: "{typography.p-body}"
    rounded: "{rounded.xs}"
    padding: 0 12px
  node-row-selected:
    backgroundColor: "{colors.primary-soft}"
    textColor: "{colors.ink}"
    typography: "{typography.p-body}"
    rounded: "{rounded.xs}"
    padding: 0 12px
  latency-bar:
    backgroundColor: "{colors.surface-3}"
    textColor: "{colors.ink-muted}"
    typography: "{typography.mono-data}"
    rounded: "{rounded.full}"
    padding: 0
  subscription-card:
    backgroundColor: "{colors.surface-1}"
    textColor: "{colors.ink}"
    typography: "{typography.p-body}"
    rounded: "{rounded.lg}"
    padding: 16px
  core-switch:
    backgroundColor: "{colors.surface-2}"
    textColor: "{colors.ink-subtle}"
    typography: "{typography.button}"
    rounded: "{rounded.pill}"
    padding: 3px
  core-switch-selected:
    backgroundColor: "{colors.canvas}"
    textColor: "{colors.ink}"
    typography: "{typography.button}"
    rounded: "{rounded.pill}"
    padding: 5px 12px
  data-table:
    backgroundColor: "{colors.canvas}"
    textColor: "{colors.ink}"
    typography: "{typography.p-body}"
    rounded: "{rounded.xs}"
    padding: 0
  chart-panel:
    backgroundColor: "{colors.surface-1}"
    textColor: "{colors.ink}"
    typography: "{typography.p-body}"
    rounded: "{rounded.lg}"
    padding: 16px
  toast:
    backgroundColor: "{colors.inverse-canvas}"
    textColor: "{colors.inverse-ink}"
    typography: "{typography.p-body-sm}"
    rounded: "{rounded.md}"
    padding: 10px 14px
---

## Overview

This is the default theme and the primary design-system specification. Its relationship to Linear is **borrow the methods, not the skin**:

Only four methods are borrowed: (1) establish hierarchy with surface layers and hairlines, not shadows or gradients; (2) allow one accent color site-wide and use it sparingly; (3) make real product UI/screenshots the focus and keep marketing chrome secondary; (4) keep motion restrained and state-driven.

Everything else is redesigned for this project: color scales, dual marketing/product type tracks and Chinese rules, radii, density, data palette, native panel components, and motion values. An output that looks like the Linear website has missed the goal: give this proxy panel its own precise visual identity.

## Colors

### Accent: Signal Blue
- `{colors.primary}` #2f6bff is the only accent. Its state scale includes darker hover/active values, a 50%-transparent focus outline, `soft` for selected/subtle emphasis backgrounds, and `deep` for readable accent text on light surfaces.
- Accent is allowed in six places only: brand mark, primary CTA, focus ring, emphasized link, selected state, and main chart line. Any other colored accent is a violation.
- The near-black-blue inverse block is not an accent; it is a closing block. Use at most one per page (footer CTA or theme-switch demo).

### Neutrals: Blue-Gray Scale
- Surface levels have a clear blue tint (surface-1 #f5f8fc → surface-4 #d3dfef). Do not replace them with plain gray; blue-gray surfaces distinguish this system from a generic grayscale template.
- Four ink levels: near-black-blue `ink`, then `muted`, `subtle`, and `tertiary`, with decreasing contrast and importance.

### Semantic and Data Colors (Functional Use Only)
- `success` / `warning` / `danger` indicate state only: running, subscription expiry, or connection failure. Never use them for headings, buttons, or decoration.
- Latency is core product language with four fixed site-wide tiers: ≤100ms `latency-good`, ≤250ms `latency-mid`, >250ms `latency-bad`, timeout/unavailable `latency-dead` (gray). Apply tiers only to dots, values, or thin bars, covering no more than 5% of a row.
- Chart palette: `chart-line` (same as primary), `chart-line-2` (gray dashed comparison), and 8%-opacity primary area fill. No third chart color.

## Typography

### Dual Type Tracks
- **Marketing track** (website): display-xl/lg/md, headline, card-title, subhead, body-lg, and body; spacious with tightened display tracking.
- **Product track** (panel): p-display, p-headline, p-card-title, p-body, p-body-sm, p-caption, and p-micro; one step smaller overall with line height around 1.45 for information density. Never mix tracks: no display-sized type in the panel and no `p-body` in website article copy.
- Numbers, latency, traffic, ports, and protocol names always use mono/mono-data with tabular numerals, preventing row jitter during refresh.

### Chinese Typography Rules (A New Layer)
- Chinese headings must not use large negative tracking: set `letterSpacing` to 0, at most -0.01em. Negative-tracking tables apply to Latin-only headings; mixed Chinese/Latin headings follow Chinese rules.
- Chinese body line height is 0.1 above the Latin value (about 1.65 for body); paragraph spacing is at least 0.8× the font size.
- For 600-weight Chinese headings, prefer a font with a real bold face. If only synthetic bold is available, reduce to 500; never fake bold with an outline.
- Do not force spaces between English words/numbers and Chinese; follow the source copy. Numbers in data contexts always switch to mono.
- Font stack: Inter → "PingFang SC" → "Hiragino Sans GB" → "Microsoft YaHei" → system-ui. Do not use serif Chinese headings; this system has no editorial-style branch.

## Layout & Density

- Use a 4px base grid; spacing comes only from spacing tokens.
- **Two densities**: comfortable (website + panel overview): 24/16px card padding and 44px rows. Compact (node lists, connection tables, logs): 36px rows, 12px card padding, p-body-sm. Density belongs to the container; do not mix row heights on one page.
- Width: website 1280px; panel is fluid/full-width with 24px side padding; tables may reach the edges.
- Website rhythm: 96px between sections. Separate sections with whitespace and surface elevation, not dividers. Dividers are for content within a section (table/log rows and settings groups).

## Motion (Make the Product's Smoothness Measurable)

- Duration tiers: instant 80ms (hover surface), fast 120ms (switches/badges/row selection), base 180ms (panel/dialog/theme), slow 280ms (rare page transitions). Never exceed 280ms.
- Easing: easing-out on enter, easing-in on exit, easing-standard for two-way transitions. **No bounce, rebound, or overshoot.**
- Data refresh: transition list values by color/opacity only, never position. Test progress grows linearly with mono numeric roll; no spring motion.
- Theme switch: transition background-color, border-color, and color only, for 150ms. Images/screenshots do not transition.
- Respect `prefers-reduced-motion` globally: set durations to zero and keep only instantaneous state changes.

## Elevation & Depth

| Level | Treatment | Use |
|---|---|---|
| 0 | No border/background | Article, heading area, footer |
| 1 | surface-1 + 1px hairline | Default card, panel, screenshot frame |
| 2 | surface-2 / white + hairline-strong | Emphasis, hover, pre-selection |
| 3 | surface-3 | Nested surface, subnav, latency-bar/core-switch track |
| Overlay | White + hairline + `0 8px 30px rgba(10,15,28,0.08)` | Dialog, command palette, menu, toast (inverse toast surface) |
| Focus | 2px primary-focus outline at 50% opacity | Focused input/button |

## Components (Native Panel Components Are Central)

Marketing components (button family, feature-card, pricing-card, product-screenshot-card, cta-banner, top-nav, footer, changelog-row, pricing-tab) follow the front-matter tokens. Key rules: one primary button per viewport; 8px button radius, never pill-shaped; screenshot frame uses xl=14px radius and 20px padding.

Native panel components:
- **sidebar-nav**: left rail on surface-1. Selected item uses primary-soft, primary-deep text, and 6px radius; never indicate selection with a left border stripe.
- **stat-card**: large mono-data value (compact p-display may be 24px/600), p-caption label. Change arrows may use semantic colors only and must appear beside a value.
- **node-row**: compact 36px row with fixed structure: status dot / name / protocol badge / latency-bar / right-aligned mono latency / actions. Selected row uses primary-soft; unavailable row uses ink-tertiary throughout. Separate rows with hairlines.
- **latency-bar**: 48px thin bar on surface-3, filled by latency tier; fill ratio = min(latency/300ms, 1). It is redundant visualization, never standalone; pair it with the numeric value.
- **subscription-card**: name, usage progress (thin surface-3 track, primary fill; warning above 85%; danger text when expired), mono expiry date, tertiary renew/update action.
- **core-switch**: segmented sing-box/mihomo control on surface-2; selected segment is raised white with ink text, unselected is ink-subtle. Use motion-base background slide only; text does not move.
- **data-table**: uppercase p-micro/ink-subtle header, hairline rows, right-aligned mono numbers, surface-1 row hover, ink-tertiary sort arrows.
- **chart-panel**: title row (p-card-title + range pills) and plot; use chart palette only, hairline grid, p-micro/ink-tertiary axes.
- **toast**: near-black inverse surface with white text, bottom-right; enter with motion-base fade + 4px rise. Use a danger semantic icon only for errors.

## Icons

- Use linear icons with 1.5px strokes and rounded ends, in 16/20/24px sizes. Do not mix in filled icons.
- Icons inherit text color. Do not apply accent/semantic colors to icons except status dots.

## Do's and Don'ts

### Do
- Choose surface level and density before implementing a component.
- Use mono + tabular numerals for all data; keep four latency tiers consistent site-wide.
- Set Chinese heading tracking to zero; keep website and panel type tracks separate.
- Follow the motion table and remove anything over 280ms.
- Add tokens before new components; follow `xxx-card` / `xxx-row` / `xxx-badge` naming.

### Don't
- Do not make this look like the Linear website: do not copy its copy structure, use purple/purple-tinted accent, or assume English-first typography.
- Do not use large accent fills, gradients, glow, or glassmorphism.
- Do not layer cards with shadows (overlays are the exception).
- Do not apply Latin negative-tracking tables to Chinese headings.
- Do not add off-palette chart colors or vary latency thresholds between pages.
- Do not use bounce/rebound motion or layout shifts during data refresh.

## Responsive Behavior

| Breakpoint | Width | Behavior |
|---|---|---|
| Desktop-XL | 1440px | Default layout |
| Desktop | 1280px | Website cards in 3 columns; panel sidebar expanded |
| Tablet | 1024px | Cards in 2 columns; panel sidebar becomes icon rail |
| Mobile-Lg | 768px | Comparison table becomes accordion; hamburger nav; hide node latency-bar, keep value |
| Mobile | 480px | Single column; display-xl around 34px; panel sidebar becomes bottom nav |

- Touch targets ≥40px (compact rows automatically increase to 44px on touch devices).
- Preserve screenshot/chart aspect ratios; on mobile, tables scroll horizontally instead of compressing columns.

## Iteration Guide

1. Change one component at a time and reference component token names.
2. Before adding a section, answer: which surface level? which density? what is the focal element?
3. When extending tokens, edit this file's front matter first, then implementation, then the matching token in `DESIGN-dark.md`; the structures must mirror.
4. After each change, self-review against prompt-pool checklist F1.
