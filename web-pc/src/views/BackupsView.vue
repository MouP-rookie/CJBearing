<!-- 文件说明：数据库备份页，生成和下载完整备份或部门归档，并支持部门 ZIP 恢复。 -->

<template>
  <div class="page">
    <h2 class="page-title">数据库备份</h2>
    <p class="page-desc">完整数据库备份仅超级管理员可生成；部门归档只包含当前部门数据，并包含密码哈希，请妥善保管。</p>
    <div class="toolbar">
      <el-button type="primary" @click="create">{{ auth.allDepartments ? '生成完整数据库备份' : '生成本部门归档' }}</el-button>
      <el-button v-if="auth.isSuperAdmin" type="danger" plain @click="openRestore">恢复部门备份</el-button>
      <el-button @click="load">刷新列表</el-button>
    </div>

    <el-table :data="items" border>
      <el-table-column prop="fileName" label="文件名" min-width="220" />
      <el-table-column label="类型" width="120">
        <template #default="{ row }">{{ row.kind === 'full' ? '完整备份' : '部门归档' }}</template>
      </el-table-column>
      <el-table-column label="部门" width="160">
        <template #default="{ row }">{{ row.department?.name || '全局' }}</template>
      </el-table-column>
      <el-table-column label="大小" width="140">
        <template #default="{ row }">{{ formatSize(row.fileSize) }}</template>
      </el-table-column>
      <el-table-column prop="createdBy" label="备份人ID" width="120" />
      <el-table-column label="生成时间" width="220">
        <template #default="{ row }">{{ formatDateTime(row.createdAt) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="140">
        <template #default="{ row }"><el-button link @click="download(row)">下载</el-button></template>
      </el-table-column>
    </el-table>
    <AppPagination v-model:page="page" v-model:size="size" :total="total" @change="load" />

    <el-dialog v-model="restoreDialog" title="恢复部门 ZIP 备份" width="680px" @closed="resetRestore">
      <el-alert
        type="warning"
        :closable="false"
        show-icon
        title="恢复操作会按 ZIP 中的部门编码覆盖该部门现有业务数据。整个恢复在数据库事务中执行，任一环节失败都会全部回滚。"
      />
      <div class="restore-note">
        只支持本系统“生成本部门归档”下载的 `.zip` 文件；完整数据库 `.dump` 文件不能在这里恢复。
      </div>
      <el-upload
        ref="uploadRef"
        drag
        :auto-upload="false"
        :limit="1"
        accept=".zip"
        :on-change="onRestoreFileChange"
        :on-remove="onRestoreFileRemove"
      >
        <el-icon class="upload-icon"><UploadFilled /></el-icon>
        <div class="upload-copy">拖拽 ZIP 备份到此处，或点击选择文件</div>
        <template #tip><div class="el-upload__tip">文件最大 100MB</div></template>
      </el-upload>
      <el-alert
        v-if="restoreResult"
        class="restore-result"
        type="success"
        :closable="false"
        :title="`恢复完成：${restoreResult.departmentName}，用户 ${restoreResult.users}、产线 ${restoreResult.lines}、填报 ${restoreResult.submissions} 条`"
      />
      <template #footer>
        <el-button @click="restoreDialog = false">取消</el-button>
        <el-button type="danger" :loading="restoring" :disabled="!restoreFile" @click="submitRestore">确认恢复</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { UploadFilled } from '@element-plus/icons-vue'
import { api } from '@/api'
import { formatDateTime } from '@shared'
import { useAuthStore } from '@/stores/auth'
import AppPagination from '@/components/AppPagination.vue'

const auth = useAuthStore()
const items = ref<any[]>([])
const page = ref(1)
const size = ref(20)
const total = ref(0)
const restoreDialog = ref(false)
const restoreFile = ref<File | null>(null)
const restoring = ref(false)
const uploadRef = ref<any>()
const restoreResult = ref<any>(null)

async function load() {
  const { data } = await api.get('/admin/backups', { params: { page: page.value, size: size.value } })
  if (!data.items?.length && page.value > 1) {
    page.value -= 1
    return load()
  }
  items.value = data.items
  total.value = data.total || 0
}

// 全部部门范围生成完整备份，具体部门生成可恢复的 ZIP 归档。
async function create() {
  await api.post('/admin/backups', { type: auth.allDepartments ? 'full' : 'department' })
  ElMessage.success(auth.allDepartments ? '完整备份已生成' : '部门归档已生成')
  await load()
}

// 通过 Blob 下载后端保存的备份文件。
async function download(row: any) {
  const res = await api.get(`/admin/backups/${row.id}/download`, { responseType: 'blob' })
  const url = URL.createObjectURL(res.data)
  const a = document.createElement('a')
  a.href = url
  a.download = row.fileName
  a.click()
  URL.revokeObjectURL(url)
}

function formatSize(value: number) {
  if (value < 1024) return `${value} B`
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`
  return `${(value / 1024 / 1024).toFixed(1)} MB`
}

function openRestore() {
  restoreFile.value = null
  restoreResult.value = null
  restoreDialog.value = true
}

function onRestoreFileChange(file: any) {
  restoreFile.value = file.raw || null
  restoreResult.value = null
}

function onRestoreFileRemove() {
  restoreFile.value = null
}

function resetRestore() {
  restoreFile.value = null
  restoreResult.value = null
  uploadRef.value?.clearFiles()
}

// 恢复前进行高风险二次确认，只上传本系统生成的部门 ZIP。
async function submitRestore() {
  if (!restoreFile.value) return
  try {
    await ElMessageBox.confirm(
      '恢复会覆盖 ZIP 中所属部门的现有业务数据。确认已经核对备份文件并继续吗？',
      '高风险操作确认',
      { type: 'warning', confirmButtonText: '确认恢复', cancelButtonText: '取消' },
    )
  } catch {
    return
  }
  restoring.value = true
  try {
    const form = new FormData()
    form.append('file', restoreFile.value)
    const { data } = await api.post('/admin/backups/restore', form, { timeout: 10 * 60 * 1000 })
    restoreResult.value = data
    ElMessage.success('备份恢复完成')
    await load()
  } catch (error: any) {
    ElMessage.error(error.message || '恢复备份失败')
  } finally {
    restoring.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.page-desc { margin: -8px 0 18px; color: var(--bench-ink-soft); font-size: 13px; }
.restore-note { margin: 16px 0 12px; color: var(--bench-ink-soft); font-size: 13px; line-height: 1.7; }
.upload-icon { margin-bottom: 8px; color: var(--bench-steel); font-size: 30px; }
.upload-copy { color: var(--bench-ink); font-size: 13px; }
.restore-result { margin-top: 16px; }
</style>
