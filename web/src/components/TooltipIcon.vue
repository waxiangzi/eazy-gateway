<template>
  <span
    class="tooltip-icon"
    tabindex="0"
    role="button"
    :aria-label="text"
    @click="toggle"
    @keydown.enter.prevent="toggle"
    @keydown.space.prevent="toggle"
  >
    ?
    <span
      class="tooltip-text"
      :class="{ show: isVisible }"
      aria-hidden="true"
    >{{ text }}</span>
  </span>
</template>

<script setup>
import { ref } from 'vue'

const props = defineProps({
  text: {
    type: String,
    required: true,
  },
})

const isVisible = ref(false)

function toggle() {
  isVisible.value = !isVisible.value
}
</script>

<style scoped>
.tooltip-icon {
  position: relative;
  display: inline-block;
  width: 1.125rem;
  height: 1.125rem;
  line-height: 1.125rem;
  font-size: 0.6875rem;
  text-align: center;
  border-radius: 50%;
  background: rgba(var(--accent-rgb), 0.2);
  color: var(--accent);
  cursor: help;
  vertical-align: middle;
  margin-left: 0.25rem;
  outline: none;
  border: 1px solid rgba(var(--accent-rgb), 0.3);
}

.tooltip-icon:focus {
  box-shadow: 0 0 0 2px rgba(var(--accent-rgb), 0.4);
}

.tooltip-text {
  visibility: hidden;
  width: max-content;
  max-width: 20rem;
  background: rgba(var(--page-rgb), 0.95);
  color: var(--text-primary);
  text-align: left;
  border-radius: 0.375rem;
  padding: 0.5rem 0.75rem;
  position: absolute;
  z-index: 10;
  bottom: 125%;
  left: 50%;
  transform: translateX(-50%);
  font-size: 0.8125rem;
  line-height: 1.5;
  white-space: normal;
  opacity: 0;
  transition: opacity 0.2s;
  font-weight: normal;
  pointer-events: none;
  border: 1px solid rgba(var(--accent-rgb), 0.2);
  box-shadow: 0 0 12px rgba(var(--accent-rgb), 0.1);
}

.tooltip-icon:hover .tooltip-text,
.tooltip-icon:focus .tooltip-text,
.tooltip-text.show {
  visibility: visible;
  opacity: 1;
}
</style>
