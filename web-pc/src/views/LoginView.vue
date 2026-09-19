<!-- 文件说明：PC 登录页，支持记住密码、自动登录和品牌展示。 -->

<template>
  <div class="login-screen">
    <section class="briefing">
      <div class="briefing-top">
        <div class="brand-lockup">
          <img v-if="brandLogo" :src="brandLogo" class="brand-logo-login" alt="Logo">
          <div v-else class="bearing-symbol"><i></i><i></i><i></i></div>
          <div><strong>轴承首末检系统</strong><span>BEARING PROCESS INSPECTION</span></div>
        </div>
        <span class="network-badge"><i></i> 局域网运行</span>
      </div>

      <div class="briefing-copy">
        <p class="eyebrow">质量过程控制 · 首件确认 · 末件追溯</p>
        <h1>把每一道检验，<br />留在产线上。</h1>
        <p class="lede">面向 PC 与 PDA 的无纸化首末检、报表和登记表工作台。</p>
      </div>

      <div class="line-board" aria-label="生产流程示意">
        <div class="line-node active"><span>01</span><div><strong>开班首检</strong><small>自动带出班次与任务</small></div></div>
        <div class="line-track"></div>
        <div class="line-node"><span>02</span><div><strong>过程填报</strong><small>仪表数据与人工确认</small></div></div>
        <div class="line-track"></div>
        <div class="line-node"><span>03</span><div><strong>日志归档</strong><small>按日期与版本追溯</small></div></div>
      </div>

      <div class="briefing-foot"><span>精密制造</span><span>过程可查</span><span>数据留存</span></div>
    </section>

    <section class="login-panel">
      <div class="form-wrap">
        <div class="form-heading">
          <p class="section-label">PC 管理与配置端</p>
          <h2>登录工作台</h2>
          <p>使用工号或账号登录，普通用户请前往 PDA 端填报。</p>
        </div>

        <el-form @submit.prevent="submit()">
          <el-form-item label="账号 / 工号">
            <el-input v-model="account" size="large" autocomplete="username" placeholder="请输入账号或工号" />
          </el-form-item>
          <el-form-item label="密码">
            <el-input v-model="password" size="large" type="password" show-password autocomplete="current-password" placeholder="请输入密码" @keyup.enter="submit()" />
          </el-form-item>
          <div class="login-options">
            <el-checkbox v-model="rememberPassword" @change="onRememberChange">记住密码</el-checkbox>
            <el-checkbox v-model="autoLogin" @change="onAutoLoginChange">自动登录</el-checkbox>
          </div>
          <el-button class="submit-button" type="primary" size="large" :loading="loading" @click="submit()">进入系统</el-button>
        </el-form>

        <div class="form-note"><span></span>登录状态有效期 12 小时</div>
      </div>
    </section>
    <CopyrightFooter />
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { api } from '@/api'
import CopyrightFooter from '@/components/CopyrightFooter.vue'

type SavedLogin = { account: string; password: string; remember: boolean; auto: boolean }
const STORAGE_KEY = 'bearing-inspection.pc-login'
const auth = useAuthStore()
const router = useRouter()
const brandLogo = ref('')
const account = ref('')
const password = ref('')
const rememberPassword = ref(false)
const autoLogin = ref(false)
const loading = ref(false)

function readSaved(): SavedLogin | null {
  try { return JSON.parse(localStorage.getItem(STORAGE_KEY) || 'null') } catch { return null }
}
// 记住密码会写入浏览器 localStorage，公用电脑不应启用。
function persist() {
  const saved: SavedLogin = { account: account.value.trim(), password: password.value, remember: rememberPassword.value || autoLogin.value, auto: autoLogin.value }
  localStorage.setItem(STORAGE_KEY, JSON.stringify(saved))
}
function clearSaved() { localStorage.removeItem(STORAGE_KEY) }
function onRememberChange(value: boolean | string | number) {
  if (!Boolean(value) && autoLogin.value) autoLogin.value = false
}
function onAutoLoginChange(value: boolean | string | number) {
  if (Boolean(value)) rememberPassword.value = true
}

async function loadBrand() {
  try { const { data } = await api.get('/settings/brand'); brandLogo.value = data.logoDataUrl || '' } catch {}
}
// PC 登录使用账号密码，成功后加载 /me 并进入系统。
async function submit(fromAuto = false) {
  if (!account.value.trim() || !password.value) { ElMessage.warning('请输入账号和密码'); return }
  loading.value = true
  try {
    await auth.login(account.value.trim(), password.value)
    if (rememberPassword.value || autoLogin.value) persist(); else clearSaved()
    await router.push('/')
  } catch (error: any) {
    if (fromAuto) { autoLogin.value = false; clearSaved() }
    ElMessage.error(error.message || '登录失败')
  } finally { loading.value = false }
}

onMounted(async () => {
  loadBrand()
  const saved = readSaved()
  if (!saved) return
  account.value = saved.account || ''
  password.value = saved.password || ''
  rememberPassword.value = saved.remember
  autoLogin.value = saved.auto
  if (saved.auto && saved.account && saved.password) await submit(true)
})
</script>

<style scoped>
.login-screen { display: grid; grid-template-columns: minmax(0, 1.12fr) minmax(390px, .88fr); grid-template-rows: minmax(0, 1fr) auto; min-height: 100vh; background: var(--bench-paper); }
.briefing { position: relative; display: flex; flex-direction: column; justify-content: space-between; padding: clamp(28px, 5vw, 68px); overflow: hidden; color: #f7f3e9; background: #1b1e22; }
.briefing::before { content: ""; position: absolute; inset: 0; opacity: .32; background: linear-gradient(rgba(112,167,181,.14) 1px, transparent 1px), linear-gradient(90deg, rgba(112,167,181,.14) 1px, transparent 1px); background-size: 34px 34px; }
.briefing::after { content: ""; position: absolute; right: -120px; bottom: -165px; width: 430px; height: 430px; border: 48px solid #0d4b60; border-radius: 50%; box-shadow: 0 0 0 24px rgba(112,167,181,.09), 0 0 0 72px rgba(112,167,181,.045); }
.briefing-top, .briefing-copy, .line-board, .briefing-foot { position: relative; z-index: 1; }
.briefing-top { display: flex; align-items: center; justify-content: space-between; gap: 18px; }
.brand-lockup { display: flex; align-items: center; gap: 14px; }
.brand-lockup strong, .brand-lockup span { display: block; }
.brand-lockup strong { font-size: 17px; letter-spacing: .03em; }
.brand-lockup span { margin-top: 5px; color: #94a2a7; font-size: 9px; letter-spacing: .14em; }
.bearing-symbol { position: relative; width: 42px; height: 42px; border: 2px solid #70a7b5; border-radius: 50%; }
.bearing-symbol i { position: absolute; inset: 8px; border: 1px solid #70a7b5; border-radius: 50%; }
.bearing-symbol i:nth-child(2) { inset: 15px; background: #2e8b65; border-color: #2e8b65; }
.bearing-symbol i:nth-child(3) { inset: 0; border-style: dashed; transform: rotate(28deg); }
.brand-logo-login { width: 42px; height: 42px; object-fit: contain; background: rgba(255,255,255,.9); border-radius: 6px; }
.network-badge { display: inline-flex; align-items: center; gap: 8px; padding: 7px 10px; border: 1px solid rgba(255,255,255,.16); color: #cdd6d8; font-size: 11px; }
.network-badge i { width: 7px; height: 7px; border-radius: 50%; background: #2e8b65; }
.briefing-copy { max-width: 700px; margin: 60px 0 24px; }
.eyebrow { margin: 0 0 17px; color: #84b3bf; font-size: 11px; letter-spacing: .16em; }
.briefing-copy h1 { margin: 0; max-width: 660px; font-size: clamp(44px, 6vw, 82px); line-height: 1.03; letter-spacing: -.055em; font-weight: 680; }
.lede { max-width: 520px; margin: 22px 0 0; color: #aeb8ba; font-size: 16px; line-height: 1.75; }
.line-board { display: flex; align-items: center; max-width: 720px; margin-top: 25px; }
.line-node { display: flex; align-items: center; gap: 9px; color: #aeb8ba; }
.line-node span { display: grid; place-items: center; width: 28px; height: 28px; border: 1px solid #62747a; font-size: 11px; }
.line-node strong, .line-node small { display: block; }
.line-node strong { color: #dfe7e6; font-size: 12px; }
.line-node small { margin-top: 3px; font-size: 10px; }
.line-node.active span { color: #1b1e22; border-color: #2e8b65; background: #2e8b65; }
.line-track { flex: 1; min-width: 20px; height: 1px; margin: 0 14px; background: #53676d; }
.briefing-foot { display: flex; gap: 20px; color: #73848a; font-size: 11px; letter-spacing: .1em; }
.login-panel { display: grid; place-items: center; padding: 30px; background: rgba(255,253,248,.76); }
.form-wrap { width: min(100%, 420px); padding: 42px 38px 32px; border: 1px solid var(--bench-line); background: var(--bench-white); box-shadow: 18px 18px 0 rgba(23,100,125,.07); }
.form-heading { margin-bottom: 30px; }
.form-heading h2 { margin: 10px 0 10px; font-size: 31px; letter-spacing: -.035em; }
.form-heading p:last-child { margin: 0; color: var(--bench-ink-soft); font-size: 13px; line-height: 1.7; }
.login-options { display: flex; justify-content: space-between; margin: -4px 0 18px; }
.submit-button { width: 100%; height: 48px; }
.form-note { display: flex; align-items: center; gap: 8px; margin-top: 22px; color: var(--bench-ink-soft); font-size: 11px; }
.form-note span { width: 6px; height: 6px; border-radius: 50%; background: var(--bench-green); }
@media (max-width: 900px) { .login-screen { grid-template-columns: 1fr; } .briefing { min-height: 42vh; padding: 28px 24px; } .briefing-copy { margin: 42px 0 18px; } .briefing-copy h1 { font-size: 42px; } .line-board, .briefing-foot { display: none; } .login-panel { padding: 24px 16px 38px; } .form-wrap { padding: 28px 22px 24px; } }
</style>
