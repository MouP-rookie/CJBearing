<!-- 文件说明：PC 工作台，展示当前用户、权限范围、服务状态和快捷入口。 -->

<template>
  <div class="page dashboard">
    <div class="dashboard-head">
      <div><p class="section-label">质量过程总览</p><h2 class="page-title">今天从一次首检开始</h2></div>
      <div class="shift-sign"><span>当前班次</span><strong>自动识别</strong></div>
    </div>

    <div class="metric-grid">
      <article class="metric-card"><p class="metric-label">当前用户</p><p class="metric-value">{{ auth.user?.name }}</p><p class="metric-note">{{ auth.user?.account }} · {{ auth.user?.role.name }}</p></article>
      <article class="metric-card green"><p class="metric-label">可访问产线</p><p class="metric-value">{{ auth.lineIds.length }}</p><p class="metric-note">按角色和产线范围授权</p></article>
      <article class="metric-card amber"><p class="metric-label">服务状态</p><p class="metric-value">在线</p><p class="metric-note">局域网服务正常，日志持续记录</p></article>
    </div>

    <div class="dashboard-grid">
      <section class="panel quick-panel">
        <div class="panel-head"><div><p class="section-label">快捷入口</p><h3>开始今天的工作</h3></div></div>
        <div class="quick-list">
          <button v-if="auth.can('form.view')" class="quick-item" @click="router.push('/forms')"><span class="quick-index">01</span><span><strong>配置任务与表单</strong><small>首末检、报表和登记表版本</small></span><el-icon><ArrowRight /></el-icon></button>
          <button v-if="auth.can('log.view')" class="quick-item" @click="router.push('/logs')"><span class="quick-index">02</span><span><strong>查看填报日志</strong><small>按产线、日期和版本追溯</small></span><el-icon><ArrowRight /></el-icon></button>
          <button v-if="auth.can('backup.manage')" class="quick-item" @click="router.push('/backups')"><span class="quick-index">03</span><span><strong>{{ auth.allDepartments ? '生成完整数据库备份' : '生成本部门数据归档' }}</strong><small>{{ auth.allDepartments ? '仅超级管理员可用' : '只包含当前部门数据' }}</small></span><el-icon><ArrowRight /></el-icon></button>
        </div>
      </section>

      <section class="panel status-panel">
        <div class="panel-head"><div><p class="section-label">工作原则</p><h3>先确认，再放行</h3></div></div>
        <div class="principle"><span>首件</span><strong>先测，再记</strong><p>电子仪表数据到达后，由人工确认是否采用。</p></div>
        <div class="principle"><span>过程</span><strong>谁填写，谁留痕</strong><p>每次提交保留人员、时间、来源和版本。</p></div>
        <div class="principle"><span>归档</span><strong>可查，可导出</strong><p>日志、审计和 Excel 分开保存。</p></div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router'
import { ArrowRight } from '@element-plus/icons-vue'
import { useAuthStore } from '@/stores/auth'
const auth = useAuthStore()
const router = useRouter()
</script>

<style scoped>
.dashboard-head { display: flex; align-items: flex-end; justify-content: space-between; gap: 20px; }
.dashboard-head .page-title { margin-bottom: 24px; }
.shift-sign { min-width: 150px; padding: 12px 14px; border-left: 3px solid var(--bench-green); background: rgba(255,253,248,.72); }
.shift-sign span, .shift-sign strong { display: block; }
.shift-sign span { color: var(--bench-ink-soft); font-size: 11px; }
.shift-sign strong { margin-top: 4px; font-size: 15px; }
.metric-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 16px; margin-bottom: 18px; }
.metric-label { margin: 0 0 19px; color: var(--bench-ink-soft); font-size: 12px; letter-spacing: .05em; }
.dashboard-grid { display: grid; grid-template-columns: 1.25fr .75fr; gap: 18px; }
.panel-head { display: flex; align-items: center; justify-content: space-between; padding: 18px 20px 14px; border-bottom: 1px solid var(--bench-line); }
.panel-head h3 { margin: 6px 0 0; font-size: 20px; }
.quick-list { padding: 6px; }
.quick-item { display: grid; grid-template-columns: 36px 1fr auto; align-items: center; width: 100%; padding: 15px 14px; border: 0; border-bottom: 1px solid var(--bench-line); color: var(--bench-ink); background: transparent; text-align: left; cursor: pointer; }
.quick-item:last-child { border-bottom: 0; }
.quick-item:hover { background: #f5f8f6; }
.quick-index { color: var(--bench-steel); font-size: 13px; font-weight: 700; }
.quick-item strong, .quick-item small { display: block; }
.quick-item strong { font-size: 14px; }
.quick-item small { margin-top: 5px; color: var(--bench-ink-soft); font-size: 12px; }
.status-panel { padding-bottom: 12px; }
.principle { padding: 14px 20px; border-bottom: 1px solid var(--bench-line); }
.principle:last-child { border-bottom: 0; }
.principle span { color: var(--bench-green); font-size: 11px; font-weight: 700; }
.principle strong { display: block; margin-top: 5px; font-size: 14px; }
.principle p { margin: 5px 0 0; color: var(--bench-ink-soft); font-size: 12px; line-height: 1.6; }
@media (max-width: 900px) { .metric-grid, .dashboard-grid { grid-template-columns: 1fr; } .dashboard-head { align-items: flex-start; flex-direction: column; } }
</style>
