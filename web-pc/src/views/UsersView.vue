<!-- 文件说明：用户与权限页，维护账号、角色、部门和批量导入。 -->

<template>
  <div class="page users-page">
    <div class="page-head">
      <div>
        <p class="section-label">系统管理</p>
        <h2 class="page-title">用户与权限</h2>
        <p class="page-desc">维护账号、角色和登录权限。批量导入使用 Excel 模板，导入用户默认密码为 123456。</p>
      </div>
      <div class="page-actions">
        <el-button :disabled="auth.allDepartments" @click="openImport">
          <el-icon><UploadFilled /></el-icon>
          批量导入
        </el-button>
        <el-button type="primary" :disabled="auth.allDepartments" @click="openCreate">新增用户</el-button>
      </div>
    </div>

    <div class="toolbar">
      <el-input v-model="account" placeholder="工号/账号" clearable style="width:180px" />
      <el-select v-model="roleId" placeholder="角色" clearable style="width:180px">
        <el-option v-for="r in roles" :key="r.id" :label="r.name" :value="r.id" />
      </el-select>
      <el-button type="primary" @click="search">查询</el-button>
    </div>

    <el-table :data="users" border>
      <el-table-column prop="account" label="工号" width="140" />
      <el-table-column prop="name" label="用户名" />
      <el-table-column label="部门" width="160">
        <template #default="{ row }">{{ row.department?.name || '全局' }}</template>
      </el-table-column>
      <el-table-column label="角色" width="160">
        <template #default="{ row }">{{ row.role?.name }}</template>
      </el-table-column>
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '启用' : '停用' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="330">
        <template #default="{ row }">
          <el-button link :disabled="row.id === auth.user?.id" @click="toggle(row)">{{ row.enabled ? '停用' : '启用' }}</el-button>
          <el-button link @click="reset(row)">重置密码</el-button>
          <el-button v-if="canChangeRole(row)" link type="danger" @click="changeRole(row)">角色/部门</el-button>
          <el-tag v-else-if="row.id === auth.user?.id" size="small" type="info">本人</el-tag>
        </template>
      </el-table-column>
    </el-table>
    <AppPagination v-model:page="page" v-model:size="size" :total="total" @change="load" />

    <el-dialog v-model="dialog" title="新增用户" width="420px">
      <el-form label-width="80px">
        <el-form-item label="工号">
          <el-input v-model="form.account" placeholder="请输入工号" />
        </el-form-item>
        <el-form-item label="用户名">
          <el-input v-model="form.name" placeholder="请输入用户名" />
        </el-form-item>
        <el-form-item label="角色">
          <el-select v-model="form.roleId" style="width:100%" placeholder="请选择角色">
            <el-option v-for="r in roles" :key="r.id" :label="r.name" :value="r.id" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="auth.isSuperAdmin" label="部门">
          <el-select v-model="form.departmentId" style="width:100%" placeholder="请选择部门">
            <el-option v-for="department in auth.departments" :key="department.id" :label="department.name" :value="department.id" :disabled="!department.enabled" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialog = false">取消</el-button>
        <el-button type="primary" @click="create">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="roleDialog" title="修改角色与部门" width="460px">
      <div class="dialog-caption">用户：{{ roleTarget?.name }}（{{ roleTarget?.account }}）</div>
      <el-form label-width="80px">
        <el-form-item label="新角色">
          <el-select v-model="roleForm.roleId" style="width:100%" placeholder="请选择角色">
            <el-option v-for="r in roles" :key="r.id" :label="r.name" :value="r.id" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="auth.isSuperAdmin" label="所属部门">
          <el-select v-model="roleForm.departmentId" style="width:100%" placeholder="请选择部门" :disabled="selectedRoleIsSuper">
            <el-option v-for="department in auth.departments" :key="department.id" :label="department.name" :value="department.id" :disabled="!department.enabled" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="roleDialog = false">取消</el-button>
        <el-button type="primary" @click="saveRole">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="importDialog" title="Excel 批量导入用户" width="640px" @closed="resetImport">
      <el-alert
        type="info"
        :closable="false"
        show-icon
        title="请按模板填写账号信息，每行一个用户。角色可以填写角色名称或编码，不填写时默认使用普通用户。"
      />
      <div class="import-format">
        Excel 列顺序：<strong>工号</strong>、<strong>用户名</strong>、<strong>角色</strong><template v-if="auth.isSuperAdmin">、<strong>部门</strong></template>
      </div>
      <el-upload
        ref="uploadRef"
        drag
        :auto-upload="false"
        :limit="1"
        accept=".xlsx"
        :on-change="onImportFileChange"
        :on-remove="onImportFileRemove"
      >
        <el-icon class="upload-icon"><UploadFilled /></el-icon>
        <div class="upload-copy">拖拽 Excel 文件到此处，或点击选择文件</div>
        <template #tip>
          <div class="el-upload__tip">仅支持 .xlsx，导入用户默认密码为 123456</div>
        </template>
      </el-upload>
      <div class="template-link">
        <el-button link type="primary" @click="downloadTemplate">下载导入模板</el-button>
      </div>
      <el-table v-if="importErrors.length" :data="importErrors" border size="small" class="error-table">
        <el-table-column prop="row" label="Excel 行" width="90" />
        <el-table-column prop="account" label="工号" width="140" />
        <el-table-column prop="message" label="未导入原因" />
      </el-table>
      <template #footer>
        <el-button @click="importDialog = false">取消</el-button>
        <el-button type="primary" :loading="importing" @click="submitImport">开始导入</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { UploadFilled } from '@element-plus/icons-vue'
import { api } from '@/api'
import { useAuthStore } from '@/stores/auth'
import AppPagination from '@/components/AppPagination.vue'

const auth = useAuthStore()
const users = ref<any[]>([])
const page = ref(1)
const size = ref(20)
const total = ref(0)
const roles = ref<any[]>([])
const account = ref('')
const roleId = ref<number>()
const dialog = ref(false)
const roleDialog = ref(false)
const importDialog = ref(false)
const roleTarget = ref<any>(null)
const importFile = ref<File | null>(null)
const importErrors = ref<any[]>([])
const importing = ref(false)
const uploadRef = ref<any>()

const form = reactive({ account: '', name: '', roleId: 0, departmentId: 0 })
const roleForm = reactive({ roleId: 0, departmentId: 0 })
const selectedRoleIsSuper = computed(() => roles.value.find(item => item.id === roleForm.roleId)?.code === 'super_admin')

async function load() {
  try {
    const { data } = await api.get('/admin/users', { params: { account: account.value, roleId: roleId.value || undefined, page: page.value, size: size.value } })
    if (!data.items?.length && page.value > 1) { page.value -= 1; return load() }
    users.value = data.items
    total.value = data.total || 0
  } catch (error: any) {
    ElMessage.error(error.message || '查询用户失败')
  }
}

function search() { page.value = 1; load() }

async function loadRoles() {
  try {
    const { data } = await api.get('/admin/role-options')
    roles.value = data.items
  } catch (error: any) {
    ElMessage.error(error.message || '查询角色失败')
  }
}

function openCreate() {
  form.account = ''
  form.name = ''
  form.roleId = roles.value.find(r => r.code === 'normal')?.id || roles.value[0]?.id || 0
  form.departmentId = auth.selectedDepartmentId || auth.departments.find(item => item.enabled)?.id || 0
  dialog.value = true
}

// 新建用户使用后端默认初始密码，并校验角色和部门范围。
async function create() {
  if (!form.account.trim() || !form.name.trim() || !form.roleId || (auth.isSuperAdmin && !form.departmentId && !selectedRoleIsSuper.value)) {
    ElMessage.warning('请填写工号、用户名、角色和部门')
    return
  }
  try {
    await api.post('/admin/users', form)
    dialog.value = false
    ElMessage.success('已创建，默认密码 123456')
    await load()
  } catch (error: any) {
    ElMessage.error(error.message || '创建用户失败')
  }
}

async function toggle(row: any) {
  if (row.id === auth.user?.id) {
    ElMessage.warning('不能停用当前登录账号')
    return
  }
  try {
    await api.patch(`/admin/users/${row.id}`, { enabled: !row.enabled })
    await load()
  } catch (error: any) {
    ElMessage.error(error.message || '修改用户状态失败')
  }
}

async function reset(row: any) {
  try {
    await ElMessageBox.confirm(`确定重置 ${row.name} 的密码为 123456？`, '提示')
    await api.post(`/admin/users/${row.id}/password`, { password: '123456' })
    ElMessage.success('已重置')
  } catch (error: any) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error(error.message || '重置密码失败')
  }
}

function canChangeRole(row: any) {
  return row.id !== auth.user?.id
}

function changeRole(row: any) {
  roleTarget.value = row
  roleForm.roleId = row.roleId || row.role?.id || 0
  roleForm.departmentId = row.departmentId || row.department?.id || 0
  roleDialog.value = true
}

// 角色和部门变更会影响登录端和可见数据范围，保存后应通知用户重新登录。
async function saveRole() {
  if (!roleTarget.value || !roleForm.roleId) {
    ElMessage.warning('请选择角色')
    return
  }
  try {
    await api.patch(`/admin/users/${roleTarget.value.id}`, { roleId: roleForm.roleId, departmentId: auth.isSuperAdmin ? roleForm.departmentId : undefined })
    roleDialog.value = false
    ElMessage.success('角色已修改')
    await load()
  } catch (error: any) {
    ElMessage.error(error.message || '修改角色失败')
  }
}

function openImport() {
  importErrors.value = []
  importFile.value = null
  importDialog.value = true
}

function onImportFileChange(file: any) {
  importFile.value = file.raw || null
  importErrors.value = []
}

function onImportFileRemove() {
  importFile.value = null
  importErrors.value = []
}

function resetImport() {
  importFile.value = null
  importErrors.value = []
  uploadRef.value?.clearFiles()
}

async function downloadTemplate() {
  try {
    const response = await api.get('/admin/users/import-template', { responseType: 'blob' })
    const url = URL.createObjectURL(response.data)
    const link = document.createElement('a')
    link.href = url
    link.download = '用户批量导入模板.xlsx'
    link.click()
    URL.revokeObjectURL(url)
  } catch (error: any) {
    ElMessage.error(error.message || '下载模板失败')
  }
}

// 批量导入后逐行展示失败原因，成功记录由后端统一写入。
async function submitImport() {
  if (!importFile.value) {
    ElMessage.warning('请先选择 Excel 文件')
    return
  }
  importing.value = true
  try {
    const data = new FormData()
    data.append('file', importFile.value)
    const response = await api.post('/admin/users/import', data)
    importErrors.value = response.data.errors || []
    if (response.data.created) ElMessage.success(`成功导入 ${response.data.created} 个用户`)
    if (importErrors.value.length) {
      ElMessage.warning(`有 ${importErrors.value.length} 行未导入，请查看下方原因`)
      await load()
      return
    }
    await load()
    importDialog.value = false
  } catch (error: any) {
    ElMessage.error(error.message || '批量导入失败')
  } finally {
    importing.value = false
  }
}

onMounted(async () => {
  await loadRoles()
  await load()
})
</script>

<style scoped>
.page-head { display: flex; align-items: flex-end; justify-content: space-between; gap: 20px; }
.page-desc { max-width: 720px; margin: -8px 0 22px; color: var(--bench-ink-soft); font-size: 13px; line-height: 1.7; }
.page-actions { display: flex; align-items: center; gap: 10px; }
.dialog-caption { margin-bottom: 18px; padding: 10px 12px; color: var(--bench-ink-soft); background: #f7f4ed; font-size: 13px; }
.import-format { margin: 16px 0 12px; color: var(--bench-ink-soft); font-size: 13px; }
.import-format strong { color: var(--bench-ink); }
.upload-icon { margin-bottom: 8px; color: var(--bench-steel); font-size: 30px; }
.upload-copy { color: var(--bench-ink); font-size: 13px; }
.template-link { margin-top: 6px; text-align: right; }
.error-table { margin-top: 16px; }
@media (max-width: 800px) {
  .page-head { align-items: flex-start; flex-direction: column; }
  .page-actions { width: 100%; }
}
</style>
