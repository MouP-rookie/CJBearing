<template>
  <div class="page settings-page">
    <div class="page-head">
      <div>
        <p class="section-label">系统设置</p>
        <h2 class="page-title">Logo 配置</h2>
        <p class="page-desc">上传自定义 Logo 后会替换 PC 和 PDA 左上角的默认图标。支持 PNG、JPG、WebP，最大 2MB。</p>
      </div>
    </div>

    <section class="panel logo-panel">
      <div class="logo-preview-card">
        <div class="logo-preview">
          <img v-if="logoDataUrl" :src="logoDataUrl" alt="当前 Logo" />
          <div v-else class="default-logo" aria-hidden="true"><i></i><i></i><i></i></div>
        </div>
        <div class="logo-meta">
          <strong>{{ logoDataUrl ? '当前自定义 Logo' : '当前使用默认 Logo' }}</strong>
          <small>显示在 PC 端左侧栏和登录页左上角</small>
        </div>
      </div>

      <el-upload
        ref="uploadRef"
        class="logo-upload"
        drag
        :auto-upload="false"
        :limit="1"
        accept="image/png,image/jpeg,image/webp"
        :on-change="onFileChange"
        :on-remove="onFileRemove"
      >
        <el-icon class="upload-icon"><UploadFilled /></el-icon>
        <div class="upload-copy">将 Logo 拖到此处，或点击选择图片</div>
        <template #tip>
          <div class="el-upload__tip">仅支持 PNG、JPG、WebP，文件大小不超过 2MB</div>
        </template>
      </el-upload>

      <div class="logo-actions">
        <el-button @click="resetLogo">恢复默认 Logo</el-button>
        <el-button type="primary" :loading="saving" :disabled="!logoDataUrl || !dirty" @click="saveLogo">保存 Logo</el-button>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { UploadFilled } from '@element-plus/icons-vue'
import { api } from '@/api'

const logoDataUrl = ref('')
const original = ref('')
const dirty = ref(false)
const saving = ref(false)
const uploadRef = ref<any>()

async function loadLogo() {
  try {
    const { data } = await api.get('/settings/brand')
    logoDataUrl.value = data.logoDataUrl || ''
    original.value = logoDataUrl.value
  } catch (error: any) {
    ElMessage.error(error.message || '读取 Logo 失败')
  }
}

function onFileChange(file: any) {
  const raw = file.raw as File | null
  if (!raw) return
  if (!['image/png', 'image/jpeg', 'image/webp'].includes(raw.type)) {
    ElMessage.warning('仅支持 PNG、JPG、WebP 图片')
    uploadRef.value?.clearFiles()
    return
  }
  if (raw.size > 2 * 1024 * 1024) {
    ElMessage.warning('Logo 文件不能超过 2MB')
    uploadRef.value?.clearFiles()
    return
  }
  const reader = new FileReader()
  reader.onload = () => {
    logoDataUrl.value = String(reader.result || '')
    dirty.value = true
  }
  reader.readAsDataURL(raw)
}

function onFileRemove() {
  logoDataUrl.value = original.value
  dirty.value = false
}

async function saveLogo() {
  if (!logoDataUrl.value) {
    ElMessage.warning('请先选择 Logo 图片')
    return
  }
  saving.value = true
  try {
    const { data } = await api.put('/admin/settings/brand', { logoDataUrl: logoDataUrl.value })
    logoDataUrl.value = data.logoDataUrl || ''
    original.value = logoDataUrl.value
    dirty.value = false
    uploadRef.value?.clearFiles()
    ElMessage.success('Logo 已保存')
  } catch (error: any) {
    ElMessage.error(error.message || '保存 Logo 失败')
  } finally {
    saving.value = false
  }
}

async function resetLogo() {
  saving.value = true
  try {
    const { data } = await api.put('/admin/settings/brand', { logoDataUrl: '' })
    logoDataUrl.value = data.logoDataUrl || ''
    original.value = ''
    dirty.value = false
    uploadRef.value?.clearFiles()
    ElMessage.success('已恢复默认 Logo')
  } catch (error: any) {
    ElMessage.error(error.message || '恢复默认 Logo 失败')
  } finally {
    saving.value = false
  }
}

onMounted(loadLogo)
</script>

<style scoped>
.page-head { max-width: 760px; }
.page-desc { margin: -8px 0 22px; color: var(--bench-ink-soft); font-size: 13px; line-height: 1.7; }
.logo-panel { max-width: 680px; padding: 24px; }
.logo-preview-card { display: flex; align-items: center; gap: 16px; margin-bottom: 20px; padding: 18px; border: 1px solid var(--bench-line); background: #fffefa; }
.logo-preview { display: grid; place-items: center; width: 84px; height: 84px; border: 1px solid var(--bench-line); background: #1b1e22; border-radius: 6px; overflow: hidden; }
.logo-preview img { width: 100%; height: 100%; object-fit: contain; }
.default-logo { position: relative; width: 40px; height: 40px; border: 2px solid #70a7b5; border-radius: 50%; }
.default-logo i { position: absolute; inset: 8px; border: 1px solid #70a7b5; border-radius: 50%; }
.default-logo i:nth-child(2) { inset: 14px; background: #2e8b65; border-color: #2e8b65; }
.default-logo i:nth-child(3) { inset: 0; border-style: dashed; transform: rotate(35deg); opacity: .65; }
.logo-meta strong, .logo-meta small { display: block; }
.logo-meta strong { font-size: 15px; }
.logo-meta small { margin-top: 7px; color: var(--bench-ink-soft); font-size: 12px; }
.upload-icon { margin-bottom: 8px; color: var(--bench-steel); font-size: 30px; }
.upload-copy { color: var(--bench-ink); font-size: 13px; }
.logo-actions { display: flex; justify-content: flex-end; gap: 10px; margin-top: 22px; }
</style>