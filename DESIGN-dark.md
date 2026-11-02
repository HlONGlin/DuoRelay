---
version: beta
name: Iceflow-linear-route-dark
description: "项目自有设计系统的深色主题（可选，非默认）。与默认浅色主题结构完全镜像：同一套字阶（含营销/产品双轨与中文规则）、圆角、间距、动效、组件命名，仅 colors 不同。画布为带蓝调的近黑 #05070d（禁用纯黑），accent 为提亮版信号蓝以保证深色对比。"
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

## 与浅色主题的关系

本文件与 DESIGN.md（浅色，默认）**结构镜像**：typography、rounded、spacing、motion、components 逐项一致，禁止任何一项只在深色里增删。两套主题靠 CSS 变量切换，变量名一一对应。

深色专属差异（除色值外只有这四条）：
- **hover 方向**：主按钮 hover 变亮（primary → primary-hover → primary-active 逐级提亮）；浅色为逐级变深。
- **抬升表达**：不用阴影；抬升面板可在顶部加 1px 极低透明度白色高光边，模拟微光泽。禁止发光（box-shadow 彩色光晕）。
- **cta-banner**：深色下用 surface-1 块面即可，不动用 inverse 白块；button-inverse（白按钮）在深色页里是最强 CTA，一页至多一处。
- **toast**：深色下用 surface-3 底 + hairline-strong 边框，不用反色块。

其余全部规则——accent 限量六处、延迟四档分级、双轨字阶、中文规则、双密度、动效参数、Do's and Don'ts、响应式断点——与 DESIGN.md 完全一致，以那份为准。
