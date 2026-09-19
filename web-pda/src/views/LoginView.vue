<!-- 文件说明：PDA 登录页，支持记住工号和自动登录。 -->

<template>
  <div class="login-page">
    <header class="login-hero">
      <div class="brand"><div class="mark"><i></i><i></i><i></i></div><div><strong>首末检</strong><small>PDA 现场终端</small></div></div>
      <div class="hero-copy"><p>质量过程控制</p><h1>从工位开始，<br />让检验有据可查。</h1></div>
    </header>

    <main class="login-body">
      <section class="login-card">
        <div class="card-title"><span>现场登录</span><h2>输入工号</h2><p>PDA 登录不需要密码，账号不存在时无法进入。</p></div>
        <van-field v-model="account" label="工号" placeholder="请扫描标签或输入工号" clearable @keyup.enter="submit()" />
        <div class="option-row"><span><strong>记住工号</strong><small>下次自动带出，不保存密码</small></span><van-switch v-model="rememberAccount" size="22px" /></div>
        <div class="option-row"><span><strong>自动登录</strong><small>打开页面后直接进入工作台</small></span><van-switch v-model="autoLogin" size="22px" @change="onAutoChange" /></div>
        <van-button block type="primary" size="large" :loading="loading" @click="submit()">进入工作台</van-button>
      </section>
      <p class="foot-note"><i></i>数据通过内网连接，提交后自动留痕</p>
    </main>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { showToast } from 'vant'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const STORAGE_KEY = 'bearing-inspection.pda-login'
const account = ref('')
const rememberAccount = ref(false)
const autoLogin = ref(false)
const loading = ref(false)
const auth = useAuthStore()
const router = useRouter()

// PDA 仅保存工号和自动登录偏好，不保存密码。
function save() { localStorage.setItem(STORAGE_KEY, JSON.stringify({ account: account.value.trim(), remember: rememberAccount.value, auto: autoLogin.value })) }
function clear() { localStorage.removeItem(STORAGE_KEY) }
function onAutoChange(value: boolean) { if (value) rememberAccount.value = true }
// 登录成功后写入 HttpOnly Cookie，并进入工作台。
async function submit(fromAuto = false) {
  if (!account.value.trim()) { showToast('请输入工号'); return }
  loading.value = true
  try { await auth.login(account.value.trim()); if (rememberAccount.value || autoLogin.value) save(); else clear(); router.replace('/') } catch (error: any) { if (fromAuto) { autoLogin.value = false; clear() }; showToast(error.message || '登录失败') } finally { loading.value = false }
}
onMounted(async () => { try { const saved = JSON.parse(localStorage.getItem(STORAGE_KEY) || 'null'); if (!saved) return; account.value = saved.account || ''; rememberAccount.value = saved.remember; autoLogin.value = saved.auto; if (saved.auto && saved.account) await submit(true) } catch { clear() } })
</script>

<style scoped>
.login-page { min-height: 100vh; background: var(--bench-paper); }
.login-hero { position: relative; min-height: 42vh; padding: 24px 20px 34px; overflow: hidden; color: #f7f3e9; background: #1b1e22; }
.login-hero::before { content: ""; position: absolute; inset: 0; opacity: .28; background: linear-gradient(rgba(112,167,181,.14) 1px, transparent 1px), linear-gradient(90deg, rgba(112,167,181,.14) 1px, transparent 1px); background-size: 26px 26px; }
.login-hero::after { content: ""; position: absolute; right: -100px; bottom: -150px; width: 280px; height: 280px; border: 36px solid #0d4b60; border-radius: 50%; }
.brand, .hero-copy { position: relative; z-index: 1; }
.brand { display: flex; align-items: center; gap: 11px; }
.brand strong, .brand small { display: block; }
.brand strong { font-size: 17px; }
.brand small { margin-top: 3px; color: #9ba8ab; font-size: 10px; letter-spacing: .08em; }
.mark { position: relative; width: 35px; height: 35px; border: 2px solid #70a7b5; border-radius: 50%; }
.mark i { position: absolute; inset: 7px; border: 1px solid #70a7b5; border-radius: 50%; }
.mark i:nth-child(2) { inset: 12px; background: var(--bench-green); border-color: var(--bench-green); }
.mark i:nth-child(3) { inset: 0; border-style: dashed; }
.hero-copy { margin-top: 58px; }
.hero-copy p { margin: 0 0 10px; color: #84b3bf; font-size: 11px; letter-spacing: .14em; }
.hero-copy h1 { margin: 0; font-size: 34px; line-height: 1.12; letter-spacing: -.05em; }
.login-body { padding: 0 14px 34px; margin-top: -18px; position: relative; z-index: 2; }
.login-card { padding: 22px 16px 18px; border: 1px solid var(--bench-line); background: #fffdf8; box-shadow: 0 18px 42px rgba(27,30,34,.1); }
.card-title { margin-bottom: 18px; }
.card-title span { color: var(--bench-steel); font-size: 11px; font-weight: 700; letter-spacing: .08em; }
.card-title h2 { margin: 6px 0 6px; font-size: 25px; letter-spacing: -.035em; }
.card-title p { margin: 0; color: var(--bench-ink-soft); font-size: 12px; line-height: 1.55; }
.login-card :deep(.van-field) { margin-bottom: 14px; border: 1px solid var(--bench-line); background: #fffefa; }
.option-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 13px 0; border-top: 1px solid var(--bench-line); }
.option-row strong, .option-row small { display: block; }
.option-row strong { font-size: 13px; }
.option-row small { margin-top: 4px; color: var(--bench-ink-soft); font-size: 11px; }
.login-card .van-button { margin-top: 18px; }
.foot-note { display: flex; align-items: center; justify-content: center; gap: 7px; margin: 18px 0 0; color: #68757b; font-size: 11px; }
.foot-note i { width: 6px; height: 6px; border-radius: 50%; background: var(--bench-green); }
</style>