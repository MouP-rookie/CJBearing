<template>
  <div class="page roles-page">
    <p class="section-label">系统管理</p>
    <h2 class="page-title">角色内部权限</h2>
    <p class="page-desc">角色决定进入系统后可以查看和执行的功能。门户另外控制账号能否进入首末检和 QMS 系统。</p>

    <div class="role-stack">
      <section v-for="role in roles" :key="role.id" class="panel role-panel">
        <header>
          <div>
            <span class="role-code">{{ role.code }}</span>
            <h3>{{ role.name }}</h3>
            <p>{{ role.allowPc ? '允许登录 PC' : '仅允许 PDA' }} · {{ role.permissionCodes.length }} 项权限</p>
          </div>
          <el-button v-if="role.code !== 'super_admin'" type="primary" :loading="saving === role.id" @click="save(role)">保存权限</el-button>
        </header>
        <el-alert v-if="role.code === 'super_admin'" title="超级管理员固定拥有全部权限，由数据库保证全局只存在一个。" type="success" :closable="false" />
        <div v-else class="permission-grid">
          <section v-for="(items, group) in permissionGroups" :key="group">
            <h4>{{ group }}</h4>
            <el-checkbox
              v-for="permission in items"
              :key="permission.code"
              :model-value="has(role, permission.code)"
              @change="toggle(role, permission.code)"
            >
              <span>{{ permission.name }}</span>
              <small>{{ permission.code }}</small>
            </el-checkbox>
          </section>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
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
const saving = ref<number | null>(null)
const drafts = reactive<Record<number, string[]>>({})
const permissionGroups = computed(() => {
  const groups: Record<string, Permission[]> = {}
  for (const item of catalog.value) (groups[item.group] ||= []).push(item)
  return groups
})

function has(role: Role, code: string) {
  return (drafts[role.id] || []).includes(code)
}
function toggle(role: Role, code: string) {
  const current = drafts[role.id] || []
  drafts[role.id] = current.includes(code)
    ? current.filter(item => item !== code)
    : [...current, code]
}
async function load() {
  try {
    const { data } = await api.get('/portal/admin/roles')
    roles.value = data.items
    catalog.value = data.catalog
    for (const role of roles.value) drafts[role.id] = [...role.permissionCodes]
  } catch (error: any) {
    ElMessage.error(error.message || '读取角色权限失败')
  }
}
async function save(role: Role) {
  saving.value = role.id
  try {
    await api.put(`/portal/admin/roles/${role.id}/permissions`, { permissionCodes: drafts[role.id] || [] })
    role.permissionCodes = [...(drafts[role.id] || [])]
    ElMessage.success(`已更新“${role.name}”的权限`)
  } catch (error: any) {
    ElMessage.error(error.message || '保存权限失败')
  } finally {
    saving.value = null
  }
}
onMounted(load)
</script>

<style scoped>
.page-desc { max-width: 780px; margin: -8px 0 22px; color: var(--bench-ink-soft); font-size: 13px; line-height: 1.7; }
.role-stack { display: grid; gap: 16px; }
.role-panel { padding: 20px; }
.role-panel header { display: flex; align-items: center; justify-content: space-between; gap: 20px; padding-bottom: 15px; border-bottom: 1px solid var(--bench-line); }
.role-panel h3 { display: inline-block; margin: 0 0 0 9px; color: var(--bench-head); font-size: 18px; }
.role-panel header p { margin: 7px 0 0; color: var(--bench-ink-soft); font-size: 11px; }
.role-code { padding: 2px 7px; color: var(--bench-steel); background: var(--el-color-primary-light-9); font-size: 10px; }
.permission-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 16px; margin-top: 17px; }
.permission-grid > section { padding: 14px; border: 1px solid var(--bench-line); background: #fbfcfe; }
.permission-grid h4 { margin: 0 0 10px; color: var(--bench-head); font-size: 12px; }
.permission-grid :deep(.el-checkbox) { display: flex; align-items: flex-start; height: auto; min-height: 43px; margin-right: 0; white-space: normal; }
.permission-grid :deep(.el-checkbox__label) { line-height: 1.35; }
.permission-grid span, .permission-grid small { display: block; }
.permission-grid small { margin-top: 2px; color: var(--bench-ink-soft); font-size: 9px; }
@media (max-width: 1000px) { .permission-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 680px) { .permission-grid { grid-template-columns: 1fr; } .role-panel header { align-items: flex-start; } }
</style>
