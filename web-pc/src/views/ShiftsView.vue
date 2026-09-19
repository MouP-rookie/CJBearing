<template>
  <div class="page shift-page">
    <div class="page-head"><div><p class="section-label">班次版本</p><h2 class="page-title">班次配置</h2><p class="page-desc">修改班次会生成新的生效版本，历史填报仍保留原班次名称和时间。</p></div><el-button type="primary" size="large" @click="openCreate">新增班次计划</el-button></div>

    <el-table :data="plans" border row-key="id">
      <el-table-column prop="name" label="计划名称" min-width="170" />
      <el-table-column label="生效时间" width="220"><template #default="{ row }"><span class="mono">{{ formatDateTime(row.effectiveAt) }}</span></template></el-table-column>
      <el-table-column label="班次安排" min-width="350"><template #default="{ row }"><div class="shift-tags"><el-tag v-for="item in row.items" :key="item.id" effect="plain" type="info">{{ item.name }} {{ item.startTime }} - {{ item.endTime }}</el-tag></div></template></el-table-column>
      <el-table-column label="状态" width="100"><template #default="{ $index }"><el-tag v-if="$index === 0" type="success">当前生效</el-tag><span v-else class="muted">历史版本</span></template></el-table-column>
      <el-table-column label="操作" width="180"><template #default="{ row }"><el-button link @click="openEdit(row)">编辑</el-button><el-button link type="danger" @click="remove(row)">删除</el-button></template></el-table-column>
    </el-table>

    <el-dialog v-model="dialog" :title="editingId ? '编辑班次计划' : '新增班次计划'" width="760px">
      <el-form label-position="top">
        <div class="form-grid two"><el-form-item label="计划名称"><el-input v-model="form.name" placeholder="例如：标准两班" /></el-form-item><el-form-item label="生效时间"><el-date-picker v-model="form.effectiveAt" type="datetime" format="YYYY-MM-DD HH:mm:ss" value-format="YYYY-MM-DD HH:mm:ss" placeholder="选择生效时间" style="width:100%" /></el-form-item></div>
        <div class="item-heading"><div><strong>班次安排</strong><small>开始和结束时间使用每天的时间点</small></div><el-button plain :icon="Plus" @click="addItem">增加班次</el-button></div>
        <div class="shift-editor">
          <div v-for="(item, index) in form.items" :key="index" class="shift-editor-row">
            <span class="row-number">{{ index + 1 }}</span>
            <el-input v-model="item.name" placeholder="班次名称" />
            <el-time-picker v-model="item.start" format="HH:mm" value-format="HH:mm" placeholder="开始" />
            <span class="dash">至</span>
            <el-time-picker v-model="item.end" format="HH:mm" value-format="HH:mm" placeholder="结束" />
            <el-checkbox v-model="item.crossMidnight">跨午夜</el-checkbox>
            <el-button text type="danger" :icon="Delete" :disabled="form.items.length === 1" @click="form.items.splice(index, 1)" />
          </div>
        </div>
      </el-form>
      <template #footer><el-button @click="dialog=false">取消</el-button><el-button type="primary" :loading="saving" @click="save">保存并立即生效</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Delete, Plus } from '@element-plus/icons-vue'
import { api } from '@/api'

type ShiftItemForm = { name: string; start: string; end: string; crossMidnight: boolean }
const plans = ref<any[]>([])
const dialog = ref(false)
const saving = ref(false)
const editingId = ref(0)
const form = reactive({ name: '', effectiveAt: '', items: [] as ShiftItemForm[] })

function pad(value: number) { return String(value).padStart(2, '0') }
function formatDateTime(value: string | Date) { const date = new Date(value); if (Number.isNaN(date.getTime())) return String(value); return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}` }
function nowString() { return formatDateTime(new Date()) }
function createDefaultItems(): ShiftItemForm[] { return [{ name: '白班', start: '07:00', end: '19:00', crossMidnight: false }, { name: '中班', start: '19:00', end: '07:00', crossMidnight: true }] }
function addItem() { form.items.push({ name: '', start: '', end: '', crossMidnight: false }) }
async function load() { const { data } = await api.get('/admin/shifts'); plans.value = data.items }
function openCreate() { editingId.value = 0; form.name = '标准两班'; form.effectiveAt = nowString(); form.items = createDefaultItems(); dialog.value = true }
function openEdit(plan: any) { editingId.value = plan.id; form.name = plan.name; form.effectiveAt = formatDateTime(plan.effectiveAt); form.items = (plan.items || []).map((item: any) => ({ name: item.name, start: item.startTime, end: item.endTime, crossMidnight: item.crossMidnight })); dialog.value = true }
async function save() {
  if (!form.name.trim()) { ElMessage.warning('请填写计划名称'); return }
  if (!form.effectiveAt) { ElMessage.warning('请选择生效时间'); return }
  if (!form.items.length) { ElMessage.warning('至少需要一个班次'); return }
  for (const item of form.items) { if (!item.name.trim() || !item.start || !item.end) { ElMessage.warning('请完整填写每个班次的名称和起止时间'); return } }
  const payload = { name: form.name.trim(), effectiveAt: form.effectiveAt, items: form.items.map((item, index) => ({ name: item.name.trim(), startTime: item.start, endTime: item.end, crossMidnight: item.crossMidnight, sortOrder: index })) }
  saving.value = true
  try { if (editingId.value) await api.put(`/admin/shifts/${editingId.value}`, payload); else await api.post('/admin/shifts', payload); dialog.value = false; await load(); ElMessage.success(editingId.value ? '班次修改已生成新版本' : '班次计划已创建') } catch (error: any) { ElMessage.error(error.message || '保存班次失败') } finally { saving.value = false }
}
async function remove(plan: any) { try { await ElMessageBox.confirm(`确定删除班次计划“${plan.name}”？删除后历史填报不受影响。`, '删除确认'); await api.delete(`/admin/shifts/${plan.id}`); await load(); ElMessage.success('班次计划已删除') } catch (error: any) { if (error !== 'cancel') ElMessage.error(error.message || '删除班次失败') } }
onMounted(load)
</script>

<style scoped>
.page-head { display: flex; align-items: flex-end; justify-content: space-between; gap: 20px; }
.page-desc { max-width: 700px; margin: -8px 0 22px; color: var(--bench-ink-soft); font-size: 13px; line-height: 1.7; }
.mono { font-variant-numeric: tabular-nums; letter-spacing: .02em; }
.muted { color: var(--bench-ink-soft); font-size: 12px; }
.shift-tags { display: flex; flex-wrap: wrap; gap: 7px; }
.item-heading { display: flex; align-items: center; justify-content: space-between; margin: 8px 0 10px; }
.item-heading strong, .item-heading small { display: block; }
.item-heading strong { font-size: 14px; }
.item-heading small { margin-top: 4px; color: var(--bench-ink-soft); font-size: 11px; }
.shift-editor { display: grid; gap: 9px; }
.shift-editor-row { display: grid; grid-template-columns: 28px 1.2fr 130px 30px 130px auto 36px; align-items: center; gap: 8px; padding: 10px; border: 1px solid var(--bench-line); background: #fffefa; }
.row-number { color: var(--bench-steel); font-size: 12px; font-weight: 700; }
.dash { color: var(--bench-ink-soft); text-align: center; font-size: 12px; }
.form-grid { display: grid; gap: 14px; }
.form-grid.two { grid-template-columns: 1fr 1fr; }
@media (max-width: 900px) { .page-head { align-items: flex-start; flex-direction: column; } .shift-editor-row { grid-template-columns: 28px 1fr; } .shift-editor-row .el-checkbox { grid-column: 2; } .form-grid.two { grid-template-columns: 1fr; } }
</style>