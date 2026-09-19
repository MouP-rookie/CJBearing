import { createRouter,createWebHashHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import LoginView from '@/views/LoginView.vue'
import HomeView from '@/views/HomeView.vue'
import FormView from '@/views/FormView.vue'
import HistoryView from '@/views/HistoryView.vue'
const router=createRouter({history:createWebHashHistory(),routes:[{path:'/login',component:LoginView},{path:'/',component:HomeView},{path:'/form/:id',component:FormView},{path:'/history',component:HistoryView}]})
router.beforeEach(async to=>{const auth=useAuthStore();if(to.path==='/login')return true;if(!auth.loaded){try{await auth.fetchMe()}catch{return '/login'}}})
export default router

