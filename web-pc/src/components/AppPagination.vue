<!-- 文件说明：PC 通用分页组件，统一表格分页交互。 -->

<template>
  <div class="pagination-bar">
    <el-pagination
      background
      layout="total, sizes, prev, pager, next, jumper"
      :current-page="page"
      :page-size="size"
      :page-sizes="pageSizes"
      :total="total"
      @update:current-page="changePage"
      @update:page-size="changeSize"
    />
  </div>
</template>

<script setup lang="ts">
const props = withDefaults(defineProps<{
  page: number
  size: number
  total: number
  pageSizes?: number[]
}>(), {
  pageSizes: () => [10, 20, 50, 100],
})

const emit = defineEmits<{
  'update:page': [value: number]
  'update:size': [value: number]
  change: []
}>()

function changePage(value: number) {
  emit('update:page', value)
  emit('change')
}

function changeSize(value: number) {
  emit('update:size', value)
  emit('update:page', 1)
  emit('change')
}

void props
</script>

<style scoped>
.pagination-bar { display: flex; justify-content: flex-end; margin-top: 16px; overflow-x: auto; }
</style>
