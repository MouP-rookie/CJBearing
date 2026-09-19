/** 文件说明：PDA Axios 客户端，统一转换后端错误信息。 */

import axios from 'axios'
export const api=axios.create({baseURL:'/api/v1',timeout:30000,withCredentials:true})
// 页面层只处理 Error.message，避免重复解析 Axios 响应结构。
api.interceptors.response.use(r=>r,e=>Promise.reject(new Error(e.response?.data?.message||e.message||'请求失败')))
