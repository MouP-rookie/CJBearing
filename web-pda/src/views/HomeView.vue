<!-- 文件说明：PDA 工作台，选择产线并展示当前可填写任务。 -->

<template>
  <div class="home-page">
    <div v-if="!online" class="offline-banner">网络已断开，当前只保留正在填写的草稿，请恢复网络后继续</div>
    <header class="workspace-head">
      <div class="head-top">
        <div class="brand"><img v-if="brandLogo" :src="brandLogo" class="brand-logo-pda" alt="Logo"><div v-else class="mark"><i></i><i></i></div><div><strong>现场工作台</strong><small>{{ auth.user?.name }} · {{ auth.user?.account }}</small></div></div>
        <div class="head-actions"><button class="history-button" @click="openHistory">我的历史</button><button class="logout-button" @click="logout">退出</button></div>
      </div>
      <button type="button" class="line-picker" aria-haspopup="dialog" :aria-expanded="showLineSheet" @click="openLinePicker">
        <span class="line-picker__copy"><small>当前产线</small><strong>{{ selectedLineName }}</strong></span>
        <span class="line-picker__switch">切换<van-icon name="arrow-down" /></span>
      </button>
    </header>

    <main class="page">
      <section class="status-strip"><span class="pulse"></span><div><strong>{{ online ? '网络正常' : '网络中断' }}</strong><small>{{ online ? '可以填报和提交数据' : '已停止填写以外的业务操作' }}</small></div></section>
      <div class="section-heading"><div><p>待办任务</p><h2>{{ selectedLineName === '请选择产线' ? '先选择产线' : '选择要填写的表单' }}</h2></div><span>{{ forms.length }} 项</span></div>
      <div v-if="formsLoading" class="task-loading" role="status"><van-loading size="20" />正在加载当前产线任务，仍可切换产线</div>
      <div v-else-if="formsError" class="empty-state" role="alert"><strong>任务加载失败</strong><p>{{ formsError }}</p><button type="button" class="retry-button" @click="reloadForms">重试加载</button><button type="button" class="retry-button" @click="openLinePicker">切换产线</button></div>
      <div v-else-if="forms.length" class="form-list">
        <button v-for="form in forms" :key="form.versionId" class="form-card" :disabled="!online" @click="openForm(form)"><span class="form-type">{{ kindName(form.kind) }}</span><span class="form-copy"><strong>{{ form.name }}</strong><small>版本 v{{ form.versionNo }} · 点击开始填写</small></span><van-icon name="arrow" /></button>
      </div>
      <div v-else class="empty-state"><strong>还没有可填写的表单</strong><p>请选择产线，或联系管理员发布任务。</p></div>
    </main>

  
    <van-popup v-model:show="showLineSheet" teleport="body" position="bottom" :style="{ maxHeight: '78vh' }">
      <section class="line-sheet">
        <div class="line-sheet__handle"></div>
        <header class="line-sheet__head">
          <div><small>产线选择</small><h3>选择当前工位</h3></div>
          <button type="button" class="line-sheet__close" @click="showLineSheet = false"><van-icon name="cross" /></button>
        </header>
        <p class="line-sheet__hint">{{ linesLoading ? '正在刷新可用产线…' : `${lines.length} 条可用产线，点击后立即切换` }}</p>
        <div v-if="linesError" class="line-sheet__error" role="alert">{{ linesError }}<button type="button" class="retry-button" @click="refreshLines">重新加载</button></div>
        <p v-else-if="!linesLoading && !lines.length" class="line-sheet__hint">当前账号暂无可用产线，请联系管理员检查产线授权。</p>
        <div class="line-sheet__list">
          <button
            v-for="line in lines"
            :key="line.id"
            type="button"
            class="line-option"
            :class="{ 'line-option--current': line.id === lineId }"
            :aria-pressed="line.id === lineId"
            @click="selectLine(line)"
          >
            <span class="line-option__copy"><strong>{{ line.name }}</strong><small>产线编号 {{ line.code || line.id }}</small></span>
            <span v-if="line.id === lineId" class="line-option__current"><van-loading v-if="formsLoading" size="18" /><van-icon v-else name="success" /> 当前</span>
            <van-icon v-else name="arrow" class="line-option__arrow" />
          </button>
        </div>
      </section>
    </van-popup>
</div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { showConfirmDialog, showToast } from 'vant'
import type { Line } from '@shared'
import { api } from '@/api'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()
const brandLogo = computed(() => auth.brandLogoDataUrl)
const lines = ref<Line[]>([])
const forms = ref<any[]>([])
const lineId = ref<number>()
const showLineSheet = ref(false)
const linesLoading = ref(false)
const linesError = ref('')
const formsLoading = ref(false)
const formsError = ref('')
const online = ref(navigator.onLine)
const selectedLineName = computed(() => lines.value.find(item => item.id === lineId.value)?.name || '请选择产线')
let lineRequestID = 0
let formRequestID = 0
let formController: AbortController | undefined
function kindName(kind: string) { return { first_inspection: '首检', final_inspection: '末检', report: '报表', registration: '登记表' }[kind] || kind }
function savedLineId() {
  try { return Number(localStorage.getItem('bearing-inspection.pda-line.' + (auth.user?.account || ''))) }
  catch { return 0 }
}
function resetSelection() {
  ++formRequestID
  formController?.abort()
  lineId.value = undefined
  forms.value = []
  formsLoading.value = false
  formsError.value = ''
  try { localStorage.removeItem('bearing-inspection.pda-line.' + (auth.user?.account || '')) } catch {}
}
// 每次打开选择器都刷新授权列表，原有选择不会因为请求未完成而锁定。
async function loadLines(restoreSelection = false) {
  const requestID = ++lineRequestID
  linesLoading.value = true
  linesError.value = ''
  try {
    const { data } = await api.get('/execution/lines')
    if (requestID !== lineRequestID) return
    lines.value = data.items || []
    if (lineId.value && !lines.value.some(line => line.id === lineId.value)) resetSelection()
    if (restoreSelection && !lineId.value) {
      const preferred = Number(route.query.lineId) || savedLineId()
      const saved = lines.value.find(line => line.id === preferred)
      if (saved) selectLine(saved)
      else {
        const leaves = lines.value.filter(line => !lines.value.some(child => child.parentId === line.id))
        if (leaves.length === 1) selectLine(leaves[0])
        else showLineSheet.value = true
      }
    }
  } catch (error: any) {
    if (requestID !== lineRequestID) return
    linesError.value = error.message || '读取产线失败，请检查网络后重试'
    if (restoreSelection) showLineSheet.value = true
  } finally {
    if (requestID === lineRequestID) linesLoading.value = false
  }
}
function refreshLines() { void loadLines() }
function openLinePicker() { showLineSheet.value = true; refreshLines() }
// 选择立即生效，任务加载使用可取消请求；旧产线的响应不能覆盖新产线。
function selectLine(item: Line, updateRoute = true) {
  if (!lines.value.some(line => line.id === item.id)) { showToast('产线已不可用，请重新选择'); return }
  showLineSheet.value = false
  if (lineId.value !== item.id || formsError.value) {
    lineId.value = item.id
    void loadForms(item.id)
  }
  try { localStorage.setItem('bearing-inspection.pda-line.' + (auth.user?.account || ''), String(item.id)) } catch {}
  if (updateRoute && Number(route.query.lineId) !== item.id) {
    void router.replace({ path: '/', query: { ...route.query, lineId: String(item.id) } }).catch(() => showToast('页面地址更新失败，请重试'))
  }
}
async function loadForms(selectedId: number) {
  const requestID = ++formRequestID
  formController?.abort()
  const controller = new AbortController()
  formController = controller
  forms.value = []
  formsError.value = ''
  formsLoading.value = true
  try {
    const { data } = await api.get('/execution/forms', { params: { lineId: selectedId }, signal: controller.signal })
    if (requestID !== formRequestID || selectedId !== lineId.value) return
    forms.value = data.items || []
  } catch (error: any) {
    if (controller.signal.aborted || requestID !== formRequestID || selectedId !== lineId.value) return
    formsError.value = error.message || '读取任务失败，请检查网络后重试'
  } finally {
    if (requestID === formRequestID) {
      formsLoading.value = false
      formController = undefined
    }
  }
}
function reloadForms() { if (lineId.value) void loadForms(lineId.value) }
function openHistory() { router.push({ path: '/history', query: lineId.value ? { lineId: String(lineId.value) } : {} }) }
function openForm(form: any) {
  if (!lineId.value || formsLoading.value || !forms.value.some(item => item.versionId === form.versionId)) return
  router.push({ path: '/form/' + form.versionId, query: { lineId: String(lineId.value), lineName: selectedLineName.value, templateId: String(form.templateId || form.template_id), name: form.name, kind: form.kind } })
}
async function logout() { try { await showConfirmDialog({ title: '退出登录', message: '确定要退出当前账号吗？' }); await auth.logout(); router.replace('/login') } catch {} }
// 浏览器返回改变产线参数时，同步工位与任务，避免恢复到旧列表。
watch(() => route.query.lineId, value => {
  const selected = lines.value.find(line => line.id === Number(value))
  if (selected && selected.id !== lineId.value) selectLine(selected, false)
})
function updateOnline() { online.value = navigator.onLine }
onMounted(() => { window.addEventListener('online', updateOnline); window.addEventListener('offline', updateOnline); void loadLines(true) })
onUnmounted(() => {
  ++lineRequestID
  ++formRequestID
  formController?.abort()
  window.removeEventListener('online', updateOnline)
  window.removeEventListener('offline', updateOnline)
})
</script>

<style scoped>
.task-loading { display: flex; align-items: center; justify-content: center; gap: 10px; padding: 28px 14px; color: #59656c; font-size: 13px; }
.retry-button { margin: 10px 5px 0; padding: 9px 13px; border: 1px solid var(--bench-line); color: var(--bench-steel); background: #fffdf8; font-size: 13px; }
.line-sheet__error { margin-bottom: 12px; color: #b94a48; font-size: 13px; }
.home-page { min-height: 100vh; }
.workspace-head { padding: 20px 16px 26px; color: #f7f3e9; background: #1b1e22; }
.head-top { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.head-actions { display: flex; align-items: center; gap: 8px; }
.history-button { padding: 7px 11px; border: 1px solid rgba(255,255,255,.24); border-radius: 3px; color: #fff; background: rgba(112,167,181,.18); font-size: 12px; }
.history-button:active { background: rgba(112,167,181,.32); }
.logout-button { padding: 7px 11px; border: 1px solid rgba(255,255,255,.18); border-radius: 3px; color: #dfe7e6; background: rgba(255,255,255,.05); font-size: 12px; }
.logout-button:active { background: rgba(255,255,255,.12); }
.brand { display: flex; align-items: center; gap: 11px; }
.brand strong, .brand small { display: block; }
.brand strong { font-size: 18px; }
.brand small { margin-top: 4px; color: #a9b4b7; font-size: 11px; }
.mark { position: relative; width: 32px; height: 32px; border: 2px solid #70a7b5; border-radius: 50%; }
.mark i { position: absolute; inset: 8px; border: 1px solid #70a7b5; border-radius: 50%; }
.mark i:nth-child(2) { inset: 12px; background: var(--bench-green); border-color: var(--bench-green); }
.brand-logo-pda { width: 32px; height: 32px; object-fit: contain; background: rgba(255,255,255,.9); border-radius: 5px; }
.line-picker { position: relative; display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: center; gap: 16px; width: 100%; margin-top: 20px; padding: 13px 13px 13px 16px; border: 1px solid rgba(112,167,181,.42); border-left: 4px solid #70a7b5; color: #f7f3e9; background: linear-gradient(90deg, rgba(112,167,181,.16), rgba(255,255,255,.035) 58%); text-align: left; }
.line-picker:focus-visible, .line-sheet__close:focus-visible, .line-option:focus-visible { outline: 2px solid #70a7b5; outline-offset: 2px; }
.line-picker:active { background: linear-gradient(90deg, rgba(112,167,181,.25), rgba(255,255,255,.06) 58%); }
.line-picker__copy { min-width: 0; }
.line-picker__copy small, .line-picker__copy strong { display: block; }
.line-picker__copy small { color: #9eb0b5; font-size: 10px; letter-spacing: .06em; }
.line-picker__copy strong { margin-top: 5px; overflow: hidden; color: #fff; font-size: 17px; font-weight: 760; letter-spacing: -.02em; text-overflow: ellipsis; white-space: nowrap; }
.line-picker__switch { display: inline-flex; align-items: center; gap: 6px; padding: 8px 9px 8px 11px; border: 1px solid rgba(112,167,181,.48); color: #d8e5e7; background: rgba(13,75,96,.42); font-size: 11px; font-weight: 700; }
.line-picker__switch .van-icon { font-size: 13px; }
.line-sheet { max-height: 78vh; border-top: 3px solid #70a7b5; padding: 10px 14px 28px; overflow: hidden; color: var(--bench-ink); background: var(--bench-paper); }
.line-sheet__handle { width: 42px; height: 4px; margin: 0 auto 14px; background: #b7b1a5; }
.line-sheet__head { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
.line-sheet__head small { color: var(--bench-steel); font-size: 10px; font-weight: 750; letter-spacing: .08em; }
.line-sheet__head h3 { margin: 5px 0 0; font-size: 22px; letter-spacing: -.035em; }
.line-sheet__close { display: grid; place-items: center; width: 38px; height: 38px; border: 1px solid var(--bench-line); color: #59656c; background: #fffdf8; font-size: 17px; }
.line-sheet__hint { margin: 14px 0 12px; color: #68757b; font-size: 12px; }
.line-sheet__list { display: grid; gap: 8px; max-height: calc(78vh - 142px); overflow-y: auto; padding-right: 2px; }
.line-option { display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: center; gap: 12px; width: 100%; min-height: 62px; padding: 11px 12px 11px 14px; border: 1px solid var(--bench-line); border-left: 4px solid transparent; color: var(--bench-ink); background: #fffdf8; text-align: left; }
.line-option:disabled { opacity: .65; }
.line-option:not(:disabled):active { background: #e8f1f0; }
.line-option--current { border-left-color: var(--bench-green); background: #e9f4ee; }
.line-option__copy { min-width: 0; }
.line-option__copy strong, .line-option__copy small { display: block; }
.line-option__copy strong { overflow: hidden; font-size: 15px; text-overflow: ellipsis; white-space: nowrap; }
.line-option__copy small { margin-top: 5px; color: #718087; font-size: 10px; }
.line-option__current { display: inline-flex; align-items: center; gap: 4px; color: #1f7657; font-size: 11px; font-weight: 750; }
.line-option__arrow { color: var(--bench-steel); font-size: 16px; }
.status-strip { display: flex; align-items: center; gap: 11px; margin-top: -10px; padding: 13px 14px; border: 1px solid var(--bench-line); background: #fffdf8; box-shadow: 0 10px 26px rgba(27,30,34,.08); }
.status-strip strong, .status-strip small { display: block; }
.status-strip strong { font-size: 13px; }
.status-strip small { margin-top: 3px; color: #66727a; font-size: 11px; }
.pulse { width: 8px; height: 8px; border-radius: 50%; background: var(--bench-green); box-shadow: 0 0 0 5px rgba(46,139,101,.12); }
.section-heading { display: flex; align-items: flex-end; justify-content: space-between; margin: 28px 0 12px; }
.section-heading p { margin: 0 0 5px; color: var(--bench-steel); font-size: 11px; font-weight: 700; }
.section-heading h2 { margin: 0; font-size: 21px; letter-spacing: -.03em; }
.section-heading > span { color: #68757b; font-size: 11px; }
.form-list { display: grid; gap: 10px; }
.form-card { display: grid; grid-template-columns: auto 1fr auto; align-items: center; gap: 11px; width: 100%; padding: 15px 12px; border: 1px solid var(--bench-line); border-left: 4px solid var(--bench-steel); color: var(--bench-ink); background: #fffdf8; text-align: left; }
.form-card:disabled { opacity: .5; }
.form-card:not(:disabled):active { background: #edf3f3; }
.form-type { padding: 4px 6px; color: var(--bench-steel-deep); border: 1px solid #b5ccd2; font-size: 10px; font-weight: 700; }
.form-copy strong, .form-copy small { display: block; }
.form-copy strong { font-size: 15px; }
.form-copy small { margin-top: 5px; color: #68757b; font-size: 11px; }
.empty-state { padding: 36px 18px; border: 1px dashed #b8b3a8; color: #68757b; background: rgba(255,253,248,.7); text-align: center; }
.empty-state strong { color: var(--bench-ink); }
.empty-state p { margin: 8px 0 0; font-size: 12px; }
</style>







