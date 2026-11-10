/** 文件说明：PC 路由表和登录守卫。 */

import { createRouter, createWebHashHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import MainLayout from '@/layouts/MainLayout.vue'
import LoginView from '@/views/LoginView.vue'
import DashboardView from '@/views/DashboardView.vue'
import ExecutionView from '@/views/ExecutionView.vue'
import UsersView from '@/views/UsersView.vue'
import RolesView from '@/views/RolesView.vue'
import LinesView from '@/views/LinesView.vue'
import ProductsView from '@/views/ProductsView.vue'
import ShiftsView from '@/views/ShiftsView.vue'
import FormsView from '@/views/FormsView.vue'
import FormAssignmentsView from '@/views/FormAssignmentsView.vue'
import LogsView from '@/views/LogsView.vue'
import BackupsView from '@/views/BackupsView.vue'
import DepartmentsView from '@/views/DepartmentsView.vue'
import SettingsView from '@/views/SettingsView.vue'
import VoidReasonsView from '@/views/VoidReasonsView.vue'
import SessionErrorView from '../../packages/shared/src/SessionErrorView.vue'
// PC 端使用 hash 路由，刷新页面时不需要服务端额外重写配置。
const router=createRouter({history:createWebHashHistory(),routes:[
 {path:'/login',component:LoginView},
 {path:'/session-error',component:SessionErrorView,props:route=>({message:String(route.query.message || '请稍后重试'),forbidden:route.query.forbidden==='1'})},
 {path:'/',component:MainLayout,children:[
  {path:'',component:DashboardView},{path:'execution',component:ExecutionView},{path:'users',component:UsersView},{path:'roles',component:RolesView},{path:'departments',component:DepartmentsView},{path:'lines',component:LinesView},{path:'products',component:ProductsView},{path:'shifts',component:ShiftsView},{path:'assignments',component:FormAssignmentsView},{path:'forms',component:FormsView},{path:'logs',component:LogsView},{path:'backups',component:BackupsView},{path:'settings',component:SettingsView},{path:'void-reasons',component:VoidReasonsView}
 ]}
]})
// 每次导航都确认统一登录状态；已有门户登录时直接跳过子系统登录页。
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


