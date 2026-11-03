---
# [简体中文](DESIGN-dark.md) | English
version: beta
name: Iceflow-linear-route-dark
description: "Optional, non-default dark theme for the project-authored design system. It mirrors the light theme's type tracks (including Chinese rules), radii, spacing, motion, and component names; only colors differ. The canvas is blue-tinted near-black #05070d, never pure black, and the accent is a brighter signal blue for dark-theme contrast."
source: "Companion to DESIGN.md (light, default). Structure mirrors it exactly; only color values differ."

colors:
  primary: "#6e97ff"
  on-primary: "#ffffff"
  primary-hover: "#8fb0ff"
  primary-active: "#a9c2ff"
  primary-focus: "#6e97ff"
  primary-soft: "#131f3d"
  primary-deep: "#a9c2ff"
  ink: "#f4f7fb"
  ink-muted: "#c3cede"
  ink-subtle: "#8593a9"
  ink-tertiary: "#5b6a82"
  canvas: "#05070d"
  surface-1: "#0b0f16"
  surface-2: "#10151f"
  surface-3: "#151c29"
  surface-4: "#1a2334"
  hairline: "#1d2739"
  hairline-strong: "#2e3c55"
  hairline-tertiary: "#42506e"
  inverse-canvas: "#ffffff"
  inverse-surface-1: "#f5f6f6"
  inverse-surface-2: "#f6f7f7"
  inverse-ink: "#000000"
  semantic-success: "#2fbf71"
  semantic-warning: "#e5a13c"
  semantic-danger: "#f0604f"
  latency-good: "#2fbf71"
  latency-mid: "#e5a13c"
  latency-bad: "#f0604f"
  latency-dead: "#5b6a82"
  chart-line: "#6e97ff"
  chart-line-2: "#5b6a82"
  chart-fill: "#6e97ff"
  semantic-overlay: "#000000"

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
    backgroundColor: "{colors.surface-1}"
    textColor: "{colors.ink}"
    typography: "{typography.button}"
    rounded: "{rounded.md}"
    padding: 8px 14px
  button-tertiary:
    backgroundColor: "{colors.canvas}"
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
    backgroundColor: "{colors.surface-2}"
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
    backgroundColor: "{colors.surface-1}"
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
    backgroundColor: "{colors.surface-1}"
    textColor: "{colors.ink}"
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
    backgroundColor: "{colors.surface-4}"
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
    backgroundColor: "{colors.surface-3}"
    textColor: "{colors.ink}"
    typography: "{typography.p-body-sm}"
    rounded: "{rounded.md}"
    padding: 10px 14px
---

## Relationship to the Light Theme

This file is a **structural mirror** of `DESIGN.md` (light, default): typography, rounded, spacing, motion, and components must match item-for-item. Do not add or remove any item in only the dark theme. CSS variables switch the themes, with one-to-one variable names.

There are only four dark-theme-specific differences beyond color values:
- **Hover direction**: primary button hover gets lighter (primary → primary-hover → primary-active); light mode gets progressively darker.
- **Elevation**: no shadows. A raised panel may have a 1px, very low-opacity white top highlight for subtle sheen. No glow or colored `box-shadow` halos.
- **cta-banner**: use a surface-1 block in dark mode, not an inverse white block. A white `button-inverse` is the strongest CTA; at most one per page.
- **Toast**: use surface-3 with hairline-strong border, not an inverse block.

All other rules—six accent placements, four latency tiers, dual type tracks, Chinese typography, dual density, motion, Do's and Don'ts, and responsive breakpoints—match `DESIGN.md`; use that file as the reference.
