<!-- 文件说明：任务与表单页，提供可视化字段设计和模板版本管理。 -->

<template>
  <div class="page forms-page">
    <div class="page-head">
      <div><p class="section-label">表单工厂</p><h2 class="page-title">任务与表单配置</h2><p class="page-desc">用可视化方式定义首末检、报表和登记表，业务人员不需要接触 JSON。</p></div>
      <div class="page-actions">
        <DepartmentActionButton type="danger" plain size="large" :disabled="!unusedItems.length" @click="deleteUnused">
          删除未使用<span v-if="unusedItems.length">（{{ unusedItems.length }}）</span>
        </DepartmentActionButton>
        <DepartmentActionButton type="primary" size="large" @click="openCreate">新建模板</DepartmentActionButton>
      </div>
    </div>

    <div class="toolbar">
      <el-segmented v-model="kind" :options="kindOptions" @change="changeKind" />
      <div class="toolbar-spacer"></div>
      <span class="result-count">共 {{ items.length }} 个模板</span>
    </div>

    <el-table :data="items" border row-key="id">
      <el-table-column prop="code" label="编码" width="190" />
      <el-table-column prop="name" label="名称" min-width="200" />
      <el-table-column label="部门" width="150"><template #default="{ row }">{{ row.department?.name || '-' }}</template></el-table-column>
      <el-table-column label="类型" width="140"><template #default="{ row }"><span class="kind-chip" :class="row.kind">{{ kindName(row.kind) }}</span></template></el-table-column>
      <el-table-column label="创建人" width="130" show-overflow-tooltip>
        <template #default="{ row }">{{ row.createdByName || (row.createdBy ? `用户 #${row.createdBy}` : '—') }}</template>
      </el-table-column>
      <el-table-column label="创建时间" width="170">
        <template #default="{ row }">{{ formatDateTime(row.createdAt) }}</template>
      </el-table-column>
      <el-table-column label="使用情况" width="190">
        <template #default="{ row }">
          <el-tag :type="row.isUnused ? 'info' : 'success'" size="small">{{ row.isUnused ? '未使用' : '已使用' }}</el-tag>
          <span class="usage-note">产线 {{ row.assignmentCount }} · 填报 {{ row.submissionCount }}</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="380"><template #default="{ row }"><el-button link @click="view(row)">版本记录</el-button><DepartmentActionButton link @click="edit(row)">编辑</DepartmentActionButton><DepartmentActionButton link @click="publishLatest(row)">发布起草</DepartmentActionButton><DepartmentActionButton v-if="row.kind==='first_inspection'" link @click="copy(row)">复制为末检</DepartmentActionButton><DepartmentActionButton v-if="row.canDelete" link type="danger" @click="deleteOne(row)">删除</DepartmentActionButton></template></el-table-column>
    </el-table>
    <AppPagination v-model:page="page" v-model:size="size" :total="total" @change="load" />

    <el-drawer v-model="builderOpen" size="94%" :with-header="false" destroy-on-close>
      <div class="builder-shell">
        <header class="builder-header"><div><p class="section-label">可视化模板编辑器</p><h2>{{ metadata.name || '未命名模板' }}</h2></div><div class="builder-actions"><el-button @click="builderOpen=false">取消</el-button><el-button type="primary" :loading="saving" @click="save">保存模板</el-button></div></header>

        <div class="builder-grid">
          <aside class="builder-column structure-column">
            <section class="builder-block">
              <div class="block-title"><span>01</span><div><strong>模板信息</strong><small>业务人员看到的中文名称和用途</small></div></div>
              <el-form label-position="top">
                <el-form-item label="模板名称"><el-input v-model="metadata.name" :disabled="!!editingTemplateId" placeholder="例如：磨工小内圈1线首检" /></el-form-item>
                <el-form-item label="模板编码" :error="codeError"><el-input v-model="metadata.code" :disabled="!!editingTemplateId || saving" placeholder="例如：FIRST_LINE_01" @input="codeError = ''" /></el-form-item>
                <el-form-item label="模板类型"><el-select v-model="metadata.kind" :disabled="!!editingTemplateId" style="width:100%"><el-option v-for="item in kindOptions" :key="item.value" :label="item.label" :value="item.value" /></el-select></el-form-item><p v-if="editingTemplateId" class="edit-note">已有模板保存为新草稿版本，发布后生效；编码、名称与类型不可修改。</p>
              </el-form>
            </section>

            <section class="builder-block field-structure">
              <div class="block-title"><span>02</span><div><strong>填写项目</strong><small>拖动不是重点，顺序用上下按钮调整</small></div></div>
              <button v-for="(field, index) in fields" :key="field.key" class="structure-item" :class="{ active: activeIndex === index }" @click="activeIndex = index">
                <span class="structure-order">{{ String(index + 1).padStart(2, '0') }}</span>
                <span class="structure-copy"><strong>{{ field.label || '未命名字段' }}</strong><small>{{ typeName(field.type) }}<template v-if="field.unit"> · {{ field.unit }}</template></small></span>
                <span class="structure-controls"><el-button text :icon="ArrowUp" @click.stop="moveField(index, -1)" /><el-button text :icon="ArrowDown" @click.stop="moveField(index, 1)" /><el-button text type="danger" :icon="Delete" @click.stop="removeField(index)" /></span>
              </button>
              <el-button class="add-field" plain :icon="Plus" @click="addField">增加填写项目</el-button>
            </section>
          </aside>

          <main class="builder-column editor-column">
            <section v-if="currentField" class="builder-block">
              <div class="block-title"><span>03</span><div><strong>编辑当前项目</strong><small>不写代码，只设置用户实际填写时需要看到的内容</small></div></div>
              <el-form label-position="top">
                <div class="form-grid two">
                  <el-form-item label="显示名称"><el-input v-model="currentField.label" placeholder="例如：端跳" @input="syncGeneratedKey(currentField)" /></el-form-item>
                  <el-form-item label="字段标识（自动生成）"><el-input v-model="currentField.key" disabled /></el-form-item>
                </div>
                <div class="form-grid two">
                  <el-form-item label="填写方式"><el-select v-model="currentField.type" style="width:100%" @change="onTypeChange(currentField)"><el-option v-for="type in fieldTypes" :key="type.value" :label="type.label" :value="type.value" /></el-select></el-form-item>
                  <el-form-item v-if="currentField.type !== 'product'" label="计量单位"><el-input v-model="currentField.unit" placeholder="例如：mm、μm、N、g" /></el-form-item>
                </div>
                <div class="switch-row"><span><strong>必须填写</strong><small>未填写时不能提交</small></span><el-switch v-model="currentField.required" /></div>
                <div class="switch-row"><span><strong>允许多个测量值</strong><small>例如端跳一次测三个或四个轴承</small></span><el-switch v-model="currentField.allowMultiple" :disabled="!canRepeatValue(currentField.type)" /></div>
                <div v-if="metadata.kind === 'registration'" class="switch-row"><span><strong>作为登记表明细字段</strong><small>一次提交可以填写多行相同结构的数据</small></span><el-switch v-model="currentField.repeatable" /></div>

                <el-form-item v-if="currentField.type !== 'product'" label="默认值"><div class="default-row"><el-input v-model="currentField.defaultValue" placeholder="可留空；也可以插入登录人、日期、班次等动态值" /><el-select class="token-select" placeholder="插入动态值" :model-value="''" @change="appendToken(currentField, $event)"><el-option v-for="token in tokenOptions" :key="token.value" :label="token.label" :value="token.value" /></el-select></div></el-form-item>

                <div v-if="currentField.type === 'number'" class="form-grid two"><el-form-item label="最小值（可空）"><el-input-number v-model="currentField.min" :controls="false" style="width:100%" placeholder="不限" /></el-form-item><el-form-item label="最大值（可空）"><el-input-number v-model="currentField.max" :controls="false" style="width:100%" placeholder="不限" /></el-form-item></div>

                <div v-if="isSelect(currentField.type)" class="option-editor"><div class="option-head"><span>可选项</span><el-button link :icon="Plus" @click="addOption(currentField)">增加选项</el-button></div><div v-for="(option, index) in currentField.options" :key="index" class="option-row"><el-input v-model="option.label" placeholder="选项文字" @input="syncOptionValue(option)" /><el-button text type="danger" :icon="Delete" @click="currentField.options.splice(index, 1)" /></div><p v-if="!currentField.options.length" class="empty-hint">选择类字段至少需要两个选项。</p></div>
              </el-form>
            </section>
            <div v-else class="empty-panel"><div><strong>还没有填写项目</strong><p>先点击左侧“增加填写项目”，再编辑字段。</p></div></div>
          </main>

          <aside class="builder-column preview-column">
            <section class="builder-block preview-card">
              <div class="block-title"><span>04</span><div><strong>实时预览</strong><small>这里展示现场人员最终看到的填写内容</small></div></div>
              <div class="preview-paper">
                <div class="preview-heading"><span class="preview-kind">{{ kindName(metadata.kind) }}</span><strong>{{ metadata.name || '模板名称' }}</strong><small>班次与业务日期将在填报时自动锁定</small></div>
                <div v-for="(field, index) in fields" :key="field.key" class="preview-field">
                  <div class="preview-label"><span>{{ field.label || '未命名项目' }}<b v-if="field.required">*</b></span><small v-if="field.unit">{{ field.unit }}</small></div>
                  <div v-if="field.type === 'static_text'" class="preview-static">{{ field.defaultValue || '固定说明文字' }}</div>
                  <el-radio-group v-else-if="field.type === 'single_select'" disabled><el-radio v-for="option in field.options" :key="option.value" :value="option.value">{{ option.label }}</el-radio></el-radio-group>
                  <el-checkbox-group v-else-if="field.type === 'multi_select'" disabled><el-checkbox v-for="option in field.options" :key="option.value" :value="option.value">{{ option.label }}</el-checkbox></el-checkbox-group>
                  <el-select v-else-if="field.type === 'product'" disabled placeholder="点检时按产线选择产品" style="width:100%" />
                  <el-input v-else disabled :placeholder="field.allowMultiple ? '可增加多个测量值' : '等待填写'" />
                  <div v-if="field.type === 'number' && (field.min !== undefined || field.max !== undefined)" class="preview-range">允许范围：{{ field.min ?? '不限' }} 至 {{ field.max ?? '不限' }}</div>
                </div>
                <div v-if="!fields.length" class="empty-preview">填写项目会在这里逐项出现</div>
              </div>
            </section>
          </aside>
        </div>
      </div>
    </el-drawer>

    <el-drawer v-model="versionsOpen" title="版本记录" size="58%"><el-table :data="pagedVersions" border><el-table-column prop="versionNo" label="版本" width="80"><template #default="{ row }">v{{ row.versionNo }}</template></el-table-column><el-table-column prop="status" label="状态" width="120" /><el-table-column label="发布时间"><template #default="{ row }">{{ formatDateTime(row.publishedAt) }}</template></el-table-column><el-table-column label="创建时间"><template #default="{ row }">{{ formatDateTime(row.createdAt) }}</template></el-table-column></el-table><AppPagination v-model:page="versionPage" v-model:size="versionSize" :total="versions.length" :page-sizes="[10, 20, 50]" /></el-drawer>
  </div>
</template>

<script setup lang="ts">
import DepartmentActionButton from '@/components/DepartmentActionButton.vue'
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowDown, ArrowUp, Delete, Plus } from '@element-plus/icons-vue'
import { api } from '@/api'
import AppPagination from '@/components/AppPagination.vue'

const page = ref(1)
const size = ref(20)
const total = ref(0)
import { formatDateTime } from '@shared'

type FieldType = 'text' | 'number' | 'single_select' | 'multi_select' | 'datetime' | 'static_text' | 'product'
type Option = { label: string; value: string }
type EditorField = { key: string; label: string; type: FieldType; required: boolean; defaultValue: string; unit: string; min?: number; max?: number; allowMultiple: boolean; repeatable: boolean; options: Option[] }

const kind = ref('first_inspection')
const kindOptions = [{ label: '首检', value: 'first_inspection' }, { label: '末检', value: 'final_inspection' }, { label: '报表', value: 'report' }, { label: '登记表', value: 'registration' }]
const fieldTypes = [{ label: '文本输入', value: 'text' }, { label: '数字输入', value: 'number' }, { label: '单选', value: 'single_select' }, { label: '多选', value: 'multi_select' }, { label: '日期时间', value: 'datetime' }, { label: '固定说明文字', value: 'static_text' }, { label: '产品选择', value: 'product' }]
const tokenOptions = [{ label: '当前日期', value: '{{current_date}}' }, { label: '当前时间', value: '{{current_time}}' }, { label: '当前班次', value: '{{current_shift}}' }, { label: '登录人姓名', value: '{{user_name}}' }, { label: '登录人工号', value: '{{user_account}}' }, { label: '当前产线', value: '{{line_name}}' }, { label: '当前表单名称', value: '{{form_name}}' }, { label: '首检/末检', value: '{{stage}}' }, { label: '当前产线的产品型号', value: '{{line_product}}' }]
const items = ref<any[]>([])
const unusedItems = computed(() => items.value.filter(item => item.isUnused))
const versions = ref<any[]>([])
const versionsOpen = ref(false)
const versionPage = ref(1)
const versionSize = ref(20)
const pagedVersions = computed(() => versions.value.slice((versionPage.value - 1) * versionSize.value, versionPage.value * versionSize.value))
const builderOpen = ref(false)
const saving = ref(false)
const codeError = ref('')
const activeIndex = ref(0)
const fields = ref<EditorField[]>([])
const metadata = reactive({ code: '', name: '', kind: 'first_inspection' })
const currentField = computed(() => fields.value[activeIndex.value])
const editingTemplateId = ref<number | null>(null)
async function edit(row: any) {
  const { data } = await api.get(`/admin/form-templates/${row.id}`)
  const versions = (data.versions || []) as any[]
  if (!versions.length) { ElMessage.warning('该模板还没有任何版本'); return }
  const latest = versions[0]
  const schema = typeof latest.schema === 'string' ? JSON.parse(latest.schema) : latest.schema
  codeError.value = ''
  metadata.code = row.code
  metadata.name = row.name
  metadata.kind = row.kind
  fields.value = ((schema?.fields || []) as any[]).sort((x, y) => (x.sortOrder || 0) - (y.sortOrder || 0)).map((f: any) => ({ key: f.key, label: f.label || '', type: f.type || 'text', required: !!f.required, defaultValue: f.defaultValue || '', unit: f.unit || '', min: f.min, max: f.max, allowMultiple: !!f.allowMultiple, repeatable: !!f.repeatable, options: (f.options || []).map((o: any) => ({ label: o.label, value: o.value })) }))
  editingTemplateId.value = row.id
  activeIndex.value = 0
  builderOpen.value = true
}

function kindName(value: string) { return kindOptions.find(item => item.value === value)?.label || value }
function typeName(value: string) { return fieldTypes.find(item => item.value === value)?.label || value }
function isSelect(value: string) { return value === 'single_select' || value === 'multi_select' }
function canRepeatValue(value: string) { return ['text', 'number', 'datetime'].includes(value) }
function makeKey(index: number) { return `field_${index + 1}_${Date.now().toString(36)}` }
function emptyField(index = fields.value.length): EditorField { return { key: makeKey(index), label: '', type: 'text', required: false, defaultValue: '', unit: '', allowMultiple: false, repeatable: false, options: [] } }
function syncGeneratedKey(field: EditorField) { if (!field.label) return; if (/^field_\d+/.test(field.key)) return; }
function addField() { fields.value.push(emptyField()); activeIndex.value = fields.value.length - 1 }
function removeField(index: number) { fields.value.splice(index, 1); activeIndex.value = Math.max(0, Math.min(activeIndex.value, fields.value.length - 1)) }
function moveField(index: number, direction: number) { const next = index + direction; if (next < 0 || next >= fields.value.length) return; const copy = fields.value[index]; fields.value[index] = fields.value[next]; fields.value[next] = copy; activeIndex.value = next }
function addOption(field: EditorField) { field.options.push({ label: '', value: `option_${field.options.length + 1}` }) }
function syncOptionValue(option: Option) { if (!option.value) option.value = `option_${Date.now().toString(36)}` }
function onTypeChange(field: EditorField) { if (isSelect(field.type) && !field.options.length) { addOption(field); addOption(field) } if (!canRepeatValue(field.type)) field.allowMultiple = false; if (field.type === 'static_text') { field.required = false; field.allowMultiple = false } if (field.type === 'product') { field.allowMultiple = false; field.repeatable = false; field.unit = ''; field.defaultValue = '' } }
function appendToken(field: EditorField, token: string) { field.defaultValue = `${field.defaultValue || ''}${token}` }
function openCreate() { editingTemplateId.value = null; codeError.value = ''; metadata.code = ''; metadata.name = ''; metadata.kind = kind.value; fields.value = [emptyField(0)]; fields.value[0].label = '检测值'; fields.value[0].type = kind.value === 'report' ? 'text' : 'number'; fields.value[0].unit = kind.value === 'report' ? '' : 'mm'; fields.value[0].allowMultiple = kind.value === 'first_inspection' || kind.value === 'final_inspection'; activeIndex.value = 0; builderOpen.value = true }
function changeKind() { page.value = 1; load() }
async function load() {
  const { data } = await api.get('/admin/form-templates', { params: { kind: kind.value, page: page.value, size: size.value } })
  if (!data.items?.length && page.value > 1) { page.value -= 1; return load() }
  items.value = data.items
  total.value = data.total || 0
}
async function view(row: any) { const { data } = await api.get(`/admin/form-templates/${row.id}`); versions.value = data.versions; versionPage.value = 1; versionsOpen.value = true }
// 把编辑器状态转换为后端约定的 FormSchema。
function buildSchema() { return { fields: fields.value.map((field, index) => ({ ...field, sortOrder: index, options: field.options.map(option => ({ ...option, value: option.value || option.label })) })) } }
// 新建模板创建 v1 草稿；已有模板保存时生成新草稿版本。
async function save() {
  if (!metadata.name.trim() || !metadata.code.trim()) { ElMessage.warning('请填写模板名称和模板编码'); return }
  if (!fields.value.length) { ElMessage.warning('请至少增加一个填写项目'); return }
  for (const field of fields.value) { if (!field.label.trim()) { ElMessage.warning('请填写所有项目的显示名称'); return }; if (isSelect(field.type) && field.options.filter(option => option.label.trim()).length < 2) { ElMessage.warning(`“${field.label}”至少需要两个有效选项`); return } }
  saving.value = true
  try {
    const schema = buildSchema()
    if (editingTemplateId.value) {
      await api.post(`/admin/form-templates/${editingTemplateId.value}/versions`, { schema })
      ElMessage.success('已保存为新草稿版本，发布后生效')
    } else {
      await api.post('/admin/form-templates', { code: metadata.code.trim(), name: metadata.name.trim(), kind: metadata.kind, schema })
      ElMessage.success('模板已保存，当前版本为草稿')
    }
    builderOpen.value = false
    load()
  } catch (error: any) {
    if (error.code === 'template_code_conflict') codeError.value = '当前部门已使用此编码，请更换。'
    ElMessage.error(error.message || '保存模板失败')
  } finally {
    saving.value = false
  }
}
// 发布最新草稿；已分配该模板的产线会在后端事务中切换活动版本。
async function publishLatest(row: any) {
  try {
    const { data } = await api.get(`/admin/form-templates/${row.id}`)
    const draft = data.versions.find((version: any) => version.status === 'draft')
    if (!draft) { ElMessage.warning('没有待发布的草稿'); return }
    await ElMessageBox.confirm('发布后该版本可在「产线分配」中分配到产线；已经分配过该模板的产线会自动更新到新版本，其他产线不受影响。是否继续？', '发布确认')
    await api.post(`/admin/form-versions/${draft.id}/publish`, { lineIds: [] })
    ElMessage.success('发布完成')
  } catch (error: any) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error(error.message || '发布模板失败')
    return
  }
  await load()
}
// 首检模板复制为末检模板时保留字段结构并生成新草稿。
async function copy(row: any) {
  try {
    await api.post(`/admin/form-templates/${row.id}/copy`, { code: `${row.code}_FINAL`, name: `${row.name}（末检）`, kind: 'final_inspection' })
  } catch (error: any) {
    ElMessage.error(error.message || '复制模板失败')
    return
  }
  ElMessage.success('已复制为末检草稿')
  kind.value = 'final_inspection'
  await load()
}
async function deleteOne(row: any) {
  try {
    const message = row.isUnused
      ? `确定删除未使用模板“${row.name}”？删除后模板编码可以重新使用。`
      : `模板“${row.name}”已有历史填报记录。删除后将从产线任务中移除，不能继续使用，但历史日志和导出仍会保留。确定删除吗？`
    await ElMessageBox.confirm(message, '删除模板', { type: 'warning' })
    const { data } = await api.post('/admin/form-template-delete', { templateIds: [row.id] })
    if (data.deleted) ElMessage.success(`已删除“${row.name}”，历史填报记录仍保留`)
    else ElMessage.warning('没有可删除的模板')
    await load()
  } catch (error: any) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error(error.message || '删除失败')
  }
}

// 只清理没有产线分配和填报历史的模板。
async function deleteUnused() {
  const count = unusedItems.value.length
  if (!count) {
    ElMessage.info('当前分类没有未使用模板')
    return
  }
  try {
    await ElMessageBox.confirm(`确定删除当前分类下 ${count} 个未使用模板？已分配产线或有填报历史的模板会自动跳过。`, '批量删除确认', { type: 'warning' })
    const { data } = await api.post('/admin/form-template-cleanup', { kind: kind.value })
    ElMessage.success(`已删除 ${data.deleted || 0} 个模板${data.skipped ? `，跳过 ${data.skipped} 个` : ''}`)
    await load()
  } catch (error: any) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error(error.message || '删除失败')
  }
}
onMounted(load)
</script>

<style scoped>
.forms-page .page-head { display: flex; align-items: flex-end; justify-content: space-between; gap: 20px; }
.page-actions { display: flex; align-items: center; gap: 10px; }
.usage-note { display: block; margin-top: 6px; color: var(--bench-ink-soft); font-size: 11px; }
.page-desc { max-width: 660px; margin: -8px 0 22px; color: var(--bench-ink-soft); font-size: 13px; line-height: 1.7; }
.toolbar-spacer { flex: 1; }
.result-count { color: var(--bench-ink-soft); font-size: 12px; }
.kind-chip { display: inline-block; padding: 4px 7px; border: 1px solid #a9bac0; color: var(--bench-steel-deep); font-size: 12px; }
.kind-chip.report { border-color: #c8b38d; color: #8a611d; }
.kind-chip.registration { border-color: #a6c5b5; color: #2e7356; }
.builder-shell { min-height: 100vh; background: var(--bench-paper); }
.builder-header { display: flex; align-items: center; justify-content: space-between; gap: 20px; padding: 22px 28px 18px; border-bottom: 1px solid var(--bench-line); background: var(--bench-white); }
.builder-header h2 { margin: 7px 0 0; font-size: 25px; letter-spacing: -.03em; }
.builder-actions { display: flex; gap: 8px; }
.builder-grid { display: grid; grid-template-columns: minmax(260px, .8fr) minmax(420px, 1.35fr) minmax(300px, .85fr); gap: 18px; padding: 20px; }
.builder-column { min-width: 0; }
.builder-block { padding: 18px; border: 1px solid var(--bench-line); background: var(--bench-white); }
.builder-block + .builder-block { margin-top: 16px; }
.block-title { display: flex; gap: 10px; align-items: flex-start; margin-bottom: 18px; }
.block-title > span { color: var(--bench-steel); font-size: 12px; font-weight: 700; }
.block-title strong, .block-title small { display: block; }
.block-title strong { font-size: 15px; }
.block-title small { margin-top: 4px; color: var(--bench-ink-soft); font-size: 11px; line-height: 1.5; }
.field-structure { padding: 16px 12px; }
.field-structure .block-title { padding: 0 6px; }
.structure-item { display: grid; grid-template-columns: 30px 1fr auto; align-items: center; gap: 8px; width: 100%; padding: 11px 6px; border: 1px solid transparent; border-bottom-color: var(--bench-line); color: var(--bench-ink); background: transparent; text-align: left; cursor: pointer; }
.structure-item:hover { background: #f5f8f6; }
.structure-item.active { border-color: var(--bench-steel); background: #eaf1f3; }
.structure-order { color: var(--bench-steel); font-size: 11px; font-weight: 700; }
.structure-copy strong, .structure-copy small { display: block; }
.structure-copy strong { font-size: 13px; }
.structure-copy small { margin-top: 4px; color: var(--bench-ink-soft); font-size: 11px; }
.structure-controls { display: flex; transform: scale(.82); transform-origin: right center; }
.add-field { width: 100%; margin-top: 13px; }
.form-grid { display: grid; gap: 13px; }
.form-grid.two { grid-template-columns: 1fr 1fr; }
.switch-row { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 13px 0; border-top: 1px solid var(--bench-line); }
.switch-row strong, .switch-row small { display: block; }
.switch-row strong { font-size: 13px; }
.switch-row small { margin-top: 4px; color: var(--bench-ink-soft); font-size: 11px; }
.default-row { display: flex; width: 100%; gap: 8px; }
.default-row .el-input { flex: 1; }
.token-select { width: 145px; }
.option-editor { margin-top: 6px; padding-top: 14px; border-top: 1px solid var(--bench-line); }
.option-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; font-size: 13px; font-weight: 650; }
.option-row { display: flex; gap: 8px; margin-top: 8px; }
.option-row .el-input { flex: 1; }
.empty-hint { margin: 9px 0 0; color: var(--bench-amber); font-size: 11px; }
.edit-note { margin: 6px 0 0; color: var(--bench-amber); font-size: 11px; line-height: 1.5; }
.preview-paper { padding: 18px; border: 1px solid var(--bench-line); background: linear-gradient(rgba(27,30,34,.025) 1px, transparent 1px), linear-gradient(90deg, rgba(27,30,34,.025) 1px, transparent 1px), #fffefa; background-size: 20px 20px; }
.preview-heading { padding-bottom: 16px; border-bottom: 2px solid var(--bench-ink); }
.preview-heading strong, .preview-heading small { display: block; }
.preview-heading strong { margin-top: 7px; font-size: 19px; }
.preview-heading small { margin-top: 5px; color: var(--bench-ink-soft); font-size: 11px; }
.preview-kind { color: var(--bench-steel); font-size: 11px; font-weight: 700; }
.preview-field { padding: 15px 0; border-bottom: 1px solid var(--bench-line); }
.preview-label { display: flex; align-items: center; justify-content: space-between; gap: 10px; margin-bottom: 8px; font-size: 13px; font-weight: 650; }
.preview-label span b { margin-left: 4px; color: var(--bench-amber); }
.preview-label small { color: var(--bench-ink-soft); font-size: 11px; font-weight: 400; }
.preview-static { padding: 8px 10px; color: var(--bench-ink-soft); background: #f0ede5; font-size: 12px; line-height: 1.6; }
.preview-range { margin-top: 6px; color: var(--bench-ink-soft); font-size: 11px; }
.empty-preview { padding: 35px 10px; color: var(--bench-ink-soft); text-align: center; font-size: 12px; }
@media (max-width: 1200px) { .builder-grid { grid-template-columns: 1fr 1fr; } .preview-column { grid-column: 1 / -1; } }
@media (max-width: 800px) { .forms-page .page-head { align-items: flex-start; flex-direction: column; } .page-actions { width: 100%; flex-wrap: wrap; } .builder-grid { grid-template-columns: 1fr; padding: 14px; } .preview-column { grid-column: auto; } .form-grid.two { grid-template-columns: 1fr; } }
</style>
