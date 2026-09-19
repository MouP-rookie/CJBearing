import axios from 'axios'
export const api = axios.create({ baseURL:'/api/v1', timeout:30000, withCredentials:true })
api.interceptors.request.use(config => {
  const departmentId = typeof localStorage === 'undefined' ? '' : localStorage.getItem('bearing-inspection.department')
  if (departmentId) config.headers['X-Department-ID'] = departmentId
  return config
})
api.interceptors.response.use(r=>r, error=>{ const message=error.response?.data?.message||error.message||'请求失败'; return Promise.reject(new Error(message)) })
