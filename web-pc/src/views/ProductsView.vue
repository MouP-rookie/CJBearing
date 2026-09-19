<template>
  <div class="page products-page">
    <div class="page-head">
      <div><p class="section-label">产品档案</p><h2 class="page-title">产品维护</h2><p class="page-desc">维护正在生产的产品清单并关联可生产的产线，点检作业时按当前产线过滤选择。</p></div>
      <el-button type="primary" size="large" @click="openCreate()">新增产品</el-button>
    </div>
    <el-table :data="items" border row-key="id">
      <el-table-column prop="code" label="产品编码" width="180" />
      <el-table-column prop="name" label="产品名称" min-width="200" />
      <el-table-column label="关联产线" min-width="260">
        <template #default="{ row }">
          <el-tag v-for="line in row.lines || []" :key="line.id" size="small" class="line-tag">{{ line.name }}</el-tag>
          <span v-if="!(row.lines || []).length" class="empty-hint">未关联产线</span>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100"><template #default="{ row }"><el-tag :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '启用' : '停用' }}</el-tag></template></el-table-column>
      <el-table-column label="操作" width="220">
        <template #default="{ row }">
          <el-button link @click="openEdit(row)">编辑</el-button>
          <el-button link @click="toggle(row)">{{ row.enabled ? '停用' : '启用' }}</el-button>
          <el-button link type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
    <el-dialog v-model="dialog" :title="form.id ? '编辑产品' : '新增产品'" width="520px">
      <el-form label-position="top">
        <el-form-item label="产品编码"><el-input v-model="form.code" :disabled="!!form.id" placeholder="例如：6203-2RS" /></el-form-item>
        <el-form-item label="产品名称"><el-input v-model="form.name" placeholder="请输入现场使用的产品名称" /></el-form-item>
        <el-form-item label="关联产线">
          <el-tree-select v-model="form.lineIds" :data="lineTree" multiple show-checkbox collapse-tags collapse-tags-tooltip :max-collapse-tags="4" node-key="id" :props="{ label: 'name', children: 'children' }" check-on-click-node placeholder="选择可生产该产品的产线" style="width:100%" />
          <p class="tree-hint">勾选父产线时，其全部子产线会一并关联该产品。</p>
        </el-form-item>
        <el-form-item label="状态"><el-switch v-model="form.enabled" active-text="启用" inactive-text="停用" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialog=false">取消</el-button><el-button type="primary" :loading="saving" @click="save">确认保存</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '@/api'

const items = ref<any[]>([])
const lineOptions = ref<any[]>([])
const lineTree = computed(() => {
  const nodes = new Map<number, any>()
  for (const line of lineOptions.value) nodes.set(line.id, { ...line, children: [] })
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
const form = reactive({ id: 0, code: '', name: '', enabled: true, lineIds: [] as number[] })
async function load() { const { data } = await api.get('/admin/products'); items.value = data.items }
async function loadLines() { const { data } = await api.get('/admin/product-line-options'); lineOptions.value = data.items }
function openCreate() { form.id = 0; form.code = ''; form.name = ''; form.enabled = true; form.lineIds = []; dialog.value = true }
function openEdit(row: any) { form.id = row.id; form.code = row.code; form.name = row.name; form.enabled = row.enabled; form.lineIds = (row.lines || []).map((line: any) => line.id); dialog.value = true }
async function save() {
  if (!form.code.trim() || !form.name.trim()) { ElMessage.warning('请填写产品编码和名称'); return }
  saving.value = true
  try {
    if (form.id) await api.patch(`/admin/products/${form.id}`, { name: form.name.trim(), enabled: form.enabled, lineIds: form.lineIds })
    else await api.post('/admin/products', { code: form.code.trim(), name: form.name.trim(), enabled: form.enabled, lineIds: form.lineIds })
    dialog.value = false
    await load()
    ElMessage.success('产品已保存')
  } catch (error: any) { ElMessage.error(error.message || '保存产品失败') } finally { saving.value = false }
}
async function toggle(row: any) { try { await api.patch(`/admin/products/${row.id}`, { enabled: !row.enabled }); await load() } catch (error: any) { ElMessage.error(error.message || '操作失败') } }
async function remove(row: any) {
  try {
    await ElMessageBox.confirm(`确定删除产品“${row.name}”？删除后原编码可以重新使用，历史填报记录中的产品名称仍会保留。`, '删除确认')
    await api.delete(`/admin/products/${row.id}`)
    await load()
    ElMessage.success('产品已删除')
  } catch (error: any) { if (error !== 'cancel' && error !== 'close') ElMessage.error(error.message || '删除产品失败') }
}
onMounted(() => { load(); loadLines() })
</script>

<style scoped>
.page-head { display: flex; align-items: flex-end; justify-content: space-between; gap: 20px; }
.page-desc { max-width: 660px; margin: -8px 0 22px; color: var(--bench-ink-soft); font-size: 13px; line-height: 1.7; }
.line-tag { margin: 2px 6px 2px 0; }
.empty-hint { color: var(--bench-ink-soft); font-size: 12px; }
.tree-hint { margin: 6px 0 0; color: var(--bench-ink-soft); font-size: 11px; }
@media (max-width: 800px) { .page-head { align-items: flex-start; flex-direction: column; } }
</style>
