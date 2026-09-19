<!-- 文件说明：部门维护页，仅超级管理员可新增、启停和删除部门。 -->

<template>
  <div class="page departments-page">
    <div class="page-head">
      <div>
        <p class="section-label">系统管理</p>
        <h2 class="page-title">部门维护</h2>
        <p class="page-desc">部门之间的人员、配置、日志和填报数据完全隔离。已有数据的部门只能停用，不能删除。</p>
      </div>
      <el-button type="primary" @click="openCreate">新增部门</el-button>
    </div>

    <el-table :data="departments" border>
      <el-table-column prop="code" label="部门编码" width="180" />
      <el-table-column prop="name" label="部门名称" min-width="220" />
      <el-table-column label="状态" width="110">
        <template #default="{ row }">
          <el-tag :type="row.enabled ? 'success' : 'info'">{{ row.enabled ? '启用' : '停用' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="240">
        <template #default="{ row }">
          <el-button link @click="openEdit(row)">编辑</el-button>
          <el-button link :type="row.enabled ? 'warning' : 'success'" @click="toggle(row)">{{ row.enabled ? '停用' : '启用' }}</el-button>
          <el-button link type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
    <AppPagination v-model:page="page" v-model:size="size" :total="total" @change="load" />

    <el-dialog v-model="dialog" :title="editingId ? '编辑部门' : '新增部门'" width="460px">
      <el-form label-width="88px">
        <el-form-item label="部门编码">
          <el-input v-model="form.code" :disabled="Boolean(editingId)" placeholder="例如 donggang" />
        </el-form-item>
        <el-form-item label="部门名称">
          <el-input v-model="form.name" placeholder="例如 东港分厂" />
        </el-form-item>
        <el-form-item label="状态">
          <el-switch v-model="form.enabled" active-text="启用" inactive-text="停用" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialog = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '@/api'
import { useAuthStore } from '@/stores/auth'
import AppPagination from '@/components/AppPagination.vue'

const auth = useAuthStore()
const departments = ref<any[]>([])
const page = ref(1)
const size = ref(20)
const total = ref(0)
const dialog = ref(false)
const saving = ref(false)
const editingId = ref(0)
const form = reactive({ code: '', name: '', enabled: true })

async function load() {
  const { data } = await api.get('/admin/departments', { params: { page: page.value, size: size.value } })
  if (!data.items?.length && page.value > 1) { page.value -= 1; return load() }
  departments.value = data.items || []
  total.value = data.total || 0
  if (auth.isSuperAdmin) await auth.fetchMe()
}

function openCreate() {
  editingId.value = 0
  form.code = ''
  form.name = ''
  form.enabled = true
  dialog.value = true
}

function openEdit(row: any) {
  editingId.value = row.id
  form.code = row.code
  form.name = row.name
  form.enabled = row.enabled
  dialog.value = true
}

// 部门编码创建后不可修改，避免破坏隔离和归档标识。
async function save() {
  if (!form.code.trim() || !form.name.trim()) {
    ElMessage.warning('请填写部门编码和名称')
    return
  }
  saving.value = true
  try {
    if (editingId.value) {
      await api.patch(`/admin/departments/${editingId.value}`, { name: form.name, enabled: form.enabled })
    } else {
      await api.post('/admin/departments', form)
    }
    dialog.value = false
    ElMessage.success(editingId.value ? '部门已更新' : '部门已创建')
    await load()
  } catch (error: any) {
    ElMessage.error(error.message || '保存部门失败')
  } finally {
    saving.value = false
  }
}

async function toggle(row: any) {
  try {
    await api.patch(`/admin/departments/${row.id}`, { enabled: !row.enabled })
    ElMessage.success(row.enabled ? '部门已停用' : '部门已启用')
    await load()
  } catch (error: any) {
    ElMessage.error(error.message || '修改状态失败')
  }
}

async function remove(row: any) {
  try {
    await ElMessageBox.confirm(`确定删除部门“${row.name}”？仅无任何关联数据时可以删除。`, '删除确认')
    await api.delete(`/admin/departments/${row.id}`)
    ElMessage.success('部门已删除')
    await load()
  } catch (error: any) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error(error.message || '删除部门失败')
  }
}

onMounted(load)
</script>

<style scoped>
.page-head { display: flex; align-items: flex-end; justify-content: space-between; gap: 20px; }
.page-desc { max-width: 760px; margin: -8px 0 22px; color: var(--bench-ink-soft); font-size: 13px; line-height: 1.7; }
</style>
