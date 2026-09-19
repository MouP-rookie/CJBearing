<template>
  <el-container class="app-shell">
    <el-aside class="rail" width="236px">
      <div class="brand-block">
        <img v-if="brandLogo" :src="brandLogo" class="brand-logo" alt="Logo">
        <div v-else class="bearing-mark" aria-hidden="true"><i></i><i></i><i></i></div>
        <div>
          <strong>首末检系统</strong>
          <span>INSPECTION BENCH</span>
        </div>
      </div>

      <nav class="nav-groups">
        <div class="nav-group">
          <p>执行</p>
          <el-menu :default-active="$route.path" router>
            <el-menu-item index="/"><el-icon><DataBoard /></el-icon><span>工作台</span></el-menu-item>
            <el-menu-item v-if="auth.can('execution.fill')" index="/execution"><el-icon><EditPen /></el-icon><span>PC 填报</span></el-menu-item>
          </el-menu>
        </div>
        <div v-if="showConfig" class="nav-group">
          <p>配置</p>
          <el-menu :default-active="$route.path" router>
            <el-menu-item v-if="auth.can('line.manage')" index="/lines"><el-icon><Connection /></el-icon><span>产线配置</span></el-menu-item>
            <el-menu-item v-if="auth.can('shift.manage')" index="/shifts"><el-icon><Clock /></el-icon><span>班次配置</span></el-menu-item>
            <el-menu-item v-if="auth.can('product.manage')" index="/products"><el-icon><Goods /></el-icon><span>产品维护</span></el-menu-item>
            <el-menu-item v-if="auth.can('form.view')" index="/forms"><el-icon><Document /></el-icon><span>任务与表单</span></el-menu-item>
            <el-menu-item v-if="auth.can('form.manage')" index="/assignments"><el-icon><Share /></el-icon><span>产线分配</span></el-menu-item>
            <el-menu-item v-if="auth.can('system.manage') || auth.can('log.view')" index="/void-reasons"><el-icon><CircleCloseFilled /></el-icon><span>废除原因</span></el-menu-item>
          </el-menu>
        </div>
        <div class="nav-group">
          <p>记录</p>
          <el-menu :default-active="$route.path" router>
            <el-menu-item v-if="auth.can('log.view')" index="/logs"><el-icon><Calendar /></el-icon><span>日志与导出</span></el-menu-item>
          </el-menu>
        </div>
        <div v-if="showSystem" class="nav-group">
          <p>系统</p>
          <el-menu :default-active="$route.path" router>
            <el-menu-item v-if="auth.can('user.manage')" index="/users"><el-icon><User /></el-icon><span>用户与权限</span></el-menu-item>
            <el-menu-item v-if="auth.can('department.manage')" index="/departments"><el-icon><OfficeBuilding /></el-icon><span>部门维护</span></el-menu-item>
            <el-menu-item v-if="auth.can('backup.manage')" index="/backups"><el-icon><Download /></el-icon><span>数据库备份</span></el-menu-item>
            <el-menu-item v-if="auth.can('system.manage')" index="/settings"><el-icon><Picture /></el-icon><span>Logo 设置</span></el-menu-item>
          </el-menu>
        </div>
      </nav>

      <div class="rail-foot">
        <span class="signal-dot"></span>
        <div><strong>内网运行</strong><small>局域网 · 无纸化</small></div>
      </div>
    </el-aside>

    <el-container class="content-shell">
      <el-header class="topbar">
        <div>
          <p class="breadcrumb">质量过程控制 / {{ route.meta.title || '工作台' }}</p>
          <h1>{{ pageTitle }}</h1>
        </div>
        <div class="account-block">
          <el-select
            v-if="auth.isSuperAdmin"
            class="department-switch"
            :model-value="auth.selectedDepartmentId"
            @change="changeDepartment"
          >
            <el-option label="全部部门" :value="0" />
            <el-option v-for="department in auth.departments" :key="department.id" :label="department.enabled ? department.name : `${department.name}（已停用）`" :value="department.id" />
          </el-select>
          <div v-else class="department-label">{{ auth.currentDepartment?.name || '未分配部门' }}</div>
          <div class="account-copy"><strong>{{ auth.user?.name }}</strong><span>{{ auth.user?.role.name }} · {{ auth.currentDepartment?.name || '全局' }}</span></div>
          <div class="account-avatar">{{ auth.user?.name?.slice(0, 1) }}</div>
          <el-button text :icon="SwitchButton" @click="logout">退出</el-button>
        </div>
      </el-header>
      <el-main class="main-stage"><router-view /></el-main>
      <CopyrightFooter />
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Calendar, Clock, Connection, DataBoard, Document, Download, EditPen, CircleCloseFilled, Goods, OfficeBuilding, Picture, Share, SwitchButton, User } from '@element-plus/icons-vue'
import { useAuthStore } from '@/stores/auth'
import CopyrightFooter from '@/components/CopyrightFooter.vue'

const auth = useAuthStore()
const brandLogo = computed(() => auth.brandLogoDataUrl)
const route = useRoute()
const router = useRouter()
const pageTitle = computed(() => ({ '/': '工作台', '/users': '用户与权限', '/departments': '部门维护', '/lines': '产线配置', '/shifts': '班次配置', '/products': '产品维护', '/forms': '任务与表单', '/logs': '日志与导出', '/backups': '数据库备份', '/void-reasons': '废除原因', '/settings': 'Logo 设置' }[route.path] || '工作台'))
const showConfig = computed(() => auth.can('line.manage') || auth.can('shift.manage') || auth.can('form.view') || auth.can('product.manage') || auth.can('system.manage') || auth.can('log.view'))
const showSystem = computed(() => auth.can('user.manage') || auth.can('department.manage') || auth.can('backup.manage') || auth.can('system.manage'))
async function changeDepartment(value: number | string) { await auth.selectDepartment(Number(value)); window.location.reload() }
async function logout() { await auth.logout(); router.push('/login') }
</script>

<style scoped>
.app-shell { min-height: 100vh; background: transparent; }
.rail { position: relative; display: flex; flex-direction: column; min-height: 100vh; background: #1b1e22; color: #f7f3e9; overflow: hidden; }
.rail::after { content: ""; position: absolute; inset: 0 0 0 auto; width: 1px; background: rgba(255,255,255,.12); }
.brand-block { display: flex; gap: 13px; align-items: center; padding: 27px 22px 23px; border-bottom: 1px solid rgba(255,255,255,.12); }
.brand-block strong { display: block; font-size: 17px; letter-spacing: .02em; }
.brand-block span { display: block; margin-top: 5px; color: #9ea9ad; font-size: 10px; letter-spacing: .14em; }
.bearing-mark { position: relative; width: 36px; height: 36px; border: 2px solid #70a7b5; border-radius: 50%; }
.bearing-mark i { position: absolute; inset: 7px; border: 1px solid #70a7b5; border-radius: 50%; }
.bearing-mark i:nth-child(2) { inset: 13px; background: #2e8b65; border-color: #2e8b65; }
.bearing-mark i:nth-child(3) { inset: 0; border-style: dashed; transform: rotate(35deg); opacity: .65; }
.brand-logo { width: 36px; height: 36px; object-fit: contain; background: rgba(255,255,255,.9); border-radius: 5px; }
.nav-groups { flex: 1; padding: 15px 11px; }
.nav-group { margin-bottom: 17px; }
.nav-group > p { margin: 0 11px 7px; color: #7f8d92; font-size: 11px; letter-spacing: .12em; }
.rail :deep(.el-menu) { border-right: 0; background: transparent; }
.rail :deep(.el-menu-item) { height: 42px; margin: 3px 0; border-radius: 3px; color: #c5cdcf; font-size: 14px; }
.rail :deep(.el-menu-item:hover) { color: #fff; background: rgba(255,255,255,.075); }
.rail :deep(.el-menu-item.is-active) { color: #fff; background: #0d4b60; box-shadow: 3px 0 0 #c97718 inset; }
.rail-foot { display: flex; gap: 10px; align-items: center; margin: 16px 18px 22px; padding-top: 15px; border-top: 1px solid rgba(255,255,255,.12); color: #a8b3b5; font-size: 12px; }
.rail-foot strong, .rail-foot small { display: block; }
.rail-foot strong { color: #e6ecec; font-size: 12px; }
.rail-foot small { margin-top: 3px; font-size: 10px; }
.signal-dot { width: 8px; height: 8px; flex: 0 0 auto; border-radius: 50%; background: #2e8b65; box-shadow: 0 0 0 4px rgba(46,139,101,.14); }
.content-shell { min-width: 0; min-height: 100vh; }
.topbar { display: flex; align-items: center; justify-content: space-between; min-height: 83px; padding: 15px 30px; border-bottom: 1px solid var(--bench-line); background: rgba(243,240,232,.92); backdrop-filter: blur(12px); }
.breadcrumb { margin: 0 0 5px; color: var(--bench-ink-soft); font-size: 11px; letter-spacing: .08em; }
.topbar h1 { margin: 0; font-size: 24px; letter-spacing: -.02em; }
.account-block { display: flex; align-items: center; gap: 10px; }
.department-switch { width: 180px; }
.department-label { max-width: 180px; overflow: hidden; color: var(--bench-steel); font-size: 12px; font-weight: 700; text-overflow: ellipsis; white-space: nowrap; }
.account-copy { text-align: right; }
.account-copy strong, .account-copy span { display: block; }
.account-copy strong { font-size: 13px; }
.account-copy span { margin-top: 3px; color: var(--bench-ink-soft); font-size: 11px; }
.account-avatar { display: grid; place-items: center; width: 34px; height: 34px; border-radius: 50%; color: #fff; background: var(--bench-steel); font-weight: 700; }
.main-stage { flex: 1; min-width: 0; padding: 0; }
@media (max-width: 900px) { .rail { width: 74px !important; } .brand-block { padding: 23px 18px; } .brand-block > div:last-child, .nav-group > p, .rail :deep(.el-menu-item span), .rail-foot div { display: none; } .rail :deep(.el-menu-item) { justify-content: center; padding: 0 !important; } .topbar { padding: 15px 18px; } .account-copy { display: none; } }
</style>



