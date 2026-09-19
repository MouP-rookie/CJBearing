/** 文件说明：PC Axios 客户端，自动附加部门 ID 并统一转换错误信息。 */

import axios from 'axios'
export const api = axios.create({ baseURL:'/api/v1', timeout:30000, withCredentials:true })
// 超级管理员切换部门后，通过请求头把当前部门上下文传给后端。
api.interceptors.request.use(config => {
  const departmentId = typeof localStorage === 'undefined' ? '' : localStorage.getItem('bearing-inspection.department')
  if (departmentId) config.headers['X-Department-ID'] = departmentId
  return config
})
// 统一把后端 message 转换为 Error，页面只需展示 error.message。
api.interceptors.response.use(r=>r, error=>{ const message=error.response?.data?.message||error.message||'请求失败'; return Promise.reject(new Error(message)) })
