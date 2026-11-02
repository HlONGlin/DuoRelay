---
version: beta
name: Iceflow-linear-route-light-default
description: "项目自有设计系统（默认浅色）。以 Linear 的系统纪律为方法（表面梯度分层、发丝线边框、单一 accent、截图主导、克制动效），但视觉语言按本项目重做：浅蓝 accent 全状态梯、蓝灰中性色、营销/产品双轨字阶、中文排版规则、舒适/紧凑双密度、数据语义色板，以及代理面板原生组件（节点行、延迟条、订阅卡、内核切换）。不是 Linear 的换色版。"
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

这是默认主题，也是整个设计系统的正文。它和 Linear 的关系是**借方法、不借皮**：

借来的方法只有四条——(1) 层级靠表面梯度 + 发丝线，不靠阴影与渐变；(2) 全站只允许一个彩色 accent，且严格限量使用；(3) 产品 UI 截图/真实界面是页面主角，营销外壳退后；(4) 动效克制、为状态服务。

其余全部按本项目重做：色彩梯度、字阶（营销/产品双轨 + 中文规则）、圆角、密度、数据色板、面板原生组件、动效参数，都是本文件自定义的值。任何"看起来像 Linear 官网"的结果都视为跑偏——目标是"一个代理面板产品自己的精密感"。

## Colors

### Accent：信号蓝
- `{colors.primary}` #2f6bff 是全系统唯一 accent，含完整状态梯：hover 变深、active 更深、focus 同色 50% 透明描边、soft 为选中/弱强调底、deep 为浅底上的 accent 文字（保证对比度）。
- 允许出现的位置共六处：品牌标记、主 CTA、焦点环、链接强调、选中态、数据图主线。除此之外任何彩色都是违规。
- inverse 块面（近黑蓝）不是 accent，是"收官块"：一页至多一处（页尾 CTA 或主题切换演示区）。

### 中性色：蓝灰梯度
- 表面梯度带明确蓝调（surface-1 #f5f8fc → surface-4 #d3dfef），禁止换成无蓝调的纯灰——蓝灰底是本系统区别于"通用灰阶模板"的性格所在。
- 墨色四级：ink 近黑蓝、muted、subtle、tertiary，对比度逐级递减，用途逐级边缘化。

### 语义与数据色（功能专用）
- success / warning / danger 只表达状态：运行、订阅到期、连接异常。禁止用于标题、按钮、装饰。
- 延迟分级是产品核心语言，固定四档，全站统一：≤100ms 用 latency-good，≤250ms 用 latency-mid，>250ms 用 latency-bad，超时/失效用 latency-dead（灰）。分级只落在圆点、数值或细条上，面积不超过所在行的 5%。
- 图表色板：主线 chart-line（即 primary），对比线 chart-line-2（灰，虚线），面积填充为 chart-line 的 8% 透明。图表里禁止出现第三种彩色。

## Typography

### 双轨字阶
- **营销轨**（官网）：display-xl/lg/md、headline、card-title、subhead、body-lg、body——舒展、大字距收紧。
- **产品轨**（面板）：p-display、p-headline、p-card-title、p-body、p-body-sm、p-caption、p-micro——整体比营销轨小一档，行高收紧到 1.45，服务信息密度。两轨不许混用：面板里禁止出现 display 级字号，官网正文区禁止用 p-body 冒充 body。
- 数字、延迟、流量、端口、协议名：一律 mono / mono-data，且开启等宽数字（tabular-nums），列表刷新时不许因数字宽度变化而抖动。

### 中文规则（重点，原体系没有这一层）
- 中文标题**禁用大负字距**：letterSpacing 固定 0，最多 -0.01em。负字距表只适用于纯拉丁标题；中英混排标题按中文规则走。
- 中文正文行高在拉丁基础上 +0.1（body 中文实际行高约 1.65），段间距不小于字号的 0.8 倍。
- 字重：中文标题用 600 时优先选有真粗体的字体源；只有伪粗体可用的环境降到 500，不许合成加粗描边。
- 混排：英文单词与数字和中文之间不强制加空格（跟随原文），但数字进入数据语境一律转 mono。
- 字体栈：Inter → "PingFang SC" → "Hiragino Sans GB" → "Microsoft YaHei" → system-ui；禁止用衬线体做中文标题（本系统没有编辑风分支）。

## Layout & Density

- 4px 基准网格；间距只用 spacing tokens。
- **双密度**：舒适密度（官网 + 面板概览）卡片内边距 24/16px、行高 44px；紧凑密度（节点列表、连接表、日志）行高 36px、卡片内边距 12px、字号走 p-body-sm。密度由容器决定，同一页面不许混杂两种行高。
- 内容宽：官网 1280px；面板全宽流式，左右留白 24px，表格区可贴边。
- 区块节奏（官网）：section 间距 96px，靠留白与表面抬升分段，不用分隔线切区块；分隔线只出现在区块内部（表格行、日志行、设置组之间）。

## Motion（本项目的"流畅"要落到参数上）

- 时长三档：instant 80ms（hover 底色）、fast 120ms（开关、徽章、行选中）、base 180ms（面板展开、弹窗、主题切换）、slow 280ms（页面级转场，慎用）。任何 UI 动效不得超过 280ms。
- 缓动：进入用 easing-out，退出用 easing-in，双向用 easing-standard；**禁止弹跳、回弹、过冲**。
- 数据刷新：列表数值变化只过渡颜色/透明度，不做位移动画；测速进度条用线性增长 + 数字滚动（mono），不用弹性动画。
- 主题切换：只过渡 background-color、border-color、color，150ms；图片与截图不参与过渡。
- 全局尊重 prefers-reduced-motion：开启时所有时长归零，只保留瞬时状态变化。

## Elevation & Depth

| 层级 | 处理 | 用途 |
|---|---|---|
| 0 | 无边框无底色 | 正文、标题区、页脚 |
| 1 | surface-1 + 1px hairline | 默认卡片、面板、截图框 |
| 2 | surface-2 / 白底 + hairline-strong | 重点卡片、悬停、选中前态 |
| 3 | surface-3 | 嵌套层、子导航、轨道底（latency-bar、core-switch） |
| 浮层 | 白底 + hairline + 0 8px 30px rgba(10,15,28,0.08) | 弹窗、命令面板、菜单、toast（toast 用 inverse 底） |
| 焦点 | 2px primary-focus 50% 透明描边 | 输入框、按钮聚焦 |

## Components（产品原生部分是本系统的重点）

营销侧组件（button 家族、feature-card、pricing-card、product-screenshot-card、cta-banner、top-nav、footer、changelog-row、pricing-tab）沿用 front matter 定义，规则要点：主按钮一屏一个；按钮 8px 圆角禁胶囊；截图框圆角 xl=14px、内边距 20px。

面板原生组件：
- **sidebar-nav**：surface-1 底的左侧栏；选中项 primary-soft 底 + primary-deep 字 + 6px 圆角，不许用左边框色条表示选中。
- **stat-card**：数值用 mono-data 大号（p-display 的紧凑版可用 24px/600），说明 p-caption，涨跌箭头只许用语义色且必须带数值同行展示。
- **node-row**：紧凑密度 36px 行；结构固定为 状态点 / 名称 / 协议徽章 / latency-bar / 延迟数值（mono 右对齐）/ 操作。选中行 primary-soft 底；失效行整行 ink-tertiary 化。行间 hairline 分隔。
- **latency-bar**：宽 48px 的细条，轨道 surface-3，填充按延迟分级色，填充比例 = min(延迟/300ms, 1)；它只是数值的可视化冗余，不许单独出现（必须与数值同行）。
- **subscription-card**：含名称、用量进度（细条，轨道 surface-3，填充 primary；超 85% 转 warning，过期转 danger 文字）、到期日（mono）、续费/更新按钮（tertiary）。
- **core-switch**：sing-box / mihomo 切换的分段胶囊，轨道 surface-2，选中段白底浮起 + ink 字，未选 ink-subtle；切换触发 motion-base 的底色滑动（只动背景，不动文字）。
- **data-table**：表头 p-micro 大写 + ink-subtle，行 hairline 分隔，数字列 mono 右对齐，行 hover surface-1，排序箭头只用 ink-tertiary。
- **chart-panel**：标题行（p-card-title + 时间范围胶囊组）+ 图区；图区只用图表色板，网格线 hairline，坐标字 p-micro ink-tertiary。
- **toast**：inverse 近黑底白字，右下角浮出，motion-base 进入（fade + 上移 4px），语义图标仅在报错时用 danger 点。

## Icons

- 线性图标，描边 1.5px，圆角端点；尺寸三档 16 / 20 / 24px，不许混用填充式图标。
- 图标颜色继承所在文字色；除状态点外不许给图标单独上 accent 或语义色。

## Do's and Don'ts

### Do
- 先定表面层级和密度，再写组件。
- 数字一律 mono + 等宽；延迟分级色全站统一四档。
- 中文标题字距归零；官网与面板严格分轨。
- 动效查 motion 表，超 280ms 的一律砍掉。
- 新增组件先扩 tokens，命名沿用 xxx-card / xxx-row / xxx-badge 风格。

### Don't
- 不要把本系统做得像 Linear 官网：不许照搬它的文案结构、紫色（任何紫调 accent）、英文优先的排版假设。
- 不要大面积 accent 底色、渐变、发光、玻璃拟态。
- 不要给卡片加投影堆层级（浮层例外）。
- 不要在中文标题上用拉丁负字距表。
- 不要在图表里用色板外的颜色；不要让延迟分级在不同页面有不同阈值。
- 不要弹跳/回弹动画；不要让数据刷新引起布局位移。

## Responsive Behavior

| 断点 | 宽度 | 变化 |
|---|---|---|
| Desktop-XL | 1440px | 默认布局 |
| Desktop | 1280px | 官网卡片 3 列；面板侧栏展开 |
| Tablet | 1024px | 卡片 2 列；面板侧栏收为图标条 |
| Mobile-Lg | 768px | 对比表变手风琴；导航汉堡；节点行隐藏 latency-bar 只留数值 |
| Mobile | 480px | 单列；display-xl 缩至约 34px；面板侧栏变底部导航 |

- 触控目标 ≥40px（紧凑密度行在触屏设备自动升到 44px 行高）。
- 截图与图表保持比例不裁切；数据表移动端横向滚动，不许压缩列宽硬塞。

## Iteration Guide

1. 一次只改一个组件，按 components token 名引用。
2. 新区块先答三问：哪级表面？哪种密度？主角元素是什么？
3. 扩 token 时先改本文件 front matter，再改实现，最后同步 DESIGN-dark.md 的同名 token（结构必须镜像）。
4. 每次改动后跑一遍提示池 F1 审查项自检。
