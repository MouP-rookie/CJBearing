import { defineStore } from 'pinia'
import { api } from '@/api'
import type { User } from '@shared'
export const useAuthStore=defineStore('auth',{state:()=>({user:null as User|null,loaded:false}),actions:{async login(account:string){await api.post('/auth/pda/login',{account});await this.fetchMe()},async fetchMe(){const {data}=await api.get('/me');this.user=data.user;this.loaded=true},async logout(){await api.post('/auth/logout');this.user=null;this.loaded=false}}})
