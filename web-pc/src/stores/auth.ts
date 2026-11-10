/** 文件说明：PC 登录状态、权限、部门和品牌状态管理。 */

import { defineStore } from 'pinia'
import { api } from '@/api'
import type { Department, User } from '@shared'
export const useAuthStore = defineStore('auth', {
  state: () => ({
    user: null as User | null,
    permissions: [] as string[],
    lineIds: [] as number[],
    loaded: false,
    isSuperAdmin: false,
    departments: [] as Department[],
    selectedDepartmentId: 0,
    brandLogoDataUrl: '',
  }),
  getters: {
    allDepartments: state => state.isSuperAdmin && state.selectedDepartmentId === 0,
    currentDepartment: state => state.user?.department || state.departments.find(item => item.id === state.selectedDepartmentId) || null,
  },
  actions: {
    async login(account:string,password:string){ await api.post('/auth/pc/login',{account,password}); await this.fetchMe() },
    // fetchMe 绑定用户、权限、产线、部门和品牌信息，是所有页面的权限来源。
    async fetchMe(){
      const {data}=await api.get('/me', { params: { client: 'pc' } })
      this.user=data.user
      this.permissions=data.permissions
      this.lineIds=data.lineIds
      this.isSuperAdmin=Boolean(data.isSuperAdmin)
      this.departments=data.departments || []
      this.selectedDepartmentId=Number(data.selectedDepartmentId || 0)
      this.brandLogoDataUrl=data.brandLogoDataUrl || ''
      if (this.isSuperAdmin) localStorage.setItem('bearing-inspection.department', String(this.selectedDepartmentId || 0))
      else localStorage.removeItem('bearing-inspection.department')
      this.loaded=true
    },
    // 部门切换后重新获取 /me，后端会按新范围返回权限和产线。
    async selectDepartment(id:number){
      if (!this.isSuperAdmin) return
      this.selectedDepartmentId=id
      localStorage.setItem('bearing-inspection.department', String(id || 0))
      await this.fetchMe()
    },
    async logout(){ await api.post('/auth/logout'); this.user=null; this.permissions=[]; this.lineIds=[]; this.departments=[]; this.selectedDepartmentId=0; this.brandLogoDataUrl=''; this.loaded=false; localStorage.removeItem('bearing-inspection.department') },
    can(code:string){ return this.permissions.includes(code) || this.user?.role.code==='super_admin' }
  }
})
