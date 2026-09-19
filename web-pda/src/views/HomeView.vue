<template>
  <div class="home-page">
    <div v-if="!online" class="offline-banner">网络已断开，当前只保留正在填写的草稿，请恢复网络后继续</div>
    <header class="workspace-head">
      <div class="head-top">
        <div class="brand"><img v-if="brandLogo" :src="brandLogo" class="brand-logo-pda" alt="Logo"><div v-else class="mark"><i></i><i></i></div><div><strong>现场工作台</strong><small>{{ auth.user?.name }} · {{ auth.user?.account }}</small></div></div>
        <div class="head-actions"><button class="history-button" @click="openHistory">我的历史</button><button class="logout-button" @click="logout">退出</button></div>
      </div>
      <button type="button" class="line-picker" @click="showLineSheet = true">
        <span class="line-picker__copy"><small>当前产线</small><strong>{{ selectedLineName }}</strong></span>
        <span class="line-picker__switch">切换<van-icon name="arrow-down" /></span>
      </button>
    </header>

    <main class="page">
      <section class="status-strip"><span class="pulse"></span><div><strong>{{ online ? '网络正常' : '网络中断' }}</strong><small>{{ online ? '可以填报和提交数据' : '已停止填写以外的业务操作' }}</small></div></section>
      <div class="section-heading"><div><p>待办任务</p><h2>{{ selectedLineName === '请选择产线' ? '先选择产线' : '选择要填写的表单' }}</h2></div><span>{{ forms.length }} 项</span></div>
      <div v-if="forms.length" class="form-list">
        <button v-for="form in forms" :key="form.versionId" class="form-card" :disabled="!online" @click="openForm(form)"><span class="form-type">{{ kindName(form.kind) }}</span><span class="form-copy"><strong>{{ form.name }}</strong><small>版本 v{{ form.versionNo }} · 点击开始填写</small></span><van-icon name="arrow" /></button>
      </div>
      <div v-else class="empty-state"><strong>还没有可填写的表单</strong><p>请选择产线，或联系管理员发布任务。</p></div>
    </main>

  
    <van-popup v-model:show="showLineSheet" position="bottom" :style="{ maxHeight: '78vh' }">
      <section class="line-sheet">
        <div class="line-sheet__handle"></div>
        <header class="line-sheet__head">
          <div><small>产线选择</small><h3>选择当前工位</h3></div>
          <button type="button" class="line-sheet__close" @click="showLineSheet = false"><van-icon name="cross" /></button>
        </header>
        <p class="line-sheet__hint">{{ lines.length }} 条可用产线，点击后立即切换</p>
        <div class="line-sheet__list">
          <button
            v-for="line in lines"
            :key="line.id"
            type="button"
            class="line-option"
            :class="{ 'line-option--current': line.id === lineId }"
            :disabled="switchingLineId !== null && switchingLineId !== undefined"
            @click="selectLine(line)"
          >
            <span class="line-option__copy"><strong>{{ line.name }}</strong><small>产线编号 {{ line.code || line.id }}</small></span>
            <span v-if="line.id === lineId" class="line-option__current"><van-icon name="success" /> 当前</span>
            <van-loading v-else-if="switchingLineId === line.id" size="18" />
            <van-icon v-else name="arrow" class="line-option__arrow" />
          </button>
        </div>
      </section>
    </van-popup>
</div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { showConfirmDialog, showToast } from 'vant'
import { api } from '@/api'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()
const brandLogo = ref('')
const lines = ref<any[]>([])
const forms = ref<any[]>([])
const lineId = ref<number>()
const showLineSheet = ref(false)
const switchingLineId = ref<number | null>(null)
const online = ref(navigator.onLine)
const selectedLineName = computed(() => lines.value.find(item => item.id === lineId.value)?.name || '请选择产线')
async function loadBrand() {
  try { const { data } = await api.get('/settings/brand'); brandLogo.value = data.logoDataUrl || '' } catch {}
}
function kindName(kind: string) { return { first_inspection: '首检', final_inspection: '末检', report: '报表', registration: '登记表' }[kind] || kind }
async function loadLines() {
  const { data } = await api.get('/execution/lines')
  lines.value = data.items
  const routeLine = Number(route.query.lineId)
  const saved = routeLine > 0 ? routeLine : Number(localStorage.getItem(`bearing-inspection.pda-line.${auth.user?.account}`))
  const savedLine = lines.value.find(item => item.id === saved)
  if (savedLine) { await selectLine(savedLine); return }
  const leaves = lines.value.filter(item => !lines.value.some(child => child.parentId === item.id))
  if (leaves.length === 1) await selectLine(leaves[0])
}
async function selectLine(item: any) {
  switchingLineId.value = item.id
  try {
    const { data } = await api.get('/execution/forms', { params: { lineId: item.id } })
    lineId.value = item.id
    showLineSheet.value = false
    online.value = true
    localStorage.setItem('bearing-inspection.pda-line.' + (auth.user?.account || ''), String(item.id))
    await router.replace({ path: '/', query: { ...route.query, lineId: String(item.id) } })
    forms.value = data.items
  } catch (error: any) {
    showToast(error?.message || '切换产线失败，请检查网络')
  } finally {
    switchingLineId.value = null
  }
}
function openHistory() { router.push({ path: '/history', query: lineId.value ? { lineId: String(lineId.value) } : {} }) }
function openForm(form: any) { router.push({ path: `/form/${form.versionId}`, query: { lineId: String(lineId.value), lineName: selectedLineName.value, templateId: String(form.templateId || form.template_id), name: form.name, kind: form.kind } }) }
async function logout() { try { await showConfirmDialog({ title: '退出登录', message: '确定要退出当前账号吗？' }); await auth.logout(); router.replace('/login') } catch {} }

function updateOnline() { online.value = navigator.onLine }
onMounted(() => { window.addEventListener('online', updateOnline); window.addEventListener('offline', updateOnline); loadBrand(); loadLines() })
onUnmounted(() => { window.removeEventListener('online', updateOnline); window.removeEventListener('offline', updateOnline) })
</script>

<style scoped>
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







