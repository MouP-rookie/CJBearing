<!-- 文件说明：产线配置页，维护两级产线并支持 Excel 导入。 -->

<template>
  <div class="page lines-page">
    <div class="page-head">
      <div><p class="section-label">产线结构</p><h2 class="page-title">产线配置</h2><p class="page-desc">产线最多两级，任务、报表和登记表挂在末级子产线上。</p></div>
      <div class="page-actions">
        <el-button size="large" :disabled="!auth.can('line.manage')" @click="openImport"><el-icon><UploadFilled /></el-icon>批量导入</el-button>
        <el-button type="primary" size="large" :disabled="!auth.can('line.manage')" @click="openCreate()">新增产线</el-button>
      </div>
    </div>
    <el-alert v-if="auth.allDepartments" class="scope-note" type="info" :closable="false" show-icon title="可直接维护各部门的产线。新增或批量导入时，请在弹窗中选择所属部门。" />
    <el-table :data="lineTree" border row-key="id" :tree-props="{ children: 'children' }">
      <el-table-column prop="code" label="产线编码" width="180" />
      <el-table-column prop="name" label="产线名称" min-width="220" />
      <el-table-column label="部门" width="150"><template #default="{ row }">{{ row.department?.name || '-' }}</template></el-table-column>
      <el-table-column label="层级" width="110"><template #default="{ row }"><span class="level-chip" :class="{ child: row.level === 2 }">{{ row.level === 1 ? '产线' : '子产线' }}</span></template></el-table-column>
      <el-table-column label="所属产线" width="220"><template #default="{ row }">{{ parentName(row.parentId) }}</template></el-table-column>
      <el-table-column label="状态" width="100"><template #default="{ row }"><el-tag :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '启用' : '停用' }}</el-tag></template></el-table-column>
      <el-table-column label="操作" width="310"><template #default="{ row }">
        <el-button link :disabled="!canWriteDepartment(row.departmentId)" @click="openEdit(row)">编辑</el-button>
        <el-button v-if="row.level === 1" link :disabled="!canWriteDepartment(row.departmentId)" @click="openCreate(row)">增加子产线</el-button>
        <el-button link :disabled="!canWriteDepartment(row.departmentId)" @click="toggle(row)">{{ row.enabled ? '停用' : '启用' }}</el-button>
        <el-button link type="danger" :disabled="!canWriteDepartment(row.departmentId)" @click="remove(row)">删除</el-button>
      </template></el-table-column>
    </el-table>
    <AppPagination v-model:page="page" v-model:size="size" :total="total" @change="load" />
    <el-dialog v-model="dialog" :title="form.id ? '编辑产线' : '新增产线'" width="460px">
      <el-form label-position="top">
        <el-form-item v-if="auth.isSuperAdmin" label="所属部门" required>
          <el-select v-model="form.departmentId" :disabled="!!form.id || saving" placeholder="请选择所属部门" style="width:100%" @change="changeFormDepartment">
            <el-option v-for="department in enabledDepartments" :key="department.id" :label="department.name" :value="department.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="产线编码"><el-input v-model="form.code" :disabled="!!form.id" placeholder="例如：L001" /></el-form-item>
        <el-form-item label="产线名称"><el-input v-model="form.name" placeholder="请输入现场使用的名称" /></el-form-item>
        <el-form-item v-if="!form.id" label="上级产线">
          <el-select v-model="form.parentId" clearable :disabled="!form.departmentId || parentLoading || saving" :loading="parentLoading" placeholder="不选择则创建为根产线" style="width:100%">
            <el-option v-for="line in rootLines" :key="line.id" :label="`${line.name}（${line.code}）`" :value="line.id" />
          </el-select>
        </el-form-item>
        <p v-if="!form.id && form.parentId" class="parent-note">将创建在“{{ parentName(form.parentId) }}”下面；产线最多两级。</p>
      </el-form>
      <template #footer><el-button @click="dialog=false">取消</el-button><el-button type="primary" :loading="saving" :disabled="!form.departmentId || parentLoading" @click="save">确认保存</el-button></template>
    </el-dialog>

    <el-dialog v-model="importDialog" title="Excel 批量导入产线" width="680px" @closed="resetImport">
      <el-form v-if="auth.isSuperAdmin" label-position="top">
        <el-form-item label="导入部门" required>
          <el-select v-model="importDepartmentId" :disabled="importing" placeholder="请选择产线所属部门" style="width:100%" @change="importErrors = []">
            <el-option v-for="department in enabledDepartments" :key="department.id" :label="department.name" :value="department.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <el-alert type="info" :closable="false" show-icon title="产线将导入所选部门。上级产线编码留空表示根产线；一个 Excel 中可同时包含根产线和两级子产线。" />
      <div class="import-format">Excel 列：<strong>产线编码</strong>、<strong>产线名称</strong>、<strong>上级产线编码</strong>、<strong>状态</strong>。状态可填“启用/停用”，留空默认启用。</div>
      <el-upload ref="uploadRef" drag :auto-upload="false" :limit="1" accept=".xlsx" :on-change="onImportFileChange" :on-remove="onImportFileRemove">
        <el-icon class="upload-icon"><UploadFilled /></el-icon>
        <div class="upload-copy">拖拽 Excel 文件到此处，或点击选择文件</div>
        <template #tip><div class="el-upload__tip">仅支持 .xlsx，文件最大 10MB</div></template>
      </el-upload>
      <div class="template-link"><el-button link type="primary" @click="downloadTemplate">下载导入模板</el-button></div>
      <el-table v-if="importErrors.length" :data="importErrors" border size="small" class="error-table">
        <el-table-column prop="row" label="Excel 行" width="90" />
        <el-table-column prop="code" label="产线编码" width="150" />
        <el-table-column prop="name" label="产线名称" width="160" />
        <el-table-column prop="message" label="未导入原因" />
      </el-table>
      <template #footer><el-button @click="importDialog=false">取消</el-button><el-button type="primary" :loading="importing" :disabled="!importDepartmentId" @click="submitImport">开始导入</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { UploadFilled } from '@element-plus/icons-vue'
import type { Line } from '@shared'
import { api } from '@/api'
import { useAuthStore } from '@/stores/auth'
import AppPagination from '@/components/AppPagination.vue'

const auth = useAuthStore()
const lines = ref<Line[]>([])
const page = ref(1)
const size = ref(20)
const total = ref(0)
const enabledDepartments = computed(() => auth.departments.filter(department => department.enabled))
const parentLines = ref<Line[]>([])
const parentLoading = ref(false)
let parentRequestID = 0
const rootLines = computed(() => parentLines.value.filter(line => line.level === 1 && line.departmentId === form.departmentId))
const lineTree = computed(() => {
  const nodes = new Map<number, Line & { children: any[] }>()
  for (const line of lines.value) nodes.set(line.id, { ...line, children: [] })
  const roots: any[] = []
  for (const node of nodes.values()) {
    if (node.parentId && nodes.has(node.parentId)) nodes.get(node.parentId)!.children.push(node)
    else roots.push(node)
  }
  const sortTree = (items: any[]) => { items.sort((a, b) => String(a.code || '').localeCompare(String(b.code || ''), 'zh-CN', { numeric: true, sensitivity: 'base' }) || a.id - b.id); for (const item of items) sortTree(item.children) }
  sortTree(roots)
  return roots
})
const dialog = ref(false)
const saving = ref(false)
const importDialog = ref(false)
const importDepartmentId = ref<number>()
const importFile = ref<File | null>(null)
const importErrors = ref<any[]>([])
const importing = ref(false)
const uploadRef = ref<any>()
const form = reactive({ id: 0, code: '', name: '', departmentId: undefined as number | undefined, parentId: undefined as number | undefined })

function initialDepartmentId() {
  if (!auth.isSuperAdmin) return auth.user?.departmentId
  return auth.selectedDepartmentId || (enabledDepartments.value.length === 1 ? enabledDepartments.value[0].id : undefined)
}
function canWriteDepartment(departmentId?: number): boolean {
  if (!departmentId || !auth.can('line.manage')) return false
  if (auth.isSuperAdmin) return enabledDepartments.value.some(department => department.id === departmentId)
  return auth.user?.departmentId === departmentId && Boolean(auth.user.department?.enabled)
}
// 写请求只使用目标产线的部门，不改变顶部的查询范围。
function departmentConfig(departmentId: number) {
  return { headers: { 'X-Department-ID': String(departmentId) } }
}
async function load(): Promise<void> {
  try {
    const { data } = await api.get('/admin/lines', { params: { page: page.value, size: size.value } })
    if (!data.items?.length && page.value > 1) { page.value -= 1; return load() }
    lines.value = data.items || []
    total.value = data.total || 0
  } catch (error: any) { ElMessage.error(error.message || '加载产线失败') }
}
function parentName(id?: number) { return id ? [...parentLines.value, ...lines.value].find(item => item.id === id)?.name || '-' : '-' }
async function loadParentLines() {
  const requestID = ++parentRequestID
  const departmentId = form.departmentId
  parentLines.value = []
  parentLoading.value = false
  if (!departmentId) return
  parentLoading.value = true
  try {
    const items: Line[] = []
    let currentPage = 1
    while (true) {
      const { data } = await api.get('/admin/lines', { ...departmentConfig(departmentId), params: { page: currentPage, size: 100 } })
      if (requestID !== parentRequestID) return
      const batch: Line[] = data.items || []
      items.push(...batch.filter(line => line.level === 1))
      if (!batch.length || currentPage * 100 >= Number(data.total || 0)) break
      currentPage += 1
    }
    parentLines.value = items
  } catch (error: any) {
    if (requestID === parentRequestID) ElMessage.error(error.message || '加载上级产线失败')
  } finally {
    if (requestID === parentRequestID) parentLoading.value = false
  }
}
function changeFormDepartment() { form.parentId = undefined; void loadParentLines() }
function openCreate(parent?: Line) {
  if (parent && (parent.level !== 1 || !canWriteDepartment(parent.departmentId))) return
  form.id = 0
  form.code = ''
  form.name = ''
  form.departmentId = parent?.departmentId || initialDepartmentId()
  form.parentId = parent?.id
  dialog.value = true
  void loadParentLines()
}
function openEdit(row: Line) {
  if (!canWriteDepartment(row.departmentId)) return
  ++parentRequestID
  parentLoading.value = false
  parentLines.value = []
  Object.assign(form, { id: row.id, code: row.code, name: row.name, departmentId: row.departmentId, parentId: row.parentId || undefined })
  dialog.value = true
}
async function save() {
  const departmentId = form.departmentId
  if (!departmentId || !canWriteDepartment(departmentId)) { ElMessage.warning('请选择启用的所属部门'); return }
  if (!form.code.trim() || !form.name.trim()) { ElMessage.warning('请输入产线编码和名称'); return }
  saving.value = true
  try {
    const config = departmentConfig(departmentId)
    if (form.id) await api.patch('/admin/lines/' + form.id, { name: form.name.trim() }, config)
    else await api.post('/admin/lines', { code: form.code.trim(), name: form.name.trim(), parentId: form.parentId || undefined }, config)
    dialog.value = false
    await load()
    ElMessage.success('产线已保存')
  } catch (error: any) { ElMessage.error(error.message || '保存产线失败') } finally { saving.value = false }
}
async function toggle(row: Line) {
  if (!canWriteDepartment(row.departmentId)) return
  try {
    await api.patch('/admin/lines/' + row.id, { enabled: !row.enabled }, departmentConfig(row.departmentId))
    await load()
  } catch (error: any) { ElMessage.error(error.message || '操作失败') }
}
async function remove(row: Line) {
  if (!canWriteDepartment(row.departmentId)) return
  try {
    await ElMessageBox.confirm('确定删除产线“' + row.name + '”？删除后原编码可以重新使用。', '删除确认')
    await api.delete('/admin/lines/' + row.id, departmentConfig(row.departmentId))
    await load()
    ElMessage.success('产线已删除')
  } catch (error: any) { if (error !== 'cancel' && error !== 'close') ElMessage.error(error.message || '删除产线失败') }
}
function openImport() {
  importDepartmentId.value = initialDepartmentId()
  resetImport()
  importDialog.value = true
}
function onImportFileChange(file: any) { importFile.value = file.raw || null; importErrors.value = [] }
function onImportFileRemove() { importFile.value = null; importErrors.value = [] }
function resetImport() { importFile.value = null; importErrors.value = []; uploadRef.value?.clearFiles() }
async function downloadTemplate() {
  try {
    const response = await api.get('/admin/lines/import-template', { responseType: 'blob' })
    const url = URL.createObjectURL(response.data)
    const link = document.createElement('a')
    link.href = url
    link.download = '产线批量导入模板.xlsx'
    link.click()
    URL.revokeObjectURL(url)
  } catch (error: any) {
    ElMessage.error(error.message || '下载模板失败')
  }
}
async function submitImport() {
  const departmentId = importDepartmentId.value
  if (!departmentId || !canWriteDepartment(departmentId)) { ElMessage.warning('请选择启用的导入部门'); return }
  if (!importFile.value) { ElMessage.warning('请先选择 Excel 文件'); return }
  importing.value = true
  try {
    const data = new FormData()
    data.append('file', importFile.value)
    const response = await api.post('/admin/lines/import', data, departmentConfig(departmentId))
    importErrors.value = response.data.errors || []
    if (response.data.created) ElMessage.success('成功导入 ' + response.data.created + ' 条产线')
    if (importErrors.value.length) { ElMessage.warning('有 ' + importErrors.value.length + ' 行未导入，请查看下方原因'); await load(); return }
    await load()
    importDialog.value = false
  } catch (error: any) {
    ElMessage.error(error.message || '批量导入失败')
  } finally {
    importing.value = false
  }
}
onMounted(load)
onUnmounted(() => { ++parentRequestID })
</script>

<style scoped>
.scope-note { margin-bottom: 16px; }
.page-head { display: flex; align-items: flex-end; justify-content: space-between; gap: 20px; }
.page-desc { max-width: 660px; margin: -8px 0 22px; color: var(--bench-ink-soft); font-size: 13px; line-height: 1.7; }
.page-actions { display: flex; align-items: center; gap: 10px; }
.level-chip { display: inline-block; padding: 4px 7px; border: 1px solid #b5ccd2; color: var(--bench-steel-deep); font-size: 12px; }
.level-chip.child { border-color: #a6c5b5; color: #2e7356; }
.parent-note { margin: 0; color: var(--bench-ink-soft); font-size: 12px; }
.import-format { margin: 16px 0 12px; color: var(--bench-ink-soft); font-size: 13px; line-height: 1.7; }
.upload-icon { margin-bottom: 8px; color: var(--bench-steel); font-size: 30px; }
.upload-copy { color: var(--bench-ink); font-size: 13px; }
.template-link { margin-top: 6px; text-align: right; }
.error-table { margin-top: 16px; }
@media (max-width: 800px) { .page-head { align-items: flex-start; flex-direction: column; } .page-actions { width: 100%; } }
</style>
