<script setup lang="ts">
defineProps<{ message: string; forbidden: boolean }>()

const portalURL = new URL('/portal/', window.location.href)
if (['5173', '5174'].includes(portalURL.port)) portalURL.port = '5175'
</script>

<template>
  <section class="session-error" role="alert">
    <div class="session-error__panel">
      <h1>{{ forbidden ? '当前账号无权进入' : '暂时无法连接服务' }}</h1>
      <p>{{ message }}</p>
      <p v-if="forbidden">请联系管理员检查系统访问权限和角色权限。</p>
      <div class="session-error__actions">
        <a :href="portalURL.href">返回统一门户</a>
        <router-link to="/">重新检查</router-link>
      </div>
    </div>
  </section>
</template>

<style scoped>
.session-error { display: grid; place-items: center; min-height: 80vh; padding: 24px; }
.session-error__panel { width: min(100%, 520px); padding: 28px; border: 1px solid var(--bench-line); border-radius: 6px; background: #fff; }
h1 { margin: 0 0 16px; color: var(--bench-ink); font-size: 22px; }
p { color: #657381; font-size: 14px; line-height: 1.7; overflow-wrap: anywhere; }
.session-error__actions { display: flex; flex-wrap: wrap; gap: 12px; margin-top: 24px; }
a { padding: 10px 14px; border: 1px solid var(--bench-steel); border-radius: 3px; color: var(--bench-steel); font-size: 14px; text-decoration: none; }
a:first-child { color: #fff; background: var(--bench-steel); }
</style>
