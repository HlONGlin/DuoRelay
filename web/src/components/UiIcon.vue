<template>
  <svg :width="size" :height="size" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <path v-for="(path, index) in paths[name] || paths.grid" :key="index" :d="path" />
  </svg>
</template>

<script setup lang="ts">
withDefaults(defineProps<{ name: string; size?: 16 | 20 | 24 }>(), { size: 20 })
const paths: Record<string, string[]> = {
  grid: ['M3 3h7v7H3z M14 3h7v7h-7z M3 14h7v7H3z M14 14h7v7h-7z'],
  nodes: ['M12 9V3 M12 15v6 M9 12H3 M15 12h6 M9 9l-4-4 M15 15l4 4 M9 15l-4 4 M15 9l4-4', 'M12 9a3 3 0 1 0 0 6 3 3 0 0 0 0-6'],
  link: ['M10 13a5 5 0 0 0 7 0l3-3a5 5 0 0 0-7-7l-2 2 M14 11a5 5 0 0 0-7 0l-3 3a5 5 0 0 0 7 7l2-2'],
  terminal: ['M3 4h18v16H3z M7 9l3 3-3 3 M13 15h4'],
  settings: ['M4 7h16 M4 17h16 M8 4v6 M16 14v6'],
  search: ['M10.5 3a7.5 7.5 0 1 0 0 15 7.5 7.5 0 0 0 0-15 M16 16l5 5'],
  moon: ['M21 13a9 9 0 1 1-10-10 7 7 0 0 0 10 10'],
  sun: ['M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8 M12 2v2 M12 20v2 M2 12h2 M20 12h2 M5 5l1.5 1.5 M17.5 17.5L19 19 M5 19l1.5-1.5 M17.5 6.5L19 5'],
  chevron: ['M14 6l-6 6 6 6'],
  menu: ['M4 6h16 M4 12h16 M4 18h16'],
  check: ['M5 12l4 4L19 6'],
  arrow: ['M5 12h14 M14 7l5 5-5 5'],
  plus: ['M12 5v14 M5 12h14'],
  edit: ['M15 4l5 5 M4 15l12-12 5 5L9 20l-6 1z'],
  trash: ['M3 6h18 M9 6V3h6v3 M5 6l1 15h12l1-15 M10 10v7 M14 10v7'],
  refresh: ['M20 7V3l-4 4 M4 17v4l4-4 M4 10a8 8 0 0 1 13-6l3 3 M20 14a8 8 0 0 1-13 6l-3-3'],
  upload: ['M12 16V3 M7 8l5-5 5 5 M4 16v5h16v-5'],
  empty: ['M4 5h16v15H4z M4 14h4l2 3h4l2-3h4'],
  clock: ['M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18 M12 7v5l3 2']
}
</script>
