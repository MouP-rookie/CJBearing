<!-- 文件说明：PDA 根组件，提供页面容器和版权页脚。 -->

<template>
  <div class="pda-shell">
    <main class="pda-content"><router-view /></main>
    <footer class="copyright-footer" aria-label="软件著作权信息">
      <div class="copyright-identity">
        <span class="copyright-bearing" aria-hidden="true"></span>
        <div>
          <strong>长江轴承股份有限公司</strong>
          <span>技术中心 · 数智化组</span>
        </div>
      </div>
      <div class="copyright-legal">软件著作权 © {{ currentYear }}</div>
    </footer>
  </div>
</template>

<script setup lang="ts">
const currentYear = new Date().getFullYear()
</script>

<style scoped>
.pda-shell { display: flex; flex-direction: column; min-height: 100vh; }
.pda-content { flex: 1; min-width: 0; }
.copyright-footer { display: flex; flex: 0 0 auto; flex-direction: column; align-items: center; gap: 6px; padding: 11px 16px calc(11px + env(safe-area-inset-bottom)); border-top: 1px solid var(--bench-line); color: #707a7e; background: rgba(255,253,248,.9); text-align: center; }
.copyright-identity { display: flex; align-items: center; justify-content: center; gap: 8px; min-width: 0; }
.copyright-bearing { position: relative; flex: 0 0 auto; width: 19px; height: 19px; border: 1px solid #70a7b5; border-radius: 50%; }
.copyright-bearing::before { content: ""; position: absolute; inset: 4px; border: 1px solid #70a7b5; border-radius: 50%; }
.copyright-bearing::after { content: ""; position: absolute; inset: 7px; border-radius: 50%; background: var(--bench-green); }
.copyright-identity strong, .copyright-identity span { display: block; text-align: left; }
.copyright-identity strong { color: var(--bench-steel); font-size: 10px; font-weight: 750; letter-spacing: .025em; }
.copyright-identity span { margin-top: 1px; color: #707a7e; font-size: 9px; letter-spacing: .04em; }
.copyright-legal { color: #7b8589; font-size: 9px; letter-spacing: .035em; }
</style>
