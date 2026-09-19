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
    async fetchMe(){
      const {data}=await api.get('/me')
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
