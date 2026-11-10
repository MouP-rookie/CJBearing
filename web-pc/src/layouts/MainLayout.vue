<!-- 文件说明：PC 主框架，负责侧边栏、部门切换、用户信息和页面容器。 -->

<template>
  <el-container class="app-shell">
    <el-header class="topbar">
      <div class="brand-block">
        <img v-if="brandLogo" :src="brandLogo" class="brand-logo" alt="Logo">
        <div v-else class="bearing-mark" aria-hidden="true"><i></i><i></i><i></i></div>
        <div class="brand-copy">
          <strong>轴承首末检系统</strong>
          <span>BEARING PROCESS INSPECTION</span>
        </div>
      </div>

      <nav class="topnav">
        <router-link to="/">工作台</router-link>
        <router-link v-if="auth.can('execution.fill')" to="/execution">PC 填报</router-link>
        <router-link v-if="auth.can('line.manage')" to="/lines">产线</router-link>
        <router-link v-if="auth.can('shift.manage')" to="/shifts">班次</router-link>
        <router-link v-if="auth.can('product.manage')" to="/products">产品</router-link>
        <router-link v-if="auth.can('form.view')" to="/forms">任务与表单</router-link>
        <router-link v-if="auth.can('form.manage')" to="/assignments">产线分配</router-link>
        <router-link v-if="auth.can('log.view')" to="/logs">日志与导出</router-link>
        <router-link v-if="auth.can('user.manage')" to="/users">用户与权限</router-link>
        <router-link v-if="auth.isSuperAdmin" to="/roles">角色内部权限</router-link>
        <router-link v-if="auth.isSuperAdmin" to="/departments">部门</router-link>
        <router-link v-if="auth.can('backup.manage')" to="/backups">备份</router-link>
        <router-link v-if="auth.can('system.manage')" to="/settings">系统设置</router-link>
      </nav>

      <div class="account-block">
        <el-select
          v-if="auth.isSuperAdmin"
          class="department-switch"
          :model-value="auth.selectedDepartmentId"
          :disabled="changingDepartment"
          @change="changeDepartment"
        >
          <el-option label="全部部门" :value="0" />
          <el-option v-for="department in auth.departments" :key="department.id" :label="department.enabled ? department.name : `${department.name}（已停用）`" :value="department.id" />
        </el-select>
        <div v-else class="department-label">{{ auth.currentDepartment?.name || '未分配部门' }}</div>
        <div class="account-copy">
          <strong>{{ auth.user?.name }}</strong>
          <span>{{ auth.user?.role.name }} · {{ auth.currentDepartment?.name || '全局' }}</span>
        </div>
        <div class="account-avatar">{{ auth.user?.name?.slice(0, 1) }}</div>
        <el-button text :icon="SwitchButton" @click="logout">退出</el-button>
      </div>
    </el-header>

    <div class="page-context">
      <p class="breadcrumb">质量过程控制 / {{ route.meta.title || '工作台' }}</p>
      <h1>{{ pageTitle }}</h1>
      <div v-if="auth.allDepartments && scopedPages.has(route.path)" class="department-notice">
        <span>当前查看全部部门。新增、编辑或提交前，请先选择操作部门。</span>
        <el-button type="primary" plain @click="auth.departmentPickerOpen = true">选择部门</el-button>
      </div>
    </div>
    <el-main class="main-stage"><router-view /></el-main>
    <el-dialog
      v-model="auth.departmentPickerOpen"
      title="选择操作部门"
      width="min(420px, 92vw)"
      :close-on-click-modal="!changingDepartment"
      :close-on-press-escape="!changingDepartment"
      :show-close="!changingDepartment"
      @closed="pendingDepartmentId = undefined"
    >
      <p>选定部门后会重新加载当前页面，请在切换后继续操作。</p>
      <el-select v-model="pendingDepartmentId" placeholder="请选择部门" style="width:100%" :disabled="changingDepartment">
        <el-option v-for="department in enabledDepartments" :key="department.id" :label="department.name" :value="department.id" />
      </el-select>
      <p v-if="!enabledDepartments.length">没有可用部门，请先在「部门」页面启用一个部门。</p>
      <template #footer>
        <el-button :disabled="changingDepartment" @click="auth.departmentPickerOpen = false">取消</el-button>
        <el-button type="primary" :disabled="!pendingDepartmentId" :loading="changingDepartment" @click="pendingDepartmentId && changeDepartment(pendingDepartmentId)">切换部门</el-button>
      </template>
    </el-dialog>
    <CopyrightFooter />
  </el-container>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { useRoute, useRouter } from 'vue-router'
import { SwitchButton } from '@element-plus/icons-vue'
import { useAuthStore } from '@/stores/auth'
import CopyrightFooter from '@/components/CopyrightFooter.vue'

// 菜单和部门切换均以 /me 返回的权限与部门范围为准。
const auth = useAuthStore()
const brandLogo = computed(() => auth.brandLogoDataUrl)
const route = useRoute()
const router = useRouter()
const scopedPages = new Set(['/shifts', '/products', '/forms', '/assignments', '/execution', '/logs'])
const enabledDepartments = computed(() => auth.departments.filter(department => department.enabled))
const pendingDepartmentId = ref<number>()
const changingDepartment = ref(false)
const pageTitle = computed(() => ({ '/': '工作台', '/users': '用户与权限', '/roles': '角色内部权限', '/departments': '部门维护', '/lines': '产线配置', '/shifts': '班次配置', '/products': '产品维护', '/forms': '任务与表单', '/logs': '日志与导出', '/backups': '数据库备份', '/void-reasons': '废除原因', '/settings': 'Logo 设置' }[route.path] || '工作台'))
// 切换部门后刷新页面，确保所有页面重新按新部门范围加载。
async function changeDepartment(value: number | string) {
  if (changingDepartment.value) return
  changingDepartment.value = true
  try {
    await auth.selectDepartment(Number(value))
    window.location.reload()
  } catch (error: any) {
    ElMessage.error(error.message || '切换部门失败')
    changingDepartment.value = false
  }
}
async function logout() { await auth.logout(); router.push('/login') }
</script>

<style scoped>
.app-shell { min-height: 100vh; background: transparent; }
.topbar { position: sticky; top: 0; z-index: 30; display: flex; flex-wrap: wrap; align-items: center; gap: 14px 22px; height: auto; min-height: 76px; padding: 13px 24px; color: #fff; background: linear-gradient(120deg, #16324f, #1c4b74); box-shadow: 0 8px 24px rgba(22,50,79,.16); }
.brand-block { display: flex; gap: 11px; align-items: center; min-width: 220px; }
.brand-copy strong { display: block; font-size: 17px; letter-spacing: .035em; }
.brand-copy span { display: block; margin-top: 4px; color: #9fb8ce; font-size: 9px; letter-spacing: .14em; }
.bearing-mark { position: relative; width: 38px; height: 38px; border: 2px solid #90c4cf; border-radius: 50%; }
.bearing-mark i { position: absolute; inset: 7px; border: 1px solid #70a7b5; border-radius: 50%; }
.bearing-mark i:nth-child(2) { inset: 13px; background: #2e8b65; border-color: #2e8b65; }
.bearing-mark i:nth-child(3) { inset: 0; border-style: dashed; transform: rotate(35deg); opacity: .65; }
.brand-logo { width: 38px; height: 38px; object-fit: contain; border-radius: 4px; background: rgba(255,255,255,.9); }
.topnav { display: flex; flex: 1; flex-wrap: wrap; gap: 4px; min-width: 300px; }
.topnav a { padding: 7px 10px; border: 1px solid transparent; border-radius: 3px; color: #cfe0ef; font-size: 12px; text-decoration: none; }
.topnav a:hover { border-color: rgba(255,255,255,.38); color: #fff; }
.topnav a.router-link-active { color: var(--bench-steel-deep); background: #fff; border-color: #fff; font-weight: 700; }
.account-block { display: flex; align-items: center; gap: 9px; margin-left: auto; }
.department-switch { width: 150px; }
.department-label { max-width: 150px; overflow: hidden; color: #d7e5ef; font-size: 11px; font-weight: 700; text-overflow: ellipsis; white-space: nowrap; }
.account-copy { text-align: right; }
.account-copy strong, .account-copy span { display: block; }
.account-copy strong { color: #fff; font-size: 12px; }
.account-copy span { margin-top: 3px; color: #adc4d7; font-size: 10px; }
.account-avatar { display: grid; place-items: center; width: 32px; height: 32px; border-radius: 50%; color: #fff; background: var(--bench-green); font-weight: 700; }
.account-block :deep(.el-button) { color: #d7e5ef; }
.page-context { padding: 22px 30px 0; }
.breadcrumb { margin: 0 0 5px; color: var(--bench-ink-soft); font-size: 11px; letter-spacing: .08em; }
.page-context h1 { margin: 0; color: var(--bench-head); font-size: 26px; letter-spacing: -.025em; }
.main-stage { flex: 1; min-width: 0; padding: 0; }
.department-notice { display: flex; flex-wrap: wrap; align-items: center; gap: 8px 16px; margin-top: 12px; font-size: 13px; }
@media (max-width: 1100px) { .topbar { align-items: flex-start; } .topnav { order: 3; width: 100%; flex-basis: 100%; overflow-x: auto; } .account-block { margin-left: auto; } }
@media (max-width: 700px) { .topbar { padding: 12px 16px; } .brand-copy span, .account-copy, .department-switch, .department-label { display: none; } .page-context { padding: 18px 16px 0; } }
</style>



