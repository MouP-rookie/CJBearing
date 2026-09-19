<!-- 文件说明：产线分配页，将已发布表单版本分配给产线并处理父子级联。 -->

<template>
  <div class="page assignment-page">
    <div class="page-head"><div><p class="section-label">表单分发</p><h2 class="page-title">任务与报表分配</h2><p class="page-desc">把已发布的表单版本分配到产线。分配父产线时，其全部子产线会自动使用同一版本；共享模板可以为不同产线指定不同版本。</p></div><el-button type="primary" size="large" :disabled="auth.allDepartments || !selectedTemplate || !publishedVersions.length" :loading="saving" @click="save">保存分配</el-button></div>

    <div class="assignment-grid">
      <aside class="panel template-panel">
        <div class="panel-heading"><span>01</span><div><strong>选择模板</strong><small>首末检、报表和登记表都在这里分配</small></div></div>
        <el-segmented v-model="kind" :options="kindOptions" @change="changeKind" />
        <div class="template-list"><button v-for="template in templates" :key="template.id" class="template-item" :class="{ active: selectedTemplate?.id === template.id }" @click="selectTemplate(template)"><span class="template-kind">{{ kindName(template.kind) }}</span><span><strong>{{ template.name }}</strong><small>{{ template.code }}</small></span><el-icon><ArrowRight /></el-icon></button></div>
        <AppPagination v-model:page="templatePage" v-model:size="templateSize" :total="templateTotal" :page-sizes="[10, 20, 50]" @change="loadTemplates" />
      </aside>

      <main class="panel line-panel">
        <template v-if="selectedTemplate">
          <div class="assignment-header"><div><span class="kind-badge">{{ kindName(selectedTemplate.kind) }}</span><h3>{{ selectedTemplate.name }}</h3><p>选择每条产线使用的已发布版本</p></div><el-button plain :disabled="auth.allDepartments || !publishedVersions.length" @click="assignLatest">全部使用最新版本</el-button></div>
          <div v-if="!publishedVersions.length" class="publish-hint">
            <span v-if="latestDraft">该模板目前只有草稿版本 v{{ latestDraft.versionNo }}，发布后才能分配到产线。</span>
            <span v-else>该模板还没有任何版本，请先到「任务与表单」新建并编辑模板。</span>
            <el-button v-if="latestDraft" size="small" type="warning" :disabled="auth.allDepartments" :loading="publishing" @click="publishLatestDraft">发布草稿 v{{ latestDraft.versionNo }}</el-button>
          </div>
          <el-table :data="pagedLineRows" border>
            <el-table-column label="产线" min-width="250"><template #default="{ row }"><span class="line-name" :class="{ child: row.parentId }">{{ row.parentId ? '└ ' : '' }}{{ row.lineName }}</span><el-tag v-if="row.hasChildren" size="small" type="info" class="cascade-tag">父产线 · 同步子线</el-tag></template></el-table-column>
            <el-table-column label="当前版本" width="120"><template #default="{ row }"><span v-if="row.originalVersionId" class="mono">v{{ row.originalVersionNo }}</span><span v-else class="muted">未分配</span></template></el-table-column>
            <el-table-column label="分配版本" width="260"><template #default="{ row }"><el-select v-model="row.versionId" style="width:100%" :disabled="auth.allDepartments" @change="onVersionChange(row)"><el-option label="不分配" :value="0" /><el-option v-for="version in publishedVersions" :key="version.id" :label="`v${version.versionNo}（${formatDateTime(version.publishedAt || version.createdAt)}）`" :value="version.id" /></el-select></template></el-table-column>
            <el-table-column label="调整结果" width="150"><template #default="{ row }"><el-tag v-if="row.versionId === row.originalVersionId" type="info">无变化</el-tag><el-tag v-else-if="row.versionId === 0" type="danger">将取消分配</el-tag><el-tag v-else type="warning">将切换版本</el-tag></template></el-table-column>
          </el-table>
          <AppPagination v-model:page="linePage" v-model:size="lineSize" :total="lineRows.length" :page-sizes="[10, 20, 50, 100]" />
        </template>
        <div v-else class="empty-panel"><div><strong>从左侧选择一个模板</strong><p>然后为每条产线指定使用版本。</p></div></div>
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { ArrowRight } from '@element-plus/icons-vue'
import { api } from '@/api'
import { useAuthStore } from '@/stores/auth'
import AppPagination from '@/components/AppPagination.vue'

const auth = useAuthStore()

type LineRow = { lineId: number; lineName: string; parentId?: number; hasChildren: boolean; versionId: number; originalVersionId: number; originalVersionNo: number }
const kind = ref('all')
const kindOptions = [{ label: '全部', value: 'all' }, { label: '首检', value: 'first_inspection' }, { label: '末检', value: 'final_inspection' }, { label: '报表', value: 'report' }, { label: '登记表', value: 'registration' }]
const templates = ref<any[]>([])
const templatePage = ref(1)
const templateSize = ref(20)
const templateTotal = ref(0)
const selectedTemplate = ref<any>()
const versions = ref<any[]>([])
const lineRows = ref<LineRow[]>([])
const linePage = ref(1)
const lineSize = ref(20)
const saving = ref(false)
const publishing = ref(false)
const publishedVersions = computed(() => versions.value.filter(version => version.status === 'published'))
const draftVersions = computed(() => versions.value.filter(version => version.status === 'draft'))
const latestDraft = computed(() => draftVersions.value[0])
const pagedLineRows = computed(() => lineRows.value.slice((linePage.value - 1) * lineSize.value, linePage.value * lineSize.value))
function kindName(value: string) { return kindOptions.find(item => item.value === value)?.label || value }
function pad(value: number) { return String(value).padStart(2, '0') }
function formatDateTime(value: string) { const date = new Date(value); if (Number.isNaN(date.getTime())) return '-'; return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}` }
function changeKind() { templatePage.value = 1; loadTemplates() }
async function loadTemplates() { const { data } = await api.get('/admin/form-templates', { params: { ...(kind.value === 'all' ? {} : { kind: kind.value }), page: templatePage.value, size: templateSize.value } }); if (!data.items?.length && templatePage.value > 1) { templatePage.value -= 1; return loadTemplates() }; templates.value = data.items; templateTotal.value = data.total || 0; if (selectedTemplate.value && !templates.value.some(item => item.id === selectedTemplate.value.id)) selectedTemplate.value = undefined }
async function selectTemplate(template: any) {
  selectedTemplate.value = template
  const detail = await api.get(`/admin/form-templates/${template.id}`)
  versions.value = detail.data.versions || []
  const assignment = await api.get(`/admin/form-templates/${template.id}/assignments`)
  const lines = await api.get('/execution/lines')
  lineRows.value = buildLineRows(lines.data.items || [], assignment.data.items || [])
  linePage.value = 1
}
function buildLineRows(lines: any[], assignments: any[]): LineRow[] {
  const ids = new Set<number>(lines.map(line => line.id))
  const compareCode = (a: any, b: any) => String(a.code || '').localeCompare(String(b.code || ''), 'zh-CN', { numeric: true, sensitivity: 'base' }) || a.id - b.id
  const childrenOf = (id: number) => lines.filter(line => line.parentId === id).sort(compareCode)
  const roots = lines.filter(line => !line.parentId || !ids.has(line.parentId)).sort(compareCode)
  const ordered: { line: any; hasChildren: boolean }[] = []
  for (const root of roots) {
    const children = childrenOf(root.id)
    ordered.push({ line: root, hasChildren: children.length > 0 })
    for (const child of children) ordered.push({ line: child, hasChildren: false })
  }
  const assignmentByLine = new Map<number, any>(assignments.map(item => [item.lineId, item]))
  return ordered.map(({ line, hasChildren }) => {
    const current = assignmentByLine.get(line.id)
    return { lineId: line.id, lineName: line.name, parentId: line.parentId, hasChildren, versionId: current?.versionId || 0, originalVersionId: current?.versionId || 0, originalVersionNo: current?.versionNo || 0 }
  })
}
// 父产线版本变化时仅联动子产线显示值，最终以保存请求为准。
function onVersionChange(row: LineRow) {
  if (row.parentId) return
  for (const child of lineRows.value) {
    if (child.parentId === row.lineId) child.versionId = row.versionId
  }
}
// 一键把所有产线切到最新已发布版本，便于批量换版。
function assignLatest() { const latest = publishedVersions.value[0]; if (!latest) { ElMessage.warning('没有已发布版本，请先发布'); return } for (const row of lineRows.value) row.versionId = latest.id }
// 页面允许先发布最新草稿，再为产线选择版本。
async function publishLatestDraft() {
  if (!selectedTemplate.value || !latestDraft.value) return
  publishing.value = true
  try {
    await api.post(`/admin/form-versions/${latestDraft.value.id}/publish`, { lineIds: [] })
    ElMessage.success(`草稿 v${latestDraft.value.versionNo} 已发布，请为产线选择该版本后保存`)
    await selectTemplate(selectedTemplate.value)
  } catch (error: any) {
    ElMessage.error(error.message || '发布失败')
  } finally {
    publishing.value = false
  }
}
// 仅提交有版本的产线，后端会校验版本属于模板且已发布。
async function save() { if (!selectedTemplate.value) return; if (!publishedVersions.value.length) { ElMessage.warning('该模板还没有已发布版本，无法分配产线'); return } saving.value = true; try { const items = lineRows.value.filter(row => row.versionId > 0).map(row => ({ lineId: row.lineId, versionId: row.versionId })); await api.put(`/admin/form-templates/${selectedTemplate.value.id}/assignments`, { items }); ElMessage.success('产线分配已保存'); await selectTemplate(selectedTemplate.value) } catch (error: any) { ElMessage.error(error.message || '保存分配失败') } finally { saving.value = false } }
onMounted(loadTemplates)
</script>

<style scoped>
.page-head { display: flex; align-items: flex-end; justify-content: space-between; gap: 20px; }
.page-desc { max-width: 760px; margin: -8px 0 22px; color: var(--bench-ink-soft); font-size: 13px; line-height: 1.7; }
.assignment-grid { display: grid; grid-template-columns: 300px minmax(0, 1fr); gap: 18px; align-items: start; }
.template-panel { position: sticky; top: 18px; padding-bottom: 12px; }
.panel-heading { display: flex; gap: 10px; padding: 17px; border-bottom: 1px solid var(--bench-line); }
.panel-heading > span { color: var(--bench-steel); font-size: 12px; font-weight: 700; }
.panel-heading strong, .panel-heading small { display: block; }
.panel-heading strong { font-size: 15px; }
.panel-heading small { margin-top: 4px; color: var(--bench-ink-soft); font-size: 11px; }
.template-panel .el-segmented { margin: 13px 14px 10px; }
.template-list { display: grid; gap: 5px; padding: 0 9px; }
.template-item { display: grid; grid-template-columns: auto 1fr auto; align-items: center; gap: 9px; width: 100%; padding: 11px 8px; border: 1px solid transparent; border-bottom-color: var(--bench-line); color: var(--bench-ink); background: transparent; text-align: left; cursor: pointer; }
.template-item:hover { background: #f5f8f6; }
.template-item.active { border-color: var(--bench-steel); background: #eaf1f3; }
.template-kind { padding: 4px 5px; color: var(--bench-steel-deep); border: 1px solid #b5ccd2; font-size: 10px; font-weight: 700; }
.template-item strong, .template-item small { display: block; }
.template-item strong { font-size: 13px; }
.template-item small { margin-top: 4px; color: var(--bench-ink-soft); font-size: 10px; }
.line-panel { min-width: 0; padding: 20px; }
.assignment-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; margin-bottom: 17px; padding-bottom: 16px; border-bottom: 2px solid var(--bench-ink); }
.kind-badge { display: inline-block; padding: 4px 6px; color: var(--bench-steel-deep); background: #eaf1f3; font-size: 11px; font-weight: 700; }
.assignment-header h3 { margin: 8px 0 5px; font-size: 24px; letter-spacing: -.03em; }
.assignment-header p { margin: 0; color: var(--bench-ink-soft); font-size: 12px; }
.mono { font-variant-numeric: tabular-nums; }
.muted { color: var(--bench-ink-soft); font-size: 12px; }
.line-name.child { padding-left: 14px; color: var(--bench-ink-soft); }
.cascade-tag { margin-left: 8px; }
.publish-hint { display: flex; align-items: center; justify-content: space-between; gap: 14px; margin-bottom: 14px; padding: 11px 13px; border: 1px solid #e0b877; border-left: 3px solid var(--bench-amber); background: #fdf6e7; color: #7a5312; font-size: 12px; }
@media (max-width: 1000px) { .assignment-grid { grid-template-columns: 1fr; } .template-panel { position: static; } }
@media (max-width: 800px) { .page-head, .assignment-header { align-items: flex-start; flex-direction: column; } }
</style>
