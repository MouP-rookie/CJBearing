/** 文件说明：PDA 登录状态和品牌信息管理。 */

import { defineStore } from 'pinia'
import { api } from '@/api'
import type { User } from '@shared'
export const useAuthStore=defineStore('auth',{
  state:()=>({user:null as User|null,loaded:false,brandLogoDataUrl:''}),
  actions:{
    async login(account:string){await api.post('/auth/pda/login',{account});await this.fetchMe()},
    // fetchMe 从后端获取用户和品牌信息，是 PDA 登录态的唯一来源。
    async fetchMe(){const {data}=await api.get('/me');this.user=data.user;this.brandLogoDataUrl=data.brandLogoDataUrl||'';this.loaded=true},
    async logout(){await api.post('/auth/logout');this.user=null;this.brandLogoDataUrl='';this.loaded=false}
  }
})
