/** 文件说明：PC Axios 客户端，自动附加部门 ID 并统一转换错误信息。 */

import axios from 'axios'
export const api = axios.create({ baseURL:'/api/v1', timeout:30000, withCredentials:true })
export const departmentStorageKey = 'bearing-inspection.department'
// 默认使用顶部部门范围；页面为具体操作指定部门时保留该请求的范围。
api.interceptors.request.use(config => {
  const departmentId = typeof sessionStorage === 'undefined' ? '' : sessionStorage.getItem(departmentStorageKey)
  if (departmentId && !config.headers.has('X-Department-ID')) config.headers.set('X-Department-ID', departmentId)
  return config
})
// 统一把后端 message 转换为 Error，页面只需展示 error.message。
api.interceptors.response.use(r=>r, error=>{ const message=error.response?.data?.message||error.message||'请求失败'; return Promise.reject(Object.assign(new Error(message), { status: error.response?.status, code: error.response?.data?.code })) })
