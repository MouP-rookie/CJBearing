<template>
  <el-button @click="handleClick"><slot /></el-button>
</template>

<script setup lang="ts">
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const emit = defineEmits<{ click: [] }>()

function handleClick() {
  if (auth.allDepartments) {
    auth.departmentPickerOpen = true
    return
  }
  emit('click')
}
</script>
