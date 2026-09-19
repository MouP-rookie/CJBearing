import { createRouter, createWebHashHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import MainLayout from '@/layouts/MainLayout.vue'
import LoginView from '@/views/LoginView.vue'
import DashboardView from '@/views/DashboardView.vue'
import ExecutionView from '@/views/ExecutionView.vue'
import UsersView from '@/views/UsersView.vue'
import LinesView from '@/views/LinesView.vue'
import ProductsView from '@/views/ProductsView.vue'
import ShiftsView from '@/views/ShiftsView.vue'
import FormsView from '@/views/FormsView.vue'
import FormAssignmentsView from '@/views/FormAssignmentsView.vue'
import LogsView from '@/views/LogsView.vue'
import BackupsView from '@/views/BackupsView.vue'
import SettingsView from '@/views/SettingsView.vue'
import VoidReasonsView from '@/views/VoidReasonsView.vue'
const router=createRouter({history:createWebHashHistory(),routes:[
 {path:'/login',component:LoginView},
 {path:'/',component:MainLayout,children:[
  {path:'',component:DashboardView},{path:'execution',component:ExecutionView},{path:'users',component:UsersView},{path:'lines',component:LinesView},{path:'products',component:ProductsView},{path:'shifts',component:ShiftsView},{path:'assignments',component:FormAssignmentsView},{path:'forms',component:FormsView},{path:'logs',component:LogsView},{path:'backups',component:BackupsView},{path:'settings',component:SettingsView},{path:'void-reasons',component:VoidReasonsView}
 ]}
]})
router.beforeEach(async (to)=>{ const auth=useAuthStore(); if(to.path==='/login') return true; if(!auth.loaded){ try{await auth.fetchMe()}catch{ return '/login' } } })
export default router


