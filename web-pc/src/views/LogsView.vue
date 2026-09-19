<template>
  <div class="page log-page">
    <div class="page-head">
      <div>
        <p class="section-label">填报追溯</p>
        <h2 class="page-title">日志与导出</h2>
        <p class="page-desc">日历中绿色表示当天有填报、白色表示当天没有填报。点击日历中的日期，可快速查看当天记录；展开记录可查看具体项目、测量值、填报人和操作时间。</p>
      </div>
      <el-button type="success" size="large" :disabled="!lineId" :loading="exporting" @click="exportData">导出 Excel</el-button>
    </div>

    <section class="calendar-panel panel">
      <div class="calendar-head">
        <div>
          <p class="section-label">填报日历</p>
          <h3>{{ calendarMonthLabel }}</h3>
        </div>
        <div class="calendar-summary">
          <span><i class="legend-chip filled"></i>有填报</span>
          <span><i class="legend-chip empty"></i>无填报</span>
          <span v-if="selectedDate" class="selected-date">已选：{{ selectedDate }}</span>
        </div>
      </div>
      <el-calendar v-model="calendarDate" class="inspection-calendar">
        <template #date-cell="{ data }">
          <div
            class="calendar-cell"
            :class="{ 'has-log': calendarCount(data.day) > 0, 'other-month': data.type !== 'current-month', 'is-selected': data.isSelected }"
            @click="selectCalendarDate(data)"
          >
            <span class="calendar-day">{{ Number(data.day.slice(8, 10)) }}</span>
            <span v-if="calendarCount(data.day) > 0" class="calendar-count">{{ calendarCount(data.day) }} 条</span>
            <span v-else class="calendar-empty">未填报</span>
          </div>
        </template>
      </el-calendar>
    </section>

    <div class="toolbar log-toolbar">
      <el-select v-model="lineId" placeholder="选择产线" style="width:230px" @change="runQuery">
        <el-option v-for="line in lineOptions" :key="line.id" :label="line.label" :value="line.id" />
      </el-select>
      <el-select v-model="templateId" placeholder="全部任务/表单" clearable style="width:230px" @change="runQuery">
        <el-option v-for="template in templates" :key="template.id" :label="template.name" :value="template.id" />
      </el-select>
      <el-date-picker
        v-model="range"
        type="daterange"
        value-format="YYYY-MM-DD"
        format="YYYY-MM-DD"
        range-separator="至"
        start-placeholder="开始日期"
        end-placeholder="结束日期"
        @change="onRangeChange"
      />
      <el-input v-model="filler" placeholder="填报人或工号" clearable style="width:180px" @keyup.enter="runQuery" />
      <el-button type="primary" @click="runQuery">查询</el-button>
    </div>

    <el-table v-loading="loading" :data="logs" border row-key="id" empty-text="暂无符合条件的填报记录" :row-class-name="logRowClass">
      <el-table-column type="expand">
        <template #default="{ row: submission }">
          <div class="detail-wrap">
            <el-table :data="submission.details || []" size="small" border>
              <el-table-column prop="rowNo" label="明细" width="70" />
              <el-table-column prop="fieldLabel" label="项目" min-width="180">
                <template #default="{ row: detail }">
                  <strong>{{ detail.fieldLabel }}</strong>
                  <small v-if="detail.unit" class="unit">{{ detail.unit }}</small>
                </template>
              </el-table-column>
              <el-table-column label="测量值" min-width="180">
                <template #default="{ row: detail }">
                  <div class="value-cell" :class="{ 'is-out-of-range': detail.status === 'active' && detail.outOfRange }">
                    <span :class="{ voided: detail.status === 'voided' }">{{ formatValue(detail) }}</span>
                    <small v-if="detail.status === 'active' && detail.outOfRange" class="range-warning">{{ rangeWarning(detail) }}</small>
                  </div>
                </template>
              </el-table-column>
              <el-table-column label="明细状态" min-width="150">
                <template #default="{ row: detail }">
                  <el-tag v-if="detail.status === 'voided'" type="danger" size="small">已废除</el-tag>
                  <el-tag v-else type="success" size="small">有效</el-tag>
                  <div v-if="detail.voidReason" class="state-note">废除原因：{{ detail.voidReason }}</div>
                  <div v-if="detail.restoreReason" class="state-note">恢复原因：{{ detail.restoreReason }}</div>
                </template>
              </el-table-column>
              <el-table-column prop="operatorName" label="填报人" width="120" />
              <el-table-column prop="operatorAccount" label="工号" width="120" />
              <el-table-column label="填报时间" width="190">
                <template #default="{ row: detail }">{{ formatDateTime(detail.operatedAt) }}</template>
              </el-table-column>
              <el-table-column label="来源" width="100">
                <template #default="{ row: detail }">
                  <el-tag size="small" :type="detail.source === 'default' ? 'info' : 'success'">{{ sourceName(detail.source) }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="操作" width="110" fixed="right">
                <template #default="{ row: detail, $index }">
                  <el-button v-if="isFirstDetail(submission, detail, $index) && detail.status === 'active' && canVoidDetail(detail)" link type="danger" @click="openRowVoidDialog(submission, detail)">废除</el-button>
                  <el-button v-else-if="isFirstDetail(submission, detail, $index) && detail.status === 'voided' && canRestoreDetail(detail)" link @click="restoreRow(submission, detail)">恢复</el-button>
                  <span v-else-if="isFirstDetail(submission, detail, $index)" class="state-note">无权操作</span>
                </template>
              </el-table-column>
            </el-table>
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="businessDate" label="业务日期" width="130">
        <template #default="{ row }">{{ formatDate(row.businessDate) }}</template>
      </el-table-column>
      <el-table-column label="状态" width="140">
        <template #default="{ row }">
          <div class="status-tags">
            <el-tag v-if="hasOutOfRange(row)" type="danger" effect="dark" size="small">上下限异常</el-tag>
            <el-tag v-if="row.status === 'voided'" type="danger" size="small">已废除</el-tag>
            <el-tag v-else-if="row.superseded" type="info" size="small">已被覆盖</el-tag>
            <el-tag v-else type="success" size="small">有效</el-tag>
          </div>
          <div v-if="row.voidReason" class="state-note">{{ row.voidReason }}</div>
        </template>
      </el-table-column>
      <el-table-column prop="shiftNameSnapshot" label="班次" width="90" />
      <el-table-column prop="templateName" label="任务/表单" min-width="220" />
      <el-table-column prop="lineName" label="产线" min-width="180" />
      <el-table-column prop="fillerName" label="填报人" width="120" />
      <el-table-column prop="fillerAccount" label="工号" width="120" />
      <el-table-column label="提交时间" width="190">
        <template #default="{ row }">{{ formatDateTime(row.submittedAt) }}</template>
      </el-table-column>
      <el-table-column prop="repeatReason" label="重复首检原因" min-width="150" />
      <el-table-column v-if="canVoid || canRestore" label="操作" width="120" fixed="right">
        <template #default="{ row }">
          <el-button v-if="canVoidSubmission(row) && row.status !== 'voided'" link type="danger" @click="openSubmissionVoidDialog(row)">整条废除</el-button>
          <el-button v-else-if="canRestoreSubmission(row) && row.status === 'voided'" link @click="restoreRecord(row)">整条恢复</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pager">
      <el-pagination
        v-model:current-page="page"
        :page-size="size"
        layout="total, prev, pager, next"
        :total="total"
        @current-change="loadLogs"
      />
    </div>

    <el-dialog v-model="voidDialogVisible" title="选择废除原因" width="480px" :close-on-click-modal="false">
      <p class="dialog-tip">废除后该明细不再计入导出与统计。配置项可直接选择；选项不完整时请选择“其他”并手填原因。</p>
      <el-radio-group v-model="voidReason" class="reason-options">
        <el-radio v-for="reason in voidReasons" :key="reason" :value="reason">{{ reason }}</el-radio>
        <el-radio value="其他">其他</el-radio>
      </el-radio-group>
      <el-input
        v-if="voidReason === '其他'"
        v-model="otherVoidReason"
        type="textarea"
        :rows="3"
        maxlength="500"
        show-word-limit
        placeholder="请填写废除原因（必填）"
      />
      <template #footer>
        <el-button @click="voidDialogVisible = false">取消</el-button>
        <el-button type="danger" :loading="voidSubmitting" :disabled="!canSubmitVoidReason" @click="submitVoid">确认废除</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '@/api'
import { useAuthStore } from '@/stores/auth'

type CalendarCell = { day: string; type: string; isSelected: boolean }

const auth = useAuthStore()

const lines = ref<any[]>([])
const lineOptions = ref<any[]>([])
const templates = ref<any[]>([])
const logs = ref<any[]>([])
const lineId = ref<number>()
const templateId = ref<number>()
const range = ref<[string, string] | null>(null)
const filler = ref('')
const loading = ref(false)
const exporting = ref(false)
const calendarDate = ref(new Date())
const calendarCounts = ref<Record<string, number>>({})
const selectedDate = ref('')
const page = ref(1)
const size = ref(20)
const total = ref(0)
const voidDialogVisible = ref(false)
const voidTarget = ref<{ kind: 'submission' | 'row'; id: number; label: string } | null>(null)
const voidReasons = ref<string[]>([])
const voidReason = ref('')
const otherVoidReason = ref('')
const voidSubmitting = ref(false)

const exportLabel = computed(() => {
  const template = templates.value.find(item => item.id === templateId.value)
  if (!template) return '记录导出'
  return (template.kind === 'report' || template.kind === 'registration') ? '报表登记表' : '首末检日志'
})
const canVoid = computed(() => auth.can('submission.void_own') || auth.can('submission.void_scope'))
const canRestore = computed(() => auth.can('submission.edit_own') || auth.can('submission.edit_scope'))
const calendarMonthLabel = computed(() => `${calendarDate.value.getFullYear()} 年 ${calendarDate.value.getMonth() + 1} 月`)
const canSubmitVoidReason = computed(() => voidReason.value === '其他' ? Boolean(otherVoidReason.value.trim()) : Boolean(voidReason.value))

function pad(value: number) { return String(value).padStart(2, '0') }
function formatDate(value: string) { const date = new Date(value); return Number.isNaN(date.getTime()) ? value : `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}` }
function formatDateTime(value: string) { if (!value) return '-'; const date = new Date(value); return Number.isNaN(date.getTime()) ? value : `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}` }
function formatValue(detail: any) { const value = detail.valueNumber !== null && detail.valueNumber !== undefined ? detail.valueNumber : detail.valueText; if (value === '' || value === null || value === undefined) return '-'; return detail.unit ? `${value} ${detail.unit}` : String(value) }
function hasOutOfRange(submission: any) { return (submission.details || []).some((detail: any) => detail.status === 'active' && detail.outOfRange) }
function rangeWarning(detail: any) {
  const limits: string[] = []
  if (detail.min !== null && detail.min !== undefined) limits.push(`下限 ${detail.min}`)
  if (detail.max !== null && detail.max !== undefined) limits.push(`上限 ${detail.max}`)
  const suffix = detail.unit ? ` ${detail.unit}` : ''
  return limits.length ? `超出上下限（${limits.join('，')}${suffix}）` : '超出上下限'
}
function logRowClass({ row }: { row: any }) { return hasOutOfRange(row) ? 'out-of-range-row' : '' }
function sourceName(source: string) { return { manual: '人工填写', default: '默认值', instrument: '仪表数据' }[source] || source }
function calendarCount(day: string) { return calendarCounts.value[day] || 0 }
function dateKey(value: string) { return String(value || '').slice(0, 10) }

function monthRange(date: Date) {
  const start = new Date(date.getFullYear(), date.getMonth(), 1)
  const end = new Date(date.getFullYear(), date.getMonth() + 1, 0)
  return [`${start.getFullYear()}-${pad(start.getMonth() + 1)}-${pad(start.getDate())}`, `${end.getFullYear()}-${pad(end.getMonth() + 1)}-${pad(end.getDate())}`] as [string, string]
}

async function loadLines() {
  const { data } = await api.get('/execution/lines')
  lines.value = data.items
  const ids = new Set(lines.value.map(line => line.id))
  const compareCode = (a: any, b: any) => String(a.code || '').localeCompare(String(b.code || ''), 'zh-CN', { numeric: true, sensitivity: 'base' }) || a.id - b.id
  const childrenOf = (id: number) => lines.value.filter(line => line.parentId === id).sort(compareCode)
  const roots = lines.value.filter(line => !line.parentId || !ids.has(line.parentId)).sort(compareCode)
  const options: any[] = []
  for (const root of roots) {
    const rootDepartment = auth.allDepartments && root.department?.name ? `[${root.department.name}] ` : ''
    options.push({ id: root.id, label: `${rootDepartment}${root.name}` })
    for (const child of childrenOf(root.id)) options.push({ id: child.id, label: `${rootDepartment}└ ${child.name}` })
  }
  lineOptions.value = options
  if (options.length) {
    lineId.value = options[0].id
    page.value = 1
    await Promise.all([loadLogs(), loadCalendar()])
  }
}

async function loadTemplates() {
  const { data } = await api.get('/admin/form-templates', { params: { page: 1, size: 200 } })
  templates.value = data.items
}

async function loadCalendar() {
  if (!lineId.value) {
    calendarCounts.value = {}
    return
  }
  const [dateFrom, dateTo] = monthRange(calendarDate.value)
  try {
    const { data } = await api.get('/admin/logs/calendar', {
      params: {
        lineId: lineId.value,
        templateId: templateId.value || undefined,
        dateFrom,
        dateTo,
        filler: filler.value || undefined
      }
    })
    const next: Record<string, number> = {}
    for (const item of data.items || []) {
      next[dateKey(item.businessDate)] = Number(item.count) || 0
    }
    calendarCounts.value = next
  } catch (error: any) {
    ElMessage.error(error.message || '查询填报日历失败')
  }
}

async function loadLogs() {
  if (!lineId.value) return
  loading.value = true
  try {
    const { data } = await api.get('/admin/logs', {
      params: {
        lineId: lineId.value,
        templateId: templateId.value || undefined,
        dateFrom: range.value?.[0],
        dateTo: range.value?.[1],
        filler: filler.value || undefined,
        page: page.value,
        size: size.value
      }
    })
    logs.value = data.items
    total.value = data.total
  } catch (error: any) {
    ElMessage.error(error.message || '查询日志失败')
  } finally {
    loading.value = false
  }
}

async function runQuery() {
  page.value = 1
  await Promise.all([loadLogs(), loadCalendar()])
}

function onRangeChange(value: [string, string] | null) {
  selectedDate.value = value && value[0] === value[1] ? value[0] : ''
  page.value = 1
  loadLogs()
}

function selectCalendarDate(data: CalendarCell) {
  if (data.type !== 'current-month') {
    calendarDate.value = new Date(`${data.day}T00:00:00`)
  }
  selectedDate.value = data.day
  range.value = [data.day, data.day]
  page.value = 1
  loadLogs()
}

async function exportData() {
  if (!range.value) { ElMessage.warning('请先选择日期范围'); return }
  exporting.value = true
  try {
    const response = await api.post('/admin/exports', {
      lineId: lineId.value,
      dateFrom: range.value[0],
      dateTo: range.value[1],
      templateIds: templateId.value ? [templateId.value] : [],
      filler: filler.value || ''
    }, { responseType: 'blob' })
    const url = URL.createObjectURL(response.data)
    const link = document.createElement('a')
    link.href = url
    link.download = `${exportLabel.value}_${range.value[0]}_${range.value[1]}.xlsx`
    link.click()
    URL.revokeObjectURL(url)
  } catch (error: any) {
    ElMessage.error(error.message || '导出失败')
  } finally {
    exporting.value = false
  }
}

function canVoidSubmission(row: any) {
  if (auth.allDepartments) return false
  return auth.can('submission.void_scope') || (auth.can('submission.void_own') && row.createdBy === auth.user?.id)
}

function canRestoreSubmission(row: any) {
  if (auth.allDepartments) return false
  return auth.can('submission.edit_scope') || (auth.can('submission.edit_own') && row.createdBy === auth.user?.id)
}

function isFirstDetail(submission: any, detail: any, index: number) {
  return (submission.details || []).findIndex((item: any) => item.rowId === detail.rowId) === index
}

function canVoidDetail(detail: any) {
  if (auth.allDepartments) return false
  return auth.can('submission.void_scope') || (auth.can('submission.void_own') && detail.operatorId === auth.user?.id)
}

function canRestoreDetail(detail: any) {
  if (auth.allDepartments) return false
  return auth.can('submission.edit_scope') || (auth.can('submission.edit_own') && detail.operatorId === auth.user?.id)
}

async function loadVoidReasons() {
  try {
    const { data } = await api.get('/settings/void-reasons')
    voidReasons.value = data.items || []
  } catch {
    voidReasons.value = []
  }
}

async function openVoidDialog(target: { kind: 'submission' | 'row'; id: number; label: string }) {
  await loadVoidReasons()
  voidTarget.value = target
  voidReason.value = voidReasons.value[0] || '其他'
  otherVoidReason.value = ''
  voidDialogVisible.value = true
}

function openSubmissionVoidDialog(row: any) {
  return openVoidDialog({ kind: 'submission', id: row.id, label: `${row.templateName} · ${formatDate(row.businessDate)}` })
}

function openRowVoidDialog(submission: any, detail: any) {
  return openVoidDialog({ kind: 'row', id: detail.rowId, label: `${submission.templateName} · 明细 ${detail.rowNo}` })
}

async function submitVoid() {
  if (!voidTarget.value || !canSubmitVoidReason.value) return
  const reason = voidReason.value === '其他' ? otherVoidReason.value.trim() : voidReason.value
  voidSubmitting.value = true
  try {
    const url = voidTarget.value.kind === 'submission'
      ? `/admin/submissions/${voidTarget.value.id}/void`
      : `/execution/rows/${voidTarget.value.id}/void`
    await api.post(url, { reason })
    ElMessage.success('已废除所选内容')
    voidDialogVisible.value = false
    await Promise.all([loadLogs(), loadCalendar()])
  } catch (error: any) {
    ElMessage.error(error.message || '废除失败')
  } finally {
    voidSubmitting.value = false
  }
}

async function restoreRecord(row: any) {
  try {
    const { value } = await ElMessageBox.prompt(`恢复后“${row.templateName}”会重新计入导出与统计，请填写恢复原因。`, '整条恢复', {
      confirmButtonText: '确认恢复',
      cancelButtonText: '取消',
      inputPlaceholder: '请填写恢复原因（必填）',
      inputValidator: (text: string) => (text && text.trim() ? true : '必须填写恢复原因'),
      type: 'warning'
    })
    await api.post(`/admin/submissions/${row.id}/restore`, { reason: String(value).trim() })
    ElMessage.success('已恢复该填报')
    await Promise.all([loadLogs(), loadCalendar()])
  } catch (error: any) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error(error?.message || '恢复失败')
  }
}

async function restoreRow(submission: any, detail: any) {
  try {
    const { value } = await ElMessageBox.prompt(`恢复“${submission.templateName}”的第 ${detail.rowNo} 条明细，请填写恢复原因。`, '明细恢复', {
      confirmButtonText: '确认恢复',
      cancelButtonText: '取消',
      inputPlaceholder: '请填写恢复原因（必填）',
      inputValidator: (text: string) => (text && text.trim() ? true : '必须填写恢复原因'),
      type: 'warning'
    })
    await api.post(`/execution/rows/${detail.rowId}/restore`, { reason: String(value).trim() })
    ElMessage.success('已恢复该明细')
    await Promise.all([loadLogs(), loadCalendar()])
  } catch (error: any) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error(error?.message || '恢复失败')
  }
}


watch(calendarDate, () => { if (lineId.value) loadCalendar() })
onMounted(async () => {
  await loadTemplates()
  await loadLines()
})
</script>

<style scoped>
.page-head { display: flex; align-items: flex-end; justify-content: space-between; gap: 20px; }
.page-desc { max-width: 780px; margin: -8px 0 22px; color: var(--bench-ink-soft); font-size: 13px; line-height: 1.7; }
.calendar-panel { margin-bottom: 18px; padding: 18px 20px 20px; }
.calendar-head { display: flex; align-items: flex-end; justify-content: space-between; gap: 18px; margin-bottom: 10px; }
.calendar-head h3 { margin: 4px 0 0; font-size: 21px; letter-spacing: -.02em; }
.calendar-summary { display: flex; align-items: center; flex-wrap: wrap; gap: 14px; color: var(--bench-ink-soft); font-size: 12px; }
.calendar-summary > span { display: inline-flex; align-items: center; gap: 6px; }
.legend-chip { display: inline-block; width: 13px; height: 13px; border: 1px solid var(--bench-line); background: var(--bench-white); }
.legend-chip.filled { border-color: #a8d8bd; background: #dff2e7; }
.selected-date { padding-left: 14px; border-left: 1px solid var(--bench-line); color: var(--bench-steel-deep); font-weight: 650; }
.inspection-calendar { --el-calendar-cell-width: 62px; }
.inspection-calendar :deep(.el-calendar__header) { padding: 3px 0 12px; border-bottom: 1px solid var(--bench-line); }
.inspection-calendar :deep(.el-calendar__body) { padding: 8px 0 0; }
.inspection-calendar :deep(.el-calendar-table) { border-collapse: collapse; }
.inspection-calendar :deep(.el-calendar-table td) { border: 1px solid var(--bench-line); padding: 0; }
.inspection-calendar :deep(.el-calendar-table .el-calendar-day) { height: 74px; padding: 0; }
.calendar-cell { display: flex; flex-direction: column; justify-content: space-between; height: 100%; padding: 8px 9px; color: var(--bench-ink); background: var(--bench-white); cursor: pointer; transition: background .15s ease, box-shadow .15s ease; }
.calendar-cell:hover { background: #eef6f4; }
.calendar-cell.other-month { color: #9da5a5; background: #f3f0e8; }
.calendar-cell.has-log { border-left: 4px solid var(--bench-green); background: #dff2e7; }
.calendar-cell.has-log .calendar-day { color: #1e7455; font-weight: 720; }
.calendar-cell.is-selected { box-shadow: inset 0 0 0 2px var(--bench-steel); }
.calendar-day { font-size: 17px; font-weight: 650; }
.calendar-count { color: #1e7455; font-size: 11px; font-weight: 700; }
.calendar-empty { color: #a8ada9; font-size: 11px; }
.log-toolbar { padding: 13px 14px; }
.detail-wrap { padding: 10px 22px 16px 50px; background: #f7f4ed; }
.unit { margin-left: 6px; color: var(--bench-ink-soft); font-size: 11px; }
.state-note { margin-top: 4px; color: var(--bench-ink-soft); font-size: 11px; line-height: 1.4; }
.status-tags { display: flex; flex-wrap: wrap; gap: 4px; }
.value-cell { display: flex; flex-direction: column; gap: 3px; }
.value-cell.is-out-of-range > span, .range-warning { color: #c62828; }
.range-warning { font-size: 11px; font-weight: 700; }
:deep(.out-of-range-row > td.el-table__cell) { background: #fff1f0 !important; }
.dialog-tip { margin: 0 0 14px; color: var(--bench-ink-soft); font-size: 13px; line-height: 1.6; }
.reason-options { display: grid; gap: 8px; margin-bottom: 14px; }
.reason-options :deep(.el-radio) { height: auto; margin-right: 0; padding: 8px 10px; border: 1px solid var(--bench-line); border-radius: 3px; }
.voided { color: var(--bench-ink-soft); text-decoration: line-through; }
.pager { display: flex; justify-content: flex-end; margin-top: 16px; }
@media(max-width: 900px) {
  .page-head, .log-toolbar, .calendar-head { align-items: flex-start; flex-direction: column; }
  .detail-wrap { padding-left: 8px; }
  .selected-date { padding-left: 0; border-left: 0; }
}
</style>

