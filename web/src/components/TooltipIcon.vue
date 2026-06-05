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
  background: #94a3b8;
  color: #fff;
  cursor: help;
  vertical-align: middle;
  margin-left: 0.25rem;
  outline: none;
}

.tooltip-icon:focus {
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.5);
}

.tooltip-text {
  visibility: hidden;
  width: max-content;
  max-width: 20rem;
  background: #0f172a;
  color: #fff;
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
}

.tooltip-icon:hover .tooltip-text,
.tooltip-icon:focus .tooltip-text,
.tooltip-text.show {
  visibility: visible;
  opacity: 1;
}
</style>
