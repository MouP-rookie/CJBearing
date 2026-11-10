/** 文件说明：PDA 路由表和登录守卫。 */

import { createRouter,createWebHashHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import LoginView from '@/views/LoginView.vue'
import HomeView from '@/views/HomeView.vue'
import FormView from '@/views/FormView.vue'
import HistoryView from '@/views/HistoryView.vue'
import SessionErrorView from '../../packages/shared/src/SessionErrorView.vue'
// PDA 使用 hash 路由，方便现场设备和静态资源部署。
const router=createRouter({history:createWebHashHistory(),routes:[{path:'/login',component:LoginView},{path:'/session-error',component:SessionErrorView,props:route=>({message:String(route.query.message || '请稍后重试'),forbidden:route.query.forbidden==='1'})},{path:'/',component:HomeView},{path:'/form/:id',component:FormView},{path:'/history',component:HistoryView}]})
// 复用门户或 PC 登录；在挂载登录页前确认身份，避免工号自动登录覆盖统一账号。
router.beforeEach(async to => {
  if (to.path === '/session-error') return true
  const auth = useAuthStore()
  try {
    await auth.fetchMe()
    return to.path === '/login' ? '/' : true
  } catch (error: any) {
    auth.loaded = false
    auth.user = null
    if (error.status === 401) return to.path === '/login' ? true : '/login'
    return { path: '/session-error', query: { message: error.message, forbidden: error.status === 403 ? '1' : '0' } }
  }
})
export default router

