<!-- 文件说明：PC 页脚版权信息组件。 -->

<template>
  <footer class="copyright-footer" aria-label="软件著作权信息">
    <div class="copyright-identity">
      <span class="copyright-bearing" aria-hidden="true"></span>
      <div>
        <strong>长江轴承股份有限公司</strong>
        <span>技术中心 · 数智化组</span>
      </div>
    </div>
    <div class="copyright-legal">
      <span>软件著作权</span>
      <span class="copyright-year">© {{ currentYear }}</span>
    </div>
  </footer>
</template>

<script setup lang="ts">
const currentYear = new Date().getFullYear()
</script>

<style scoped>
.copyright-footer { grid-column: 1 / -1; display: flex; flex: 0 0 auto; align-items: center; justify-content: space-between; gap: 22px; padding: 14px 30px 16px; border-top: 1px solid var(--bench-line); color: var(--bench-ink-soft); background: rgba(243,240,232,.84); }
.copyright-identity { display: flex; align-items: center; gap: 10px; min-width: 0; }
.copyright-bearing { position: relative; flex: 0 0 auto; width: 24px; height: 24px; border: 1px solid #70a7b5; border-radius: 50%; }
.copyright-bearing::before { content: ""; position: absolute; inset: 5px; border: 1px solid #70a7b5; border-radius: 50%; }
.copyright-bearing::after { content: ""; position: absolute; inset: 8px; border-radius: 50%; background: var(--bench-green); }
.copyright-identity strong, .copyright-identity span { display: block; }
.copyright-identity strong { color: var(--bench-steel-deep); font-size: 11px; font-weight: 750; letter-spacing: .025em; }
.copyright-identity span { margin-top: 2px; color: var(--bench-ink-soft); font-size: 10px; letter-spacing: .04em; }
.copyright-legal { display: flex; flex: 0 0 auto; align-items: baseline; gap: 7px; font-size: 10px; letter-spacing: .035em; }
.copyright-year { color: #657176; font-weight: 700; font-variant-numeric: tabular-nums; }
@media (max-width: 900px) {
  .copyright-footer { align-items: center; flex-direction: column; gap: 8px; padding: 13px 18px 15px; text-align: center; }
  .copyright-legal { justify-content: center; }
}
</style>
