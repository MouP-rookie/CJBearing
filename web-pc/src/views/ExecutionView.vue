<!-- 文件说明：PC 填报页，按产线、任务、班次动态渲染表单并支持多行提交。 -->

<template>
  <div class="page execution-page">
    <div class="page-head"><div><p class="section-label">PC 填报</p><h2 class="page-title">首末检执行中心</h2><p class="page-desc">选择产线和任务后直接填写、确认并提交，提交记录与 PDA 使用同一套日志。</p></div><div class="today-chip"><span>业务日期</span><strong>{{ businessDate }}</strong></div></div>
    <el-alert v-if="auth.allDepartments" type="info" :closable="false" show-icon title="当前为全部部门只读模式，请在顶栏选择具体部门后再进行填报。" class="scope-alert" />

    <div class="execution-grid">
      <aside class="panel selection-panel">
        <div class="panel-heading"><span>01</span><div><strong>选择产线与任务</strong><small>只显示当前账号有权限的产线</small></div></div>
        <el-select v-model="lineId" placeholder="请选择产线" class="line-select" @change="loadForms"><el-option v-for="line in lines" :key="line.id" :label="line.name" :value="line.id" /></el-select>
        <div class="kind-filter"><button v-for="item in kindFilters" :key="item.value" :class="{ active: kindFilter === item.value }" @click="kindFilter = item.value">{{ item.label }}</button></div>
        <div class="task-list"><button v-for="form in pagedForms" :key="form.versionId" class="task-item" :class="{ active: selectedForm?.versionId === form.versionId }" @click="selectForm(form)"><span class="task-kind">{{ kindName(form.kind) }}</span><span><strong>{{ form.name }}</strong><small>版本 v{{ form.versionNo }} · {{ form.code }}</small></span><el-icon><ArrowRight /></el-icon></button><div v-if="lineId && !filteredForms.length" class="empty-small">该产线当前没有此类任务</div><div v-if="!lineId" class="empty-small">先选择一条产线</div></div>
        <AppPagination v-if="filteredForms.length" v-model:page="taskPage" v-model:size="taskSize" :total="filteredForms.length" :page-sizes="[10, 20, 50]" />
      </aside>

      <main class="panel fill-panel">
        <template v-if="selectedForm">
          <div class="fill-header"><div><span class="kind-badge">{{ kindName(selectedForm.kind) }}</span><h3>{{ selectedForm.name }}</h3><p>模板版本 v{{ selectedForm.versionNo }}</p></div><el-tag v-if="selectedForm.kind === 'first_inspection'" type="success">首检完成后日历变绿</el-tag></div>

          <div class="fill-context">
            <el-form-item label="业务日期"><el-date-picker v-model="businessDate" type="date" format="YYYY-MM-DD" value-format="YYYY-MM-DD" style="width:100%" /></el-form-item>
            <el-form-item label="班次"><el-select v-model="shiftItemId" style="width:100%" placeholder="请选择班次"><el-option v-for="shift in shifts" :key="shift.id" :label="`${shift.name} ${shift.startTime}-${shift.endTime}`" :value="shift.id" /></el-select></el-form-item>
            <el-form-item v-if="selectedForm.kind === 'first_inspection'" label="重复首检原因（重复时必填）"><el-input v-model="repeatReason" placeholder="例如：换料、换刀、设备重启" /></el-form-item>
          </div>

          <div class="form-rows">
            <section v-for="(row, rowIndex) in rows" :key="rowIndex" class="entry-row">
              <div v-if="selectedForm.kind === 'registration'" class="row-heading"><span>明细 {{ rowIndex + 1 }}</span><el-button v-if="rows.length > 1" text type="danger" :icon="Delete" @click="rows.splice(rowIndex, 1)">删除明细</el-button></div>
              <div v-for="field in fields" :key="field.key" class="field-block">
                <div class="field-label"><strong>{{ field.label }}</strong><b v-if="field.required">*</b><span v-if="field.unit">{{ field.unit }}</span></div>
                <template v-if="field.type === 'static_text'"><div class="static-value">{{ field.defaultValue || '固定说明文字' }}</div></template>
                <template v-else-if="field.type === 'single_select'"><el-radio-group v-model="row.values[field.key][0]"><el-radio v-for="option in field.options" :key="option.value" :value="option.value">{{ option.label }}</el-radio></el-radio-group></template>
                <template v-else-if="field.type === 'multi_select'"><el-checkbox-group v-model="row.multiValues[field.key]"><el-checkbox v-for="option in field.options" :key="option.value" :value="option.value">{{ option.label }}</el-checkbox></el-checkbox-group></template>
                <template v-else-if="field.type === 'product'"><el-select v-model="row.values[field.key][0]" placeholder="请选择产品" style="width:100%;max-width:420px"><el-option v-for="product in products" :key="product.id" :label="`${product.name}（${product.code}）`" :value="product.name" /></el-select><span v-if="!products.length" class="range-warning">该产线未配置产品，请联系管理员维护</span></template>
                <template v-else><div class="value-list"><el-input v-for="(_, valueIndex) in row.values[field.key]" :key="valueIndex" v-model="row.values[field.key][valueIndex]" :type="field.type === 'number' ? 'number' : 'text'" :placeholder="field.allowMultiple ? `第 ${valueIndex + 1} 个测量值` : '等待填写'" @focus="clearDefault(row, field, valueIndex)" @blur="restoreDefault(row, field, valueIndex)"><template #append v-if="field.unit">{{ field.unit }}</template></el-input><el-button v-if="field.allowMultiple" plain :icon="Plus" @click="row.values[field.key].push(defaultValue(field, row))">增加测量值</el-button><span v-if="outOfRange(field, row.values[field.key][0])" class="range-warning">超出配置范围，仍可提交</span></div></template>
              </div>
            </section>
            <el-button v-if="selectedForm.kind === 'registration'" plain :icon="Plus" @click="rows.push(makeRow())">增加一条登记明细</el-button>
          </div>

          <footer class="fill-actions"><div class="submit-note">提交后日期和班次将锁定，修改会保留审计记录。</div><el-button type="primary" size="large" :disabled="auth.allDepartments" :loading="submitting" @click="submit">确认提交</el-button></footer>
        </template>
        <div v-else class="empty-panel"><div><strong>选择左侧任务开始填报</strong><p>PC 和 PDA 的填报记录会进入同一份日志。</p></div></div>
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { ArrowRight, Delete, Plus } from '@element-plus/icons-vue'
import { api } from '@/api'
import { useAuthStore } from '@/stores/auth'
import AppPagination from '@/components/AppPagination.vue'
import { formatDateTime, type FieldDefinition } from '@shared'

type FormRow = { values: Record<string, string[]>; multiValues: Record<string, string[]> }
const auth = useAuthStore()
const lines = ref<any[]>([])
const forms = ref<any[]>([])
const shifts = ref<any[]>([])
const lineId = ref<number>()
const shiftItemId = ref<number>()
const selectedForm = ref<any>()
const fields = ref<FieldDefinition[]>([])
const rows = ref<FormRow[]>([])
const products = ref<any[]>([])
const kindFilter = ref('all')
const taskPage = ref(1)
const taskSize = ref(10)
const businessDate = ref(new Date().toLocaleDateString('sv-SE'))
const repeatReason = ref('')
const submitting = ref(false)
const kindFilters = [{ label: '全部', value: 'all' }, { label: '首检', value: 'first_inspection' }, { label: '末检', value: 'final_inspection' }, { label: '报表', value: 'report' }, { label: '登记表', value: 'registration' }]
const selectedLineName = computed(() => lines.value.find(line => line.id === lineId.value)?.name || '')
const filteredForms = computed(() => kindFilter.value === 'all' ? forms.value : forms.value.filter(form => form.kind === kindFilter.value))
const pagedForms = computed(() => filteredForms.value.slice((taskPage.value - 1) * taskSize.value, taskPage.value * taskSize.value))
watch([kindFilter, filteredForms], () => { taskPage.value = 1 })
function kindName(kind: string) { return { first_inspection: '首检', final_inspection: '末检', report: '报表', registration: '登记表' }[kind] || kind }
function defaultValue(field: FieldDefinition, _row?: FormRow) { const now = new Date(); const shift = shifts.value.find(item => item.id === shiftItemId.value); const map: Record<string, string> = { '{{current_date}}': businessDate.value, '{{current_time}}': formatDateTime(now), '{{current_shift}}': shift?.name || '', '{{user_name}}': auth.user?.name || '', '{{user_account}}': auth.user?.account || '', '{{line_name}}': selectedLineName.value, '{{line_product}}': products.value.map(p => p.name).join('、'), '{{form_name}}': selectedForm.value?.name || '', '{{stage}}': kindName(selectedForm.value?.kind || '') }; let value = field.defaultValue || ''; for (const [key, replacement] of Object.entries(map)) value = value.replaceAll(key, replacement); return value }
function makeRow(): FormRow { const row: FormRow = { values: {}, multiValues: {} }; for (const field of fields.value) { row.values[field.key] = [defaultValue(field, row)]; row.multiValues[field.key] = field.type === 'multi_select' && field.defaultValue ? field.defaultValue.split(',').filter(Boolean) : [] } return row }
function resetRows() { rows.value = [makeRow()] }
function clearDefault(row: FormRow, field: FieldDefinition, index: number) { if (row.values[field.key][index] === defaultValue(field, row)) row.values[field.key][index] = '' }
function restoreDefault(row: FormRow, field: FieldDefinition, index: number) { if (!row.values[field.key][index]) row.values[field.key][index] = defaultValue(field, row) }
function outOfRange(field: FieldDefinition, value: string) { const number = Number(value); if (!value || Number.isNaN(number)) return false; return (field.min !== undefined && number < field.min) || (field.max !== undefined && number > field.max) }
// 只把末级产线作为填报入口，父产线用于组织结构。
async function loadLines() { const { data } = await api.get('/execution/lines'); lines.value = data.items; const leaves = lines.value.filter(line => !lines.value.some(child => child.parentId === line.id)).sort((a, b) => String(a.code || '').localeCompare(String(b.code || ''), 'zh-CN', { numeric: true, sensitivity: 'base' }) || a.id - b.id); if (leaves.length) { lineId.value = leaves[0].id; await loadForms() } }
// 切换产线后同时刷新可用表单和产品。
async function loadForms() { forms.value = []; selectedForm.value = undefined; products.value = []; if (!lineId.value) return; const { data } = await api.get('/execution/forms', { params: { lineId: lineId.value } }); forms.value = data.items; try { const prodRes = await api.get('/execution/products', { params: { lineId: lineId.value } }); products.value = prodRes.data.items || [] } catch {} }
async function loadShifts() { const { data } = await api.get('/execution/shifts'); shifts.value = data.items || []; shiftItemId.value = data.current?.id || shifts.value[0]?.id }
// 选择任务时按提交版本 Schema 动态生成字段和默认值。
async function selectForm(form: any) { const { data } = await api.get(`/execution/forms/${form.versionId}`); const schema = typeof data.version.schema === 'string' ? JSON.parse(data.version.schema) : data.version.schema; fields.value = (schema.fields || []).sort((a: any, b: any) => (a.sortOrder || 0) - (b.sortOrder || 0)); selectedForm.value = { ...form, versionNo: data.version.versionNo, schema }; resetRows() }
// 提交前把页面值铺平为后端字段值数组，每个明细独立保存。
async function submit() {
  if (!selectedForm.value || !lineId.value || !shiftItemId.value) { ElMessage.warning('请先选择产线、任务和班次'); return }
  if (selectedForm.value.kind === 'first_inspection') { const { data } = await api.get('/execution/repeat-check', { params: { lineId: lineId.value, businessDate: businessDate.value, shiftPlanItemId: shiftItemId.value } }); if (data.required && !repeatReason.value.trim()) { ElMessage.warning('重复首检必须填写原因'); return } }
  for (const row of rows.value) { for (const field of fields.value) { if (field.required && field.type === 'multi_select' && !(row.multiValues[field.key]?.length)) { ElMessage.warning('请填写必填项'); return }
      if (field.required && field.type !== 'static_text' && field.type !== 'multi_select' && !row.values[field.key]?.some(value => value !== '')) { ElMessage.warning(`请填写“${field.label}”`); return } } }
  const payloadRows = rows.value.map((row, rowIndex) => ({ rowNo: rowIndex + 1, values: fields.value.flatMap(field => { if (field.type === 'multi_select') return (row.multiValues[field.key] || []).map((value, index) => ({ fieldKey: field.key, valueType: field.type, valueText: value, repeatIndex: index, source: 'manual' })); return (row.values[field.key] || []).map((value, index) => ({ fieldKey: field.key, valueType: field.type, valueText: value, valueNumber: field.type === 'number' && value !== '' && !Number.isNaN(Number(value)) ? Number(value) : undefined, repeatIndex: index, source: value === defaultValue(field, row) ? 'default' : 'manual' })) }) }))
  submitting.value = true
  try { await api.post('/execution/submissions', { templateId: selectedForm.value.templateId, versionId: selectedForm.value.versionId, lineId: lineId.value, businessDate: businessDate.value, shiftPlanItemId: shiftItemId.value, repeatReason: repeatReason.value, source: 'pc', clientTime: new Date().toISOString(), rows: payloadRows }); ElMessage.success('填报已提交'); repeatReason.value = ''; resetRows() } catch (error: any) { ElMessage.error(error.message || '提交失败') } finally { submitting.value = false }
}
onMounted(async () => { await Promise.all([loadLines(), loadShifts()]) })
</script>

<style scoped>.scope-alert{margin:-12px 0 18px}</style>

<style scoped>
.execution-page .page-head { display: flex; align-items: flex-end; justify-content: space-between; gap: 20px; }
.page-desc { max-width: 700px; margin: -8px 0 22px; color: var(--bench-ink-soft); font-size: 13px; line-height: 1.7; }
.today-chip { min-width: 150px; padding: 11px 13px; border-left: 3px solid var(--bench-amber); background: #fffdf8; }
.today-chip span, .today-chip strong { display: block; }
.today-chip span { color: var(--bench-ink-soft); font-size: 11px; }
.today-chip strong { margin-top: 4px; font-variant-numeric: tabular-nums; }
.execution-grid { display: grid; grid-template-columns: 300px minmax(0, 1fr); gap: 18px; align-items: start; }
.selection-panel { position: sticky; top: 18px; padding-bottom: 12px; }
.panel-heading { display: flex; gap: 10px; padding: 17px; border-bottom: 1px solid var(--bench-line); }
.panel-heading > span { color: var(--bench-steel); font-size: 12px; font-weight: 700; }
.panel-heading strong, .panel-heading small { display: block; }
.panel-heading strong { font-size: 15px; }
.panel-heading small { margin-top: 4px; color: var(--bench-ink-soft); font-size: 11px; }
.selection-panel .line-select { margin: 15px 14px 10px; width: calc(100% - 28px); }
.kind-filter { display: flex; flex-wrap: wrap; gap: 5px; padding: 0 14px 12px; }
.kind-filter button { padding: 5px 8px; border: 1px solid var(--bench-line); color: var(--bench-ink-soft); background: transparent; font-size: 11px; cursor: pointer; }
.kind-filter button.active { border-color: var(--bench-steel); color: #fff; background: var(--bench-steel); }
.task-list { display: grid; gap: 5px; padding: 0 9px; }
.task-item { display: grid; grid-template-columns: auto 1fr auto; align-items: center; gap: 9px; width: 100%; padding: 11px 8px; border: 1px solid transparent; border-bottom-color: var(--bench-line); color: var(--bench-ink); background: transparent; text-align: left; cursor: pointer; }
.task-item:hover { background: #f5f8f6; }
.task-item.active { border-color: var(--bench-steel); background: #eaf1f3; }
.task-kind { padding: 4px 5px; color: var(--bench-steel-deep); border: 1px solid #b5ccd2; font-size: 10px; font-weight: 700; }
.task-item strong, .task-item small { display: block; }
.task-item strong { font-size: 13px; }
.task-item small { margin-top: 4px; color: var(--bench-ink-soft); font-size: 10px; }
.empty-small { padding: 20px 10px; color: var(--bench-ink-soft); text-align: center; font-size: 12px; }
.fill-panel { min-width: 0; padding: 20px; }
.fill-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; padding-bottom: 17px; border-bottom: 2px solid var(--bench-ink); }
.kind-badge { display: inline-block; padding: 4px 6px; color: var(--bench-steel-deep); background: #eaf1f3; font-size: 11px; font-weight: 700; }
.fill-header h3 { margin: 8px 0 5px; font-size: 24px; letter-spacing: -.03em; }
.fill-header p { margin: 0; color: var(--bench-ink-soft); font-size: 12px; }
.fill-context { display: grid; grid-template-columns: 180px 220px 1fr; gap: 14px; margin: 20px 0; padding: 14px; border: 1px solid var(--bench-line); background: #f7f4ed; }
.fill-context :deep(.el-form-item) { margin-bottom: 0; }
.form-rows { display: grid; gap: 14px; }
.entry-row { padding: 17px; border: 1px solid var(--bench-line); background: #fffefa; }
.row-heading { display: flex; align-items: center; justify-content: space-between; margin: -4px 0 14px; padding-bottom: 10px; border-bottom: 1px solid var(--bench-line); color: var(--bench-steel); font-size: 12px; font-weight: 700; }
.field-block { padding: 13px 0; border-bottom: 1px solid var(--bench-line); }
.field-block:last-child { border-bottom: 0; }
.field-label { display: flex; align-items: center; gap: 5px; margin-bottom: 8px; }
.field-label strong { font-size: 14px; }
.field-label b { color: var(--bench-amber); }
.field-label span { color: var(--bench-ink-soft); font-size: 12px; }
.static-value { padding: 9px 11px; color: var(--bench-ink-soft); background: #f0ede5; font-size: 13px; }
.value-list { display: grid; gap: 8px; max-width: 620px; }
.range-warning { color: var(--bench-amber); font-size: 11px; }
.fill-actions { display: flex; align-items: center; justify-content: space-between; gap: 20px; margin-top: 22px; padding-top: 17px; border-top: 1px solid var(--bench-line); }
.submit-note { color: var(--bench-ink-soft); font-size: 12px; }
@media (max-width: 1100px) { .execution-grid { grid-template-columns: 1fr; } .selection-panel { position: static; } .fill-context { grid-template-columns: 1fr 1fr; } }
@media (max-width: 800px) { .execution-page .page-head, .fill-actions { align-items: flex-start; flex-direction: column; } .fill-context { grid-template-columns: 1fr; } }
</style>
