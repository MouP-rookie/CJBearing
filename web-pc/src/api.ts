import axios from 'axios'
export const api = axios.create({ baseURL:'/api/v1', timeout:30000, withCredentials:true })
api.interceptors.response.use(r=>r, error=>{ const message=error.response?.data?.message||error.message||'请求失败'; return Promise.reject(new Error(message)) })
