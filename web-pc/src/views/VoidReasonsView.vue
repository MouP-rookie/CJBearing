<template>
  <div class="page reason-page">
    <div class="page-head">
      <div>
        <p class="section-label">系统配置</p>
        <h2 class="page-title">废除原因</h2>
        <p class="page-desc">维护 PC 与 PDA 废除明细时可快捷选择的原因。“其他”由系统固定提供，操作人需手填原因；这里的修改只影响后续操作，不会改写历史记录。</p>
      </div>
      <el-button type="primary" size="large" :loading="saving" @click="save">保存配置</el-button>
    </div>

    <section class="panel reason-panel">
      <div class="panel-heading">
        <div><strong>快捷选项</strong><small>{{ items.length }} / 50 项</small></div>
        <el-button plain :disabled="items.length >= 50" @click="addItem"><el-icon><Plus /></el-icon>新增原因</el-button>
      </div>

      <div v-if="items.length" class="reason-list">
        <div v-for="(item, index) in items" :key="item.key" class="reason-row">
          <span class="sort-number">{{ String(index + 1).padStart(2, '0') }}</span>
          <el-input v-model="item.value" maxlength="50" show-word-limit placeholder="请输入废除原因" />
          <el-button link type="danger" @click="removeItem(index)">删除</el-button>
        </div>
      </div>
      <div v-else class="empty-reasons">当前没有快捷选项，废除时仍可使用“其他”手填原因。</div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { api } from '@/api'

type EditableReason = { key: number; value: string }

const items = ref<EditableReason[]>([])
const saving = ref(false)
let nextKey = 1

function addItem() {
  if (items.value.length >= 50) return
  items.value.push({ key: nextKey++, value: '' })
}

function removeItem(index: number) {
  items.value.splice(index, 1)
}

async function load() {
  try {
    const { data } = await api.get('/settings/void-reasons')
    items.value = (data.items || []).map((value: string) => ({ key: nextKey++, value }))
  } catch (error: any) {
    ElMessage.error(error.message || '读取废除原因失败')
  }
}

function normalizedItems() {
  const values = items.value.map(item => item.value.trim())
  if (values.some(value => !value)) throw new Error('废除原因不能为空')
  if (values.some(value => value === '其他')) throw new Error('“其他”为系统保留选项，无需配置')
  if (values.some(value => Array.from(value).length > 50)) throw new Error('单项废除原因不能超过 50 字')
  if (new Set(values).size !== values.length) throw new Error('废除原因不能重复')
  return values
}

async function save() {
  saving.value = true
  try {
    const values = normalizedItems()
    const { data } = await api.put('/admin/settings/void-reasons', { items: values })
    items.value = (data.items || []).map((value: string) => ({ key: nextKey++, value }))
    ElMessage.success('废除原因已保存')
  } catch (error: any) {
    ElMessage.error(error.message || '保存废除原因失败')
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.reason-page { max-width: 920px; }
.page-head { display: flex; align-items: flex-end; justify-content: space-between; gap: 24px; }
.page-desc { max-width: 720px; margin: -8px 0 22px; color: var(--bench-ink-soft); font-size: 13px; line-height: 1.7; }
.reason-panel { padding: 22px; }
.panel-heading { display: flex; align-items: center; justify-content: space-between; gap: 18px; margin-bottom: 18px; }
.panel-heading strong, .panel-heading small { display: block; }
.panel-heading strong { font-size: 16px; }
.panel-heading small { margin-top: 5px; color: var(--bench-ink-soft); font-size: 12px; }
.reason-list { display: grid; gap: 10px; }
.reason-row { display: grid; grid-template-columns: 42px minmax(0, 1fr) 56px; align-items: center; gap: 10px; padding: 10px; border: 1px solid var(--bench-line); background: #fffefa; }
.sort-number { color: var(--bench-steel); font-size: 12px; font-weight: 750; letter-spacing: .08em; }
.empty-reasons { padding: 32px 18px; border: 1px dashed var(--bench-line); color: var(--bench-ink-soft); background: #fffefa; text-align: center; font-size: 13px; }
@media (max-width: 760px) {
  .page-head, .panel-heading { align-items: flex-start; flex-direction: column; }
}
</style>
