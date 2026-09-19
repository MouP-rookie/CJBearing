/** 文件说明：PDA 路由表和登录守卫。 */

import { createRouter,createWebHashHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import LoginView from '@/views/LoginView.vue'
import HomeView from '@/views/HomeView.vue'
import FormView from '@/views/FormView.vue'
import HistoryView from '@/views/HistoryView.vue'
// PDA 使用 hash 路由，方便现场设备和静态资源部署。
const router=createRouter({history:createWebHashHistory(),routes:[{path:'/login',component:LoginView},{path:'/',component:HomeView},{path:'/form/:id',component:FormView},{path:'/history',component:HistoryView}]})
// 路由守卫首次进入时通过 /me 校验登录状态。
router.beforeEach(async to=>{const auth=useAuthStore();if(to.path==='/login')return true;if(!auth.loaded){try{await auth.fetchMe()}catch{return '/login'}}})
export default router

