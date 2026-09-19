<!-- 文件说明：PDA 我的历史页，查看本人明细并执行废除或恢复。 -->

<template>
  <div class="history-page">
    <div v-if="!online" class="offline-banner">网络已断开，历史查询、废除和恢复暂不可用
</div>
    <van-nav-bar title="我的历史" left-text="返回" @click-left="router.back()">
      <template #right><van-icon name="replay" size="19" @click="reload" /></template>
    </van-nav-bar>

    <div class="page">
      <button type="button" class="line-filter" @click="showLineSheet = true">
        <span class="line-filter__copy"><small>查询范围</small><strong>{{ selectedLineName }}</strong></span>
        <span class="line-filter__switch">更换<van-icon name="arrow-down" /></span>
      </button>

      <van-pull-refresh v-model="refreshing" @refresh="reload">
        <van-list v-model:loading="loading" :finished="finished" finished-text="没有更多记录" @load="loadPage">
          <button v-for="item in items" :key="item.id" class="history-card" @click="openDetail(item.id)">
            <div class="card-head">
              <span class="kind">{{ kindName(item.kind) }}</span>
              <van-tag :type="statusOf(item).type" size="medium">{{ statusOf(item).text }}</van-tag>
            </div>
            <strong>{{ item.templateName }}</strong>
            <div class="card-meta">
              <span>{{ formatDate(item.businessDate) }}</span>
              <span>{{ item.shiftNameSnapshot }}</span>
              <span>{{ item.lineName }}</span>
            </div>
            <div class="card-foot">
              <span>本人明细 {{ item.ownRowCount }} 条</span>
              <span v-if="item.voidedRowCount">{{ item.voidedRowCount }} 条已废除</span>
              <span>{{ formatDateTime(item.submittedAt) }}</span>
            </div>
          </button>
          <van-empty v-if="!loading && !items.length" description="暂无本人填报历史" />
        </van-list>
      </van-pull-refresh>
    </div>

    <van-popup v-model:show="detailVisible" position="bottom" round :style="{ height: '88%' }" @closed="activeItem = null">
      <div v-if="activeItem" class="detail-sheet">
        <div class="sheet-head">
          <div><small>{{ kindName(activeItem.kind) }} · {{ activeItem.lineName }}</small><h3>{{ activeItem.templateName }}</h3></div>
          <van-icon name="cross" size="20" @click="detailVisible = false" />
        </div>
        <div class="detail-meta">
          <span>业务日期 {{ formatDate(activeItem.businessDate) }}</span>
          <span>班次 {{ activeItem.shiftNameSnapshot }}</span>
          <span>提交时间 {{ formatDateTime(activeItem.submittedAt) }}</span>
        </div>
        <div v-if="activeItem.repeatReason" class="repeat-reason">重复首检原因：{{ activeItem.repeatReason }}</div>

        <div class="detail-list">
          <article v-for="row in activeItem.rows" :key="row.id" class="detail-row" :class="{ voided: row.status === 'voided' }">
            <div class="row-head">
              <strong>明细 {{ row.rowNo }}</strong>
              <div class="row-state">
                <van-tag v-if="rowHasOutOfRange(row)" type="danger">上下限异常</van-tag>
                <van-tag v-if="row.status === 'voided'" type="danger">已废除</van-tag>
                <van-tag v-else-if="row.superseded" type="primary">已被覆盖</van-tag>
                <van-tag v-else type="success">有效</van-tag>
              </div>
            </div>
            <dl>
              <template v-for="(value, index) in row.values" :key="`${row.id}-${value.fieldKey}-${index}`">
                <dt>{{ value.fieldLabel }}<small v-if="value.unit">（{{ value.unit }}）</small></dt>
                <dd :class="{ 'range-warning': value.outOfRange && row.status === 'active' }">
                  <span>{{ formatValue(value) }}</span>
                  <small v-if="value.outOfRange && row.status === 'active'">{{ rangeWarning(value) }}</small>
                </dd>
              </template>
            </dl>
            <div v-if="row.voidReason" class="state-reason danger">废除原因：{{ row.voidReason }}</div>
            <div v-if="row.restoreReason" class="state-reason success">恢复原因：{{ row.restoreReason }}</div>
            <div class="row-actions">
              <van-button v-if="row.status === 'active'" size="small" plain type="danger" :disabled="!online" @click="openVoid(row, activeItem)">废除该明细</van-button>
              <van-button v-else size="small" plain type="primary" :disabled="!online" @click="openRestore(row, activeItem)">恢复该明细</van-button>
            </div>
          </article>
        </div>
      </div>
    </van-popup>

    <van-popup v-model:show="voidVisible" position="bottom" round :style="{ height: '72%' }">
      <div class="action-sheet">
        <div class="sheet-head"><div><small>废除明细</small><h3>{{ actionLabel }}</h3></div><van-icon name="cross" size="20" @click="voidVisible = false" /></div>
        <p class="sheet-tip">请选择废除原因；选项不完整时选择“其他”并手填具体原因。</p>
        <van-radio-group v-model="voidReason" class="reason-list">
          <van-radio v-for="reason in voidReasons" :key="reason" :name="reason">{{ reason }}</van-radio>
          <van-radio name="其他">其他</van-radio>
        </van-radio-group>
        <van-field v-if="voidReason === '其他'" v-model="otherVoidReason" type="textarea" rows="3" autosize maxlength="500" show-word-limit placeholder="请填写废除原因（必填）" />
        <div class="sheet-actions">
          <van-button block plain @click="voidVisible = false">取消</van-button>
          <van-button block type="danger" :loading="submitting" :disabled="!canSubmitVoid" @click="submitVoid">确认废除</van-button>
        </div>
      </div>
    </van-popup>

    <van-popup v-model:show="restoreVisible" position="bottom" round :style="{ height: '58%' }">
      <div class="action-sheet">
        <div class="sheet-head"><div><small>恢复明细</small><h3>{{ actionLabel }}</h3></div><van-icon name="cross" size="20" @click="restoreVisible = false" /></div>
        <p class="sheet-tip">恢复后该明细会重新参与统计与导出，请填写恢复原因。</p>
        <van-field v-model="restoreReason" type="textarea" rows="4" autosize maxlength="500" show-word-limit placeholder="请填写恢复原因（必填）" />
        <div class="sheet-actions">
          <van-button block plain @click="restoreVisible = false">取消</van-button>
          <van-button block type="primary" :loading="submitting" :disabled="!restoreReason.trim()" @click="submitRestore">确认恢复</van-button>
        </div>
      </div>
    </van-popup>
  
    <van-popup v-model:show="showLineSheet" position="bottom" :style="{ maxHeight: '76vh' }">
      <section class="history-line-sheet">
        <div class="history-line-sheet__handle"></div>
        <header class="history-line-sheet__head">
          <div><small>查询范围</small><h3>选择历史产线</h3></div>
          <button type="button" class="history-line-sheet__close" @click="showLineSheet = false"><van-icon name="cross" /></button>
        </header>
        <div class="history-line-list">
          <button type="button" class="history-line-option" :class="{ 'history-line-option--current': selectedLine === 0 }" @click="selectHistoryLine(0)">
            <span><strong>全部授权产线</strong><small>显示当前账号可查看的全部历史</small></span>
            <span v-if="selectedLine === 0" class="history-line-current"><van-icon name="success" /> 当前</span>
            <van-icon v-else name="arrow" />
          </button>
          <button
            v-for="line in lines"
            :key="line.id"
            type="button"
            class="history-line-option"
            :class="{ 'history-line-option--current': selectedLine === line.id }"
            @click="selectHistoryLine(line.id)"
          >
            <span><strong>{{ line.name }}</strong><small>产线编号 {{ line.code || line.id }}</small></span>
            <span v-if="selectedLine === line.id" class="history-line-current"><van-icon name="success" /> 当前</span>
            <van-icon v-else name="arrow" />
          </button>
        </div>
      </section>
    </van-popup>
</div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { showToast } from 'vant'
import { api } from '@/api'

type HistoryTagType = 'primary' | 'success' | 'warning' | 'danger'

const route = useRoute()
const router = useRouter()
const online = ref(navigator.onLine)
const lines = ref<any[]>([])
const selectedLine = ref<number>(Number(route.query.lineId) || 0)
const showLineSheet = ref(false)
const items = ref<any[]>([])
const page = ref(1)
const size = 20
const loading = ref(false)
const refreshing = ref(false)
const finished = ref(false)
const detailVisible = ref(false)
const activeItem = ref<any>(null)
const voidVisible = ref(false)
const voidReasons = ref<string[]>([])
const voidReason = ref('')
const otherVoidReason = ref('')
const restoreVisible = ref(false)
const restoreReason = ref('')
const actionTarget = ref<any>(null)
const actionLabel = ref('')
const submitting = ref(false)
let requestVersion = 0

const selectedLineName = computed(() => selectedLine.value === 0 ? '全部授权产线' : lines.value.find(line => line.id === selectedLine.value)?.name || '当前产线')
const canSubmitVoid = computed(() => voidReason.value === '其他' ? Boolean(otherVoidReason.value.trim()) : Boolean(voidReason.value))

function kindName(kind: string) {
  return { first_inspection: '首检', final_inspection: '末检', report: '报表', registration: '登记表' }[kind] || kind
}
function pad(value: number) { return String(value).padStart(2, '0') }
function formatDate(value: string) { const date = new Date(value); return Number.isNaN(date.getTime()) ? value : `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}` }
function formatDateTime(value: string) { if (!value) return '-'; const date = new Date(value); return Number.isNaN(date.getTime()) ? value : `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}` }
function formatValue(value: any) { const raw = value.valueNumber !== null && value.valueNumber !== undefined ? value.valueNumber : value.valueText; if (raw === '' || raw === null || raw === undefined) return '-'; return value.unit ? `${raw} ${value.unit}` : String(raw) }
function rowHasOutOfRange(row: any) { return row.status === 'active' && (row.values || []).some((value: any) => value.outOfRange) }
function rangeWarning(value: any) {
  const limits: string[] = []
  if (value.min !== null && value.min !== undefined) limits.push(`下限 ${value.min}`)
  if (value.max !== null && value.max !== undefined) limits.push(`上限 ${value.max}`)
  const suffix = value.unit ? ` ${value.unit}` : ''
  return limits.length ? `超出上下限（${limits.join('，')}${suffix}）` : '超出上下限'
}
// 汇总本人明细的有效、部分废除、已废除和已被覆盖状态。
function statusOf(item: any): { text: string; type: HistoryTagType } {
  if (!item.activeRowCount) return { text: '已废除', type: 'danger' }
  if (item.voidedRowCount) return { text: '部分废除', type: 'warning' }
  if (item.superseded) return { text: '已被覆盖', type: 'primary' }
  return { text: '有效', type: 'success' }
}
function updateOnline() { online.value = navigator.onLine }

async function loadLines() {
  try {
    const { data } = await api.get('/execution/lines')
    lines.value = data.items || []
    if (selectedLine.value && !lines.value.some(line => line.id === selectedLine.value)) selectedLine.value = 0
  } catch (error: any) {
    showToast(error.message || '读取产线失败')
  }
}

function selectHistoryLine(id: number) {
  selectedLine.value = id
  showLineSheet.value = false
  reload()
}

// 使用递增页码加载本人历史，并防止旧请求覆盖新筛选。
async function loadPage(version = requestVersion) {
  if (version !== requestVersion) return
  if (!online.value) {
    loading.value = false
    finished.value = true
    return
  }
  try {
    const { data } = await api.get('/execution/my-submissions', {
      params: { lineId: selectedLine.value || undefined, page: page.value, size }
    })
    if (version !== requestVersion) return
    const next = data.items || []
    items.value.push(...next)
    finished.value = next.length < size
    page.value += 1
  } catch (error: any) {
    if (version !== requestVersion) return
    finished.value = true
    showToast(error.message || '查询历史失败')
  } finally {
    if (version === requestVersion) {
      loading.value = false
      refreshing.value = false
    }
  }
}

async function reload() {
  const version = ++requestVersion
  if (!online.value) {
    showToast('网络断开，无法查询历史')
    refreshing.value = false
    return
  }
  page.value = 1
  items.value = []
  finished.value = false
  loading.value = true
  await loadPage(version)
}

// 详情只返回当前用户自己的明细，但展示同填报的公共上下文。
async function openDetail(id: number) {
  if (!online.value) { showToast('网络断开，无法查看历史'); return }
  try {
    const { data } = await api.get(`/execution/my-submissions/${id}`)
    activeItem.value = data
    detailVisible.value = true
  } catch (error: any) {
    showToast(error.message || '读取历史详情失败')
  }
}

async function loadVoidReasons() {
  try {
    const { data } = await api.get('/settings/void-reasons')
    voidReasons.value = data.items || []
  } catch {
    voidReasons.value = []
  }
}

async function openVoid(row: any, item: any) {
  if (!online.value) { showToast('网络断开，无法废除'); return }
  await loadVoidReasons()
  actionTarget.value = row
  actionLabel.value = `${item.templateName} · 明细 ${row.rowNo}`
  voidReason.value = voidReasons.value[0] || '其他'
  otherVoidReason.value = ''
  voidVisible.value = true
}

function openRestore(row: any, item: any) {
  if (!online.value) { showToast('网络断开，无法恢复'); return }
  actionTarget.value = row
  actionLabel.value = `${item.templateName} · 明细 ${row.rowNo}`
  restoreReason.value = ''
  restoreVisible.value = true
}

// 明细废除必须选择或填写原因，操作结果写审计日志。
async function submitVoid() {
  if (!actionTarget.value || !canSubmitVoid.value) return
  submitting.value = true
  try {
    const reason = voidReason.value === '其他' ? otherVoidReason.value.trim() : voidReason.value
    await api.post(`/execution/rows/${actionTarget.value.id}/void`, { reason })
    showToast('明细已废除')
    voidVisible.value = false
    await afterAction()
  } catch (error: any) {
    showToast(error.message || '废除失败')
  } finally {
    submitting.value = false
  }
}

// 明细恢复必须填写恢复原因，恢复后重新加载详情。
async function submitRestore() {
  if (!actionTarget.value || !restoreReason.value.trim()) return
  submitting.value = true
  try {
    await api.post(`/execution/rows/${actionTarget.value.id}/restore`, { reason: restoreReason.value.trim() })
    showToast('明细已恢复')
    restoreVisible.value = false
    await afterAction()
  } catch (error: any) {
    showToast(error.message || '恢复失败')
  } finally {
    submitting.value = false
  }
}

async function afterAction() {
  const submissionID = activeItem.value?.id
  await reload()
  if (submissionID) await openDetail(submissionID)
}

onMounted(() => {
  window.addEventListener('online', updateOnline)
  window.addEventListener('offline', updateOnline)
  loadLines()
  reload()
})
onUnmounted(() => {
  window.removeEventListener('online', updateOnline)
  window.removeEventListener('offline', updateOnline)
})
</script>

<style scoped>
.history-page { min-height: 100vh; }
.line-filter { display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: center; gap: 14px; width: 100%; margin-bottom: 14px; padding: 12px 12px 12px 14px; border: 1px solid var(--bench-line); border-left: 4px solid var(--bench-steel); color: var(--bench-ink); background: #fffdf8; text-align: left; }
.line-filter__copy { min-width: 0; }
.line-filter__copy small, .line-filter__copy strong { display: block; }
.line-filter__copy small { color: #66727a; font-size: 10px; }
.line-filter__copy strong { margin-top: 4px; overflow: hidden; font-size: 15px; text-overflow: ellipsis; white-space: nowrap; }
.line-filter__switch { display: inline-flex; align-items: center; gap: 5px; padding: 7px 9px; border: 1px solid #b5ccd2; color: var(--bench-steel-deep); background: #edf4f4; font-size: 11px; font-weight: 750; }
.history-line-sheet { max-height: 76vh; border-top: 3px solid #70a7b5; padding: 10px 14px 28px; color: var(--bench-ink); background: var(--bench-paper); }
.history-line-sheet__handle { width: 42px; height: 4px; margin: 0 auto 14px; background: #b7b1a5; }
.history-line-sheet__head { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-bottom: 14px; }
.history-line-sheet__head small { color: var(--bench-steel); font-size: 10px; font-weight: 750; letter-spacing: .08em; }
.history-line-sheet__head h3 { margin: 5px 0 0; font-size: 22px; letter-spacing: -.035em; }
.history-line-sheet__close { display: grid; place-items: center; width: 38px; height: 38px; border: 1px solid var(--bench-line); color: #59656c; background: #fffdf8; font-size: 17px; }
.history-line-list { display: grid; gap: 8px; max-height: calc(76vh - 132px); overflow-y: auto; padding-right: 2px; }
.history-line-option { display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: center; gap: 12px; width: 100%; min-height: 60px; padding: 10px 12px 10px 14px; border: 1px solid var(--bench-line); border-left: 4px solid transparent; color: var(--bench-ink); background: #fffdf8; text-align: left; }
.history-line-option--current { border-left-color: var(--bench-green); background: #e9f4ee; }
.history-line-option strong, .history-line-option small { display: block; }
.history-line-option strong { overflow: hidden; font-size: 15px; text-overflow: ellipsis; white-space: nowrap; }
.history-line-option small { margin-top: 5px; color: #718087; font-size: 10px; }
.history-line-current { display: inline-flex; align-items: center; gap: 4px; color: #1f7657; font-size: 11px; font-weight: 750; }
.line-filter:focus-visible, .history-line-sheet__close:focus-visible, .history-line-option:focus-visible { outline: 2px solid #70a7b5; outline-offset: 2px; }
.history-card { display: block; width: 100%; margin-bottom: 10px; padding: 14px; border: 1px solid var(--bench-line); border-left: 4px solid var(--bench-steel); color: var(--bench-ink); background: #fffdf8; text-align: left; box-shadow: 0 7px 18px rgba(27,30,34,.05); }
.history-card:active { background: #edf3f3; }
.card-head, .row-head, .card-foot, .sheet-head { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
.row-state { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 6px; }
.card-head { margin-bottom: 8px; }
.kind { color: var(--bench-steel-deep); font-size: 11px; font-weight: 750; }
.history-card > strong { display: block; font-size: 16px; }
.card-meta { display: flex; flex-wrap: wrap; gap: 7px 12px; margin-top: 9px; color: #4e5961; font-size: 12px; }
.card-foot { margin-top: 11px; padding-top: 9px; border-top: 1px solid var(--bench-line); color: #718087; font-size: 10px; }
.detail-sheet, .action-sheet { height: 100%; padding: 18px 14px 28px; overflow: auto; background: #f3f0e8; }
.sheet-head h3 { margin: 4px 0 0; font-size: 18px; }
.sheet-head small { color: #66727a; font-size: 10px; }
.detail-meta { display: grid; gap: 5px; margin: 14px 0; padding: 11px; border: 1px solid var(--bench-line); color: #59656c; background: #fffdf8; font-size: 12px; }
.repeat-reason { margin-bottom: 14px; padding: 10px; color: #7a4b00; background: #fff0d7; font-size: 12px; }
.detail-list { display: grid; gap: 12px; }
.detail-row { padding: 13px; border: 1px solid var(--bench-line); background: #fffdf8; }
.detail-row.voided { opacity: .76; }
.detail-row dl { display: grid; grid-template-columns: minmax(90px, 38%) 1fr; gap: 7px 12px; margin: 12px 0 0; }
.detail-row dt { color: #66727a; font-size: 12px; }
.detail-row dd { margin: 0; text-align: right; font-size: 13px; font-weight: 650; word-break: break-all; }
.detail-row dd.range-warning { color: #c62828; }
.detail-row dd.range-warning small { display: block; margin-top: 3px; color: #c62828; font-size: 10px; font-weight: 650; }
.state-reason { margin-top: 10px; padding: 8px 9px; font-size: 11px; line-height: 1.5; }
.state-reason.danger { color: #8f3533; background: #fbe9e8; }
.state-reason.success { color: #246b52; background: #e7f4ed; }
.row-actions { display: flex; justify-content: flex-end; margin-top: 12px; }
.sheet-tip { margin: 16px 0 12px; color: #66727a; font-size: 12px; line-height: 1.6; }
.reason-list { display: grid; gap: 2px; margin-bottom: 12px; border: 1px solid var(--bench-line); background: #fffdf8; }
.reason-list :deep(.van-radio) { min-height: 46px; padding: 0 12px; border-bottom: 1px solid var(--bench-line); }
.reason-list :deep(.van-radio:last-child) { border-bottom: 0; }
.sheet-actions { display: grid; grid-template-columns: 1fr 1.4fr; gap: 10px; margin-top: 18px; }
</style>




