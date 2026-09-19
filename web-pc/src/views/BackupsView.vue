<template><div class="page"><h2 class="page-title">数据库备份</h2><div class="toolbar"><el-button type="primary" @click="create">生成完整备份</el-button><el-button @click="load">刷新列表</el-button></div><el-table :data="items" border><el-table-column prop="fileName" label="文件名" /><el-table-column prop="fileSize" label="大小" width="140"><template #default="{row}">{{ formatSize(row.fileSize) }}</template></el-table-column><el-table-column prop="createdBy" label="备份人ID" width="120" /><el-table-column label="生成时间" width="220"><template #default="{ row }">{{ formatDateTime(row.createdAt) }}</template></el-table-column><el-table-column label="操作" width="140"><template #default="{row}"><el-button link @click="download(row)">下载</el-button></template></el-table-column></el-table></div></template>
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '@/api'
import { formatDateTime } from '@shared'
const items=ref<any[]>([])
async function load(){const {data}=await api.get('/admin/backups');items.value=data.items}
async function create(){await api.post('/admin/backups');ElMessage.success('备份已生成');load()}
async function download(row:any){const res=await api.get(`/admin/backups/${row.id}/download`,{responseType:'blob'});const url=URL.createObjectURL(res.data);const a=document.createElement('a');a.href=url;a.download=row.fileName;a.click();URL.revokeObjectURL(url)}
function formatSize(n:number){if(n<1024)return `${n} B`;if(n<1024*1024)return `${(n/1024).toFixed(1)} KB`;return `${(n/1024/1024).toFixed(1)} MB`}
onMounted(load)
</script>
