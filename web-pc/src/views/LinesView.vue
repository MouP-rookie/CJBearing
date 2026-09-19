<template>
  <div class="page lines-page">
    <div class="page-head"><div><p class="section-label">产线结构</p><h2 class="page-title">产线配置</h2><p class="page-desc">产线最多两级，任务、报表和登记表挂在末级子产线上。</p></div><el-button type="primary" size="large" @click="openCreate()">新增产线</el-button></div>
    <el-table :data="lineTree" border row-key="id" default-expand-all :tree-props="{ children: 'children' }">
      <el-table-column prop="code" label="产线编码" width="180" />
      <el-table-column prop="name" label="产线名称" min-width="220" />
      <el-table-column label="层级" width="110"><template #default="{ row }"><span class="level-chip" :class="{ child: row.level === 2 }">{{ row.level === 1 ? '产线' : '子产线' }}</span></template></el-table-column>
      <el-table-column label="所属产线" width="220"><template #default="{ row }">{{ parentName(row.parentId) }}</template></el-table-column>
      <el-table-column label="状态" width="100"><template #default="{ row }"><el-tag :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '启用' : '停用' }}</el-tag></template></el-table-column>
      <el-table-column label="操作" width="250"><template #default="{ row }"><el-button link @click="openCreate(row)">增加子产线</el-button><el-button link @click="toggle(row)">{{ row.enabled ? '停用' : '启用' }}</el-button><el-button link type="danger" @click="remove(row)">删除</el-button></template></el-table-column>
    </el-table>
    <el-dialog v-model="dialog" :title="form.id ? '编辑产线' : '新增产线'" width="460px"><el-form label-position="top"><el-form-item label="产线编码"><el-input v-model="form.code" :disabled="!!form.id" placeholder="例如：L001" /></el-form-item><el-form-item label="产线名称"><el-input v-model="form.name" placeholder="请输入现场使用的名称" /></el-form-item><p v-if="form.parentId" class="parent-note">将创建在“{{ parentName(form.parentId) }}”下面</p></el-form><template #footer><el-button @click="dialog=false">取消</el-button><el-button type="primary" :loading="saving" @click="save">确认保存</el-button></template></el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '@/api'

const lines = ref<any[]>([])
const lineTree = computed(() => {
  const nodes = new Map<number, any>()
  for (const line of lines.value) nodes.set(line.id, { ...line, children: [] })
  const roots: any[] = []
  for (const node of nodes.values()) {
    if (node.parentId && nodes.has(node.parentId)) nodes.get(node.parentId).children.push(node)
    else roots.push(node)
  }
  const sortTree = (items: any[]) => { items.sort((a, b) => a.id - b.id); for (const item of items) sortTree(item.children) }
  sortTree(roots)
  return roots
})
const dialog = ref(false)
const saving = ref(false)
const form = reactive({ id: 0, code: '', name: '', parentId: undefined as number | undefined })
async function load() { const { data } = await api.get('/admin/lines'); lines.value = data.items }
function parentName(id?: number) { return id ? lines.value.find(item => item.id === id)?.name || '-' : '-' }
function openCreate(parent?: any) { form.id = 0; form.code = ''; form.name = ''; form.parentId = parent?.level === 1 ? parent.id : undefined; dialog.value = true }
async function save() {
  saving.value = true
  try {
    if (form.id) await api.patch(`/admin/lines/${form.id}`, { name: form.name })
    else await api.post('/admin/lines', { code: form.code.trim(), name: form.name.trim(), parentId: form.parentId })
    dialog.value = false
    await load()
    ElMessage.success('产线已保存')
  } catch (error: any) { ElMessage.error(error.message || '保存产线失败') } finally { saving.value = false }
}
async function toggle(row: any) { try { await api.patch(`/admin/lines/${row.id}`, { enabled: !row.enabled }); await load() } catch (error: any) { ElMessage.error(error.message || '操作失败') } }
async function remove(row: any) {
  try {
    await ElMessageBox.confirm(`确定删除产线“${row.name}”？删除后原编码可以重新使用。`, '删除确认')
    await api.delete(`/admin/lines/${row.id}`)
    await load()
    ElMessage.success('产线已删除')
  } catch (error: any) { if (error !== 'cancel') ElMessage.error(error.message || '删除产线失败') }
}
onMounted(load)
</script>

<style scoped>
.page-head { display: flex; align-items: flex-end; justify-content: space-between; gap: 20px; }
.page-desc { max-width: 660px; margin: -8px 0 22px; color: var(--bench-ink-soft); font-size: 13px; line-height: 1.7; }
.level-chip { display: inline-block; padding: 4px 7px; border: 1px solid #b5ccd2; color: var(--bench-steel-deep); font-size: 12px; }
.level-chip.child { border-color: #a6c5b5; color: #2e7356; }
.parent-note { margin: 0; color: var(--bench-ink-soft); font-size: 12px; }
@media (max-width: 800px) { .page-head { align-items: flex-start; flex-direction: column; } }
</style>
