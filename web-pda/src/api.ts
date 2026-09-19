import axios from 'axios'
export const api=axios.create({baseURL:'/api/v1',timeout:30000,withCredentials:true})
api.interceptors.response.use(r=>r,e=>Promise.reject(new Error(e.response?.data?.message||e.message||'请求失败')))
