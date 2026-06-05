<template>
  <Layout>
    <RouterView :key="$route.fullPath" />
  </Layout>
</template>

<script setup>
import Layout from './components/Layout.vue'
</script>
