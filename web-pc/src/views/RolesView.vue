<template>
  <div class="page roles-page">
    <p class="section-label">系统管理</p>
    <h2 class="page-title">角色内部权限</h2>
    <p class="page-desc">角色决定进入系统后可以查看和执行的功能。门户另外控制账号能否进入首末检和 QMS 系统。</p>

    <div v-loading="loading" class="role-grid">
      <button v-for="role in roles" :key="role.id" type="button" class="panel role-card" aria-haspopup="dialog" @click="openRole(role)">
        <span class="role-name">{{ role.name }}</span>
        <span class="role-code">{{ role.code }}</span>
        <span class="role-meta">{{ loginLabel(role) }} · {{ role.permissionCodes.length }} 项权限</span>
        <span class="role-action">{{ role.code === 'super_admin' ? '查看说明' : '分配权限' }}</span>
      </button>
    </div>
    <el-alert v-if="loadError" :title="loadError" type="error" :closable="false" show-icon />
    <el-empty v-else-if="!loading && !roles.length" description="暂无角色" />

    <el-dialog
      v-model="dialogOpen"
      :title="selectedRole ? selectedRole.name + ' · 权限' : '角色权限'"
      width="min(960px, 94vw)"
      top="4vh"
      destroy-on-close
      :close-on-click-modal="false"
      :close-on-press-escape="!saving"
      :show-close="!saving"
      @closed="clearDraft"
    >
      <template v-if="selectedRole">
        <el-alert v-if="readOnly" title="超级管理员固定拥有全部权限，由数据库保证全局只存在一个。" type="success" :closable="false" />
        <el-checkbox-group v-else v-model="draft" :disabled="saving" class="permission-grid">
          <section v-for="(items, group) in permissionGroups" :key="group">
            <h4>{{ group }}</h4>
            <el-checkbox
              v-for="permission in items"
              :key="permission.code"
              :value="permission.code"
            >
              <span>{{ permission.name }}</span>
              <small>{{ permission.code }}</small>
            </el-checkbox>
          </section>
        </el-checkbox-group>
      </template>
      <template #footer>
        <el-alert v-if="saveError" class="save-error" :title="saveError" type="error" :closable="false" show-icon />
        <el-button :disabled="saving" @click="dialogOpen = false">{{ readOnly ? '关闭' : '取消' }}</el-button>
        <el-button v-if="!readOnly" type="primary" :loading="saving" @click="save">保存权限</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '@/api'

type Role = {
  id: number
  code: string
  name: string
  allowPc: boolean
  allowPda: boolean
  permissionCodes: string[]
}
type Permission = { code: string; name: string; group: string }

const roles = ref<Role[]>([])
const catalog = ref<Permission[]>([])
const loading = ref(true)
const loadError = ref('')
const saving = ref(false)
const saveError = ref('')
const dialogOpen = ref(false)
const selectedRole = ref<Role | null>(null)
const draft = ref<string[]>([])
const readOnly = computed(() => selectedRole.value?.code === 'super_admin')
const permissionGroups = computed(() => {
  const groups: Record<string, Permission[]> = {}
  for (const item of catalog.value) (groups[item.group] ||= []).push(item)
  return groups
})

function loginLabel(role: Role) {
  if (role.allowPc && role.allowPda) return 'PC / PDA'
  if (role.allowPc) return '仅 PC'
  return role.allowPda ? '仅 PDA' : '未开放登录'
}

function openRole(role: Role) {
  selectedRole.value = role
  draft.value = [...role.permissionCodes]
  saveError.value = ''
  dialogOpen.value = true
}

function clearDraft() {
  if (dialogOpen.value) return
  selectedRole.value = null
  draft.value = []
  saveError.value = ''
}
async function load() {
  try {
    const { data } = await api.get('/portal/admin/roles')
    roles.value = data.items
    catalog.value = data.catalog
  } catch (error: any) {
    loadError.value = error.message || '读取角色权限失败'
  } finally {
    loading.value = false
  }
}

async function save() {
  const role = selectedRole.value
  if (!role || readOnly.value || saving.value) return
  const permissionCodes = [...draft.value]
  saving.value = true
  saveError.value = ''
  try {
    await api.put(`/portal/admin/roles/${role.id}/permissions`, { permissionCodes })
    role.permissionCodes = permissionCodes
    dialogOpen.value = false
    ElMessage.success(`已更新“${role.name}”的权限`)
  } catch (error: any) {
    saveError.value = error.message || '保存权限失败'
  } finally {
    saving.value = false
  }
}
onMounted(load)
</script>

<style scoped>
.page-desc { max-width: 780px; margin: -8px 0 22px; color: var(--bench-ink-soft); font-size: 13px; line-height: 1.7; }
.role-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(min(100%, 220px), 1fr)); gap: 12px; min-height: 48px; }
.role-card { display: grid; gap: 6px; justify-items: start; padding: 15px 16px; min-width: 0; text-align: left; cursor: pointer; overflow-wrap: anywhere; }
.role-card:hover { border-color: var(--el-color-primary); }
.role-card:focus-visible { outline: 2px solid var(--el-color-primary); outline-offset: 2px; }
.role-name { color: var(--bench-head); font-size: 17px; font-weight: 700; }
.role-code, .role-meta { color: var(--bench-ink-soft); font-size: 12px; }
.role-action { margin-top: 3px; color: var(--el-color-primary); font-size: 12px; font-weight: 600; }
.permission-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; max-height: min(65dvh, calc(100dvh - 225px)); min-height: 80px; overflow-y: auto; overscroll-behavior: contain; }
.permission-grid > section { min-width: 0; padding: 12px; border: 1px solid var(--bench-line); background: #fbfcfe; overflow-wrap: anywhere; }
.permission-grid h4 { margin: 0 0 10px; color: var(--bench-head); font-size: 12px; }
.permission-grid :deep(.el-checkbox) { display: flex; align-items: flex-start; height: auto; min-height: 43px; margin-right: 0; white-space: normal; }
.permission-grid :deep(.el-checkbox__label) { line-height: 1.35; }
.permission-grid span, .permission-grid small { display: block; }
.permission-grid small { margin-top: 2px; color: var(--bench-ink-soft); font-size: 9px; }
@media (max-width: 1000px) { .permission-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
.save-error { margin-bottom: 12px; text-align: left; }
@media (max-width: 680px) { .permission-grid { grid-template-columns: 1fr; } }
</style>
