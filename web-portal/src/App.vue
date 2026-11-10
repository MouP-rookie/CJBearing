<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'

type PortalSystem = {
  code: string
  name: string
  description: string
  entryPath: string
  entryPort: number
  healthPath: string
  accent: string
  icon: string
  enabled: boolean
  sortOrder: number
  allowed: boolean
}
type PortalUser = {
  id: number
  account: string
  name: string
  role: { code: string; name: string; allowPc: boolean; allowPda: boolean }
  department?: { name: string } | null
  enabled: boolean
}
type AdminUser = {
  id: number
  account: string
  name: string
  enabled: boolean
  roleCode: string
  roleName: string
  department: string
  systemCodes: string[]
  isSuperAdmin: boolean
}
type RoleSummary = {
  id: number
  code: string
  name: string
  isSystem: boolean
  allowPc: boolean
  allowPda: boolean
  permissionCodes: string[]
}
type PermissionDefinition = { code: string; name: string; group: string }

const loading = ref(true)
const error = ref('')
const notice = ref('')
const account = ref('')
const password = ref('')
const user = ref<PortalUser | null>(null)
const systems = ref<PortalSystem[]>([])
const isSuperAdmin = ref(false)
const page = ref<'home' | 'access' | 'systems' | 'permissions'>('home')
const adminUsers = ref<AdminUser[]>([])
const adminSystems = ref<PortalSystem[]>([])
const roles = ref<RoleSummary[]>([])
const catalog = ref<PermissionDefinition[]>([])
const searchText = ref('')
const accessSaving = ref<number | null>(null)
const roleSaving = ref<number | null>(null)
const loginLoading = ref(false)
const loggingOut = ref(false)
let portalRequestID = 0
const accessDraft = reactive<Record<number, string[]>>({})
const roleDraft = reactive<Record<number, string[]>>({})

const visibleSystems = computed(() => systems.value.filter(system => system.allowed))
const filteredUsers = computed(() => {
  const keyword = searchText.value.trim().toLowerCase()
  if (!keyword) return adminUsers.value
  return adminUsers.value.filter(item =>
    [item.account, item.name, item.roleName, item.department]
      .some(value => value.toLowerCase().includes(keyword))
  )
})
const permissionGroups = computed(() => {
  const groups: Record<string, PermissionDefinition[]> = {}
  for (const item of catalog.value) {
    ;(groups[item.group] ||= []).push(item)
  }
  return groups
})

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const response = await fetch(path, {
    credentials: 'same-origin',
    cache: 'no-store',
    ...options,
    headers: {
      ...(options.body ? { 'Content-Type': 'application/json' } : {}),
      ...(options.headers || {})
    }
  })
  if (response.status === 204) return undefined as T
  const text = await response.text()
  let body: any = null
  try { body = text ? JSON.parse(text) : null } catch { body = text }
  if (!response.ok) {
    const message = body?.message || body?.error || `HTTP ${response.status}`
    const failure = new Error(message) as Error & { status?: number }
    failure.status = response.status
    throw failure
  }
  return body as T
}

function clearSession() {
  user.value = null
  systems.value = []
  isSuperAdmin.value = false
  adminUsers.value = []
  adminSystems.value = []
  roles.value = []
  catalog.value = []
  for (const key of Object.keys(accessDraft)) delete accessDraft[Number(key)]
  for (const key of Object.keys(roleDraft)) delete roleDraft[Number(key)]
  page.value = 'home'
  notice.value = ''
}

async function loadPortal() {
  const requestID = ++portalRequestID
  loading.value = true
  error.value = ''
  try {
    const data = await request<{ systems: PortalSystem[]; user: PortalUser; isSuperAdmin: boolean }>('/api/v1/portal/systems')
    if (requestID !== portalRequestID) return false
    if (user.value && user.value.id !== data.user.id) clearSession()
    systems.value = data.systems
    user.value = data.user
    isSuperAdmin.value = data.isSuperAdmin
    return true
  } catch (err: any) {
    if (requestID !== portalRequestID) return false
    if (err.status === 401) clearSession()
    else error.value = err.message || '门户加载失败'
    return false
  } finally {
    if (requestID === portalRequestID) loading.value = false
  }
}

async function login() {
  if (!account.value.trim() || !password.value) {
    error.value = '请输入账号和密码'
    return
  }
  loginLoading.value = true
  error.value = ''
  try {
    await request('/api/v1/auth/portal/login', {
      method: 'POST',
      body: JSON.stringify({ account: account.value.trim(), password: password.value })
    })
    password.value = ''
    await loadPortal()
  } catch (err: any) {
    error.value = err.message || '登录失败'
  } finally {
    loginLoading.value = false
  }
}

async function logout() {
  if (loggingOut.value) return
  loggingOut.value = true
  ++portalRequestID
  error.value = ''
  try {
    await request('/api/v1/auth/logout', { method: 'POST' })
    clearSession()
  } catch (err: any) {
    if (err.status === 401) clearSession()
    else error.value = err.message || '退出失败，请重试'
  } finally {
    loading.value = false
    loggingOut.value = false
  }
}

function systemURL(system: PortalSystem, destination?: string) {
  const protocol = location.protocol === 'https:' ? 'https:' : 'http:'
  const inspection = system.code === 'bearing_inspection'
  const entry = inspection && system.entryPath === '/pc/' && !user.value?.role.allowPc && user.value?.role.allowPda
    ? '/pda/'
    : system.entryPath
  const path = destination || entry
  const port = import.meta.env.DEV && inspection && system.entryPort === 8090
    ? (path.startsWith('/pda/') ? 5174 : 5173)
    : system.entryPort
  return new URL(path, `${protocol}//${location.hostname}:${port}`).href
}

async function openSystem(system: PortalSystem, path?: string) {
  if (loggingOut.value || !await loadPortal()) return
  const current = systems.value.find(item => item.code === system.code)
  if (!current?.allowed) return
  location.href = systemURL(current, path)
}

const accountManagementSystem = computed(() => user.value?.role.allowPc
  ? visibleSystems.value.find(system => system.code === 'bearing_inspection' && system.enabled)
  : undefined)

async function loadAdmin() {
  if (!isSuperAdmin.value) return
  error.value = ''
  try {
    const [userData, systemData, roleData] = await Promise.all([
      request<{ items: AdminUser[] }>('/api/v1/portal/admin/users'),
      request<{ systems: PortalSystem[] }>('/api/v1/portal/admin/systems'),
      request<{ items: RoleSummary[]; catalog: PermissionDefinition[] }>('/api/v1/portal/admin/roles')
    ])
    adminUsers.value = userData.items
    adminSystems.value = systemData.systems
    roles.value = roleData.items
    catalog.value = roleData.catalog
    for (const item of adminUsers.value) {
      accessDraft[item.id] = Array.isArray(item.systemCodes) ? [...item.systemCodes] : []
    }
    for (const role of roles.value) roleDraft[role.id] = [...role.permissionCodes]
  } catch (err: any) {
    error.value = err.message || '管理数据加载失败'
  }
}

async function switchPage(next: typeof page.value) {
  page.value = next
  if (next !== 'home') await loadAdmin()
}

function hasSystem(userID: number, code: string) {
  return (accessDraft[userID] || []).includes(code)
}

function toggleSystem(userID: number, code: string) {
  const current = accessDraft[userID] || []
  accessDraft[userID] = current.includes(code)
    ? current.filter(item => item !== code)
    : [...current, code]
}

async function saveAccess(item: AdminUser) {
  accessSaving.value = item.id
  try {
    await request(`/api/v1/portal/admin/users/${item.id}/access`, {
      method: 'PUT',
      body: JSON.stringify({ systemCodes: accessDraft[item.id] || [] })
    })
    item.systemCodes = [...(accessDraft[item.id] || [])]
    notice.value = `已更新 ${item.name} 的系统查看权限`
  } catch (err: any) {
    error.value = err.message || '保存系统权限失败'
  } finally {
    accessSaving.value = null
  }
}

function togglePermission(role: RoleSummary, code: string) {
  if (role.code === 'super_admin') return
  const current = roleDraft[role.id] || []
  roleDraft[role.id] = current.includes(code)
    ? current.filter(item => item !== code)
    : [...current, code]
}

function hasPermission(role: RoleSummary, code: string) {
  return (roleDraft[role.id] || []).includes(code)
}

async function saveRole(role: RoleSummary) {
  roleSaving.value = role.id
  try {
    await request(`/api/v1/portal/admin/roles/${role.id}/permissions`, {
      method: 'PUT',
      body: JSON.stringify({ permissionCodes: roleDraft[role.id] || [] })
    })
    role.permissionCodes = [...(roleDraft[role.id] || [])]
    notice.value = `已更新“${role.name}”的子系统权限`
  } catch (err: any) {
    error.value = err.message || '保存子系统权限失败'
  } finally {
    roleSaving.value = null
  }
}

function iconText(type: string) {
  if (type === 'inspection') return '检'
  if (type === 'environment') return '环'
  return '系'
}

// 浏览器返回可能恢复整个旧页面，必须重新读取登录状态和系统入口。
function restorePortal(event: PageTransitionEvent) {
  if (event.persisted && !loggingOut.value) void loadPortal()
}
onMounted(() => {
  window.addEventListener('pageshow', restorePortal)
  void loadPortal()
})
onUnmounted(() => window.removeEventListener('pageshow', restorePortal))
</script>

<template>
  <div class="portal-shell">
    <header class="topbar">
      <div class="brand-lockup">
        <div class="brand-mark"><i></i><i></i><i></i></div>
        <div>
          <strong>长江轴承东港分厂</strong>
          <span>UNIFIED OPERATIONS PORTAL</span>
        </div>
      </div>
      <nav v-if="user" class="topnav">
        <button :class="{ active: page === 'home' }" @click="switchPage('home')">应用中心</button>
        <template v-if="isSuperAdmin">
          <button v-if="accountManagementSystem" @click="openSystem(accountManagementSystem, '/pc/#/users')">用户管理</button>
          <button :class="{ active: page === 'access' }" @click="switchPage('access')">系统权限</button>
          <button :class="{ active: page === 'permissions' }" @click="switchPage('permissions')">子系统权限</button>
          <button :class="{ active: page === 'systems' }" @click="switchPage('systems')">系统注册</button>
        </template>
      </nav>
      <div v-if="user" class="account">
        <span class="account-avatar">{{ user.name.slice(0, 1) }}</span>
        <div><strong>{{ user.name }}</strong><small>{{ user.role.name }}</small></div>
        <button :disabled="loggingOut" @click="logout">{{ loggingOut ? '退出中…' : '退出' }}</button>
      </div>
    </header>

    <main v-if="loading" class="loading-state">正在加载统一门户…</main>

    <main v-else-if="!user" class="login-stage">
      <section class="login-story">
        <p class="eyebrow">统一身份与分级授权</p>
        <h1>一个入口，<br />连接每一道管理流程。</h1>
        <p>统一身份、统一入口、分级授权。门户控制系统访问范围，各子系统继续管理内部功能权限。</p>
        <div class="story-flow">
          <span>统一登录</span><i></i><span>系统授权</span><i></i><span>业务操作</span>
        </div>
      </section>
      <section class="login-panel">
        <div class="login-card">
          <p class="eyebrow">长江轴承东港分厂</p>
          <h2>登录统一门户</h2>
          <p class="login-note">使用现有系统账号登录，系统会根据门户授权显示可访问的应用。</p>
          <label>账号 / 工号<input v-model="account" autocomplete="username" placeholder="请输入账号" @keyup.enter="login" /></label>
          <label>密码<input v-model="password" type="password" autocomplete="current-password" placeholder="请输入密码" @keyup.enter="login" /></label>
          <button class="primary-button" :disabled="loginLoading" @click="login">{{ loginLoading ? '登录中…' : '进入门户' }}</button>
        </div>
      </section>
    </main>

    <main v-else class="workspace">
      <div v-if="error" class="alert">{{ error }}</div>
      <div v-if="notice" class="toast">{{ notice }}</div>

      <template v-if="page === 'home'">
        <section class="page-heading">
          <div>
            <p class="eyebrow">统一入口</p>
            <h1>统一业务门户</h1>
            <p>当前账号可访问 {{ visibleSystems.length }} 个系统。跨系统权限由门户统一控制。</p>
          </div>
          <div class="status-chip"><i></i> 内网运行</div>
        </section>
        <section class="system-grid">
          <article
            v-for="system in visibleSystems"
            :key="system.code"
            class="system-card"
            :style="{ '--system-accent': system.accent }"
            role="button"
            tabindex="0"
            @click="openSystem(system)"
            @keydown.enter="openSystem(system)"
            @keydown.space.prevent="openSystem(system)"
          >
            <div class="system-icon">{{ iconText(system.icon) }}</div>
            <div class="system-copy">
              <small>{{ system.code.toUpperCase() }}</small>
              <h2>{{ system.name }}</h2>
              <p>{{ system.description }}</p>
            </div>
            <span class="enter-link">打开系统</span>
          </article>
          <article v-if="!visibleSystems.length" class="empty-system">
            <h2>暂无可访问系统</h2>
            <p>请联系超级管理员在“系统权限”中授予查看权限。</p>
          </article>
        </section>
        <section class="authorization-note">
          <div><span>门户层</span><strong>系统查看权限</strong><p>决定用户是否能进入某个系统。</p></div>
          <div><span>系统层</span><strong>内部功能权限</strong><p>进入系统后，由角色继续决定可查看和执行的功能。</p></div>
          <div><span>全局层</span><strong>唯一超级管理员</strong><p>全局超级管理员由数据库约束保证只存在一个。</p></div>
        </section>
      </template>

      <template v-else-if="page === 'access'">
        <section class="page-heading">
          <div>
            <p class="eyebrow">门户层授权</p>
            <h1>系统查看权限</h1>
            <p>新增账户请前往轴承首末检 PC 端的“用户与权限”，创建后可在这里分配系统查看权限。</p>
            <button v-if="accountManagementSystem" class="small-button" @click="openSystem(accountManagementSystem, '/pc/#/users')">前往用户管理</button>
          </div>
          <input v-model="searchText" class="search-input" placeholder="搜索账号、姓名、角色或部门" />
        </section>
        <section class="admin-table panel">
          <table>
            <thead><tr><th>用户</th><th>角色 / 部门</th><th v-for="system in adminSystems" :key="system.code">{{ system.name }}</th><th>操作</th></tr></thead>
            <tbody>
              <tr v-for="item in filteredUsers" :key="item.id">
                <td><strong>{{ item.name }}</strong><small>{{ item.account }}</small></td>
                <td>{{ item.roleName }}<small>{{ item.department }}</small></td>
                <td v-for="system in adminSystems" :key="system.code">
                  <input v-if="!item.isSuperAdmin" type="checkbox" :checked="hasSystem(item.id, system.code)" @change="toggleSystem(item.id, system.code)" />
                  <span v-else class="fixed-grant">全部</span>
                </td>
                <td><button v-if="!item.isSuperAdmin" class="small-button" :disabled="accessSaving === item.id" @click="saveAccess(item)">{{ accessSaving === item.id ? '保存中…' : '保存' }}</button></td>
              </tr>
            </tbody>
          </table>
        </section>
      </template>

      <template v-else-if="page === 'permissions'">
        <section class="page-heading">
          <div><p class="eyebrow">系统层授权</p><h1>子系统内部权限</h1><p>门户授权解决“能否进入”，角色权限解决“进入后能做什么”。</p></div>
        </section>
        <section class="role-stack">
          <article v-for="role in roles" :key="role.id" class="role-card panel">
            <header>
              <div><span class="role-code">{{ role.code }}</span><h2>{{ role.name }}</h2><p>{{ role.allowPc ? '允许 PC' : '仅 PDA' }} · {{ role.permissionCodes.length }} 项权限</p></div>
              <button v-if="role.code !== 'super_admin'" class="primary-button compact" :disabled="roleSaving === role.id" @click="saveRole(role)">{{ roleSaving === role.id ? '保存中…' : '保存权限' }}</button>
            </header>
            <div v-if="role.code === 'super_admin'" class="locked-role">超级管理员固定拥有全部权限，不参与编辑。</div>
            <div v-else class="permission-groups">
              <section v-for="(items, group) in permissionGroups" :key="group">
                <h3>{{ group }}</h3>
                <label v-for="permission in items" :key="permission.code" class="permission-option">
                  <input type="checkbox" :checked="hasPermission(role, permission.code)" @change="togglePermission(role, permission.code)" />
                  <span><strong>{{ permission.name }}</strong><small>{{ permission.code }}</small></span>
                </label>
              </section>
            </div>
          </article>
        </section>
      </template>

      <template v-else-if="page === 'systems'">
        <section class="page-heading">
          <div><p class="eyebrow">可扩展接入</p><h1>系统注册</h1><p>后续系统通过注册表接入，门户只关联入口地址、端口和权限，不绑定具体业务代码。</p></div>
        </section>
        <section class="registry-grid">
          <article v-for="system in adminSystems" :key="system.code" class="registry-card panel" :style="{ '--system-accent': system.accent }">
            <div class="registry-head"><span class="system-icon small">{{ iconText(system.icon) }}</span><div><small>{{ system.code }}</small><h2>{{ system.name }}</h2></div></div>
            <p>{{ system.description }}</p>
            <dl><dt>入口</dt><dd>{{ system.entryPath }}</dd><dt>端口</dt><dd>{{ system.entryPort }}</dd><dt>状态</dt><dd>{{ system.enabled ? '已启用' : '已停用' }}</dd></dl>
          </article>
        </section>
      </template>
    </main>
  </div>
</template>
