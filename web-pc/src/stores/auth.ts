import { defineStore } from 'pinia'
import { api } from '@/api'
import type { User } from '@shared'
export const useAuthStore = defineStore('auth', {
  state: () => ({ user: null as User | null, permissions: [] as string[], lineIds: [] as number[], loaded: false }),
  actions: {
    async login(account:string,password:string){ await api.post('/auth/pc/login',{account,password}); await this.fetchMe() },
    async fetchMe(){ const {data}=await api.get('/me'); this.user=data.user; this.permissions=data.permissions; this.lineIds=data.lineIds; this.loaded=true },
    async logout(){ await api.post('/auth/logout'); this.user=null; this.permissions=[]; this.lineIds=[] },
    can(code:string){ return this.permissions.includes(code) || this.user?.role.code==='super_admin' }
  }
})
