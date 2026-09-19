<!-- 文件说明：PDA 动态表单填报页，负责字段渲染、草稿、班次、产品和提交。 -->

<template><div><div v-if="!online" class="offline-banner">网络已断开，只保留当前草稿，禁止切换和提交</div><van-nav-bar :title="name" left-text="返回" @click-left="goBack" /><div class="page"><div v-if="repeatRequired" class="form-item"><div class="field-label">重复首检原因</div><van-field v-model="repeatReason" type="textarea" rows="2" autosize placeholder="请输入原因" /></div><div v-for="field in fields" :key="field.key" class="form-item"><div class="field-label">{{ field.label }} <span v-if="field.unit">（{{ field.unit }}）</span></div><template v-if="field.type==='static_text'"><div>{{ field.defaultValue }}</div></template><template v-else-if="field.type==='single_select'"><van-radio-group v-model="values[field.key][0]"><van-radio v-for="o in field.options" :key="o.value" :name="o.value">{{ o.label }}</van-radio></van-radio-group></template><template v-else-if="field.type==='multi_select'"><van-checkbox-group v-model="multiValues[field.key]"><van-checkbox v-for="o in field.options" :key="o.value" :name="o.value">{{ o.label }}</van-checkbox></van-checkbox-group></template><template v-else-if="field.type==='product'"><van-field :model-value="values[field.key][0]" readonly is-link placeholder="请选择产品" @click="openProductPicker(field)" /><div v-if="!products.length" style="color:#b45309;font-size:12px;margin-top:4px">该产线未配置产品，请联系管理员维护</div></template><template v-else><van-field v-for="(_,idx) in values[field.key]" :key="idx" v-model="values[field.key][idx]" :type="field.type==='number'?'number':'text'" :placeholder="field.allowMultiple?`第${idx+1}个值`:'请输入'" @focus="onFocus(field,idx)" @blur="onBlur(field,idx)" /><van-button v-if="field.allowMultiple" size="small" plain @click="values[field.key].push(resolveDefault(field))">增加一条</van-button><div v-if="outOfRange(field,values[field.key][0])" style="color:#dc2626">超出配置范围，仍可提交</div></template></div><div class="actions"><van-button block plain :disabled="!online" @click="save">保存草稿</van-button><van-button block type="primary" :disabled="!online" :loading="submitting" @click="submit">人工提交</van-button></div><van-popup v-model:show="showProductPicker" position="bottom" round><van-picker :columns="productColumns" @confirm="onProductConfirm" @cancel="showProductPicker=false" /></van-popup></div></div></template>
<script setup lang="ts">
import { computed,onMounted,onUnmounted,reactive,ref } from 'vue'
import { useRoute,useRouter } from 'vue-router'
import { showConfirmDialog,showToast } from 'vant'
import { api } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { saveDraft,getDraft,deleteDraft } from '@/draft'
import { formatDateTime, type FieldDefinition } from '@shared'
const route=useRoute(),router=useRouter(),auth=useAuthStore(),online=ref(navigator.onLine),submitting=ref(false)
const name=String(route.query.name||'表单'),versionId=Number(route.params.id),lineId=Number(route.query.lineId),templateId=Number(route.query.templateId),kind=String(route.query.kind||'report'),lineName=String(route.query.lineName||'')
const fields=ref<FieldDefinition[]>([]),values=reactive<Record<string,string[]>>({}),multiValues=reactive<Record<string,string[]>>({}),repeatReason=ref(''),repeatRequired=ref(false),shiftItemId=ref(0),businessDate=ref(new Date().toLocaleDateString('sv-SE')),shifts=ref<any[]>([]),products=ref<any[]>([])
const isInspection=kind==='first_inspection'||kind==='final_inspection'
const draftKey=computed(()=>`${auth.user?.id||0}:${versionId}:${lineId}`)
const showProductPicker=ref(false),activeProductField=ref<FieldDefinition|null>(null)
const productColumns=computed(()=>products.value.map(p=>({text:`${p.name}（${p.code}）`,value:p.name})))
function openProductPicker(field:FieldDefinition){if(!products.value.length)return;activeProductField.value=field;showProductPicker.value=true}
function onProductConfirm(payload:any){const selected=payload?.selectedValues?.[0];if(activeProductField.value&&selected!=null)values[activeProductField.value.key][0]=String(selected);showProductPicker.value=false}
// 解析模板默认变量，例如当前日期、班次、用户和产线。
function resolveDefault(field:FieldDefinition){let v=field.defaultValue||'';const now=new Date();const map:Record<string,string>={'{{current_date}}':now.toLocaleDateString('sv-SE'),'{{current_time}}':formatDateTime(now),'{{current_shift}}':shifts.value.find(x=>x.id===shiftItemId.value)?.name||'','{{user_name}}':auth.user?.name||'','{{user_account}}':auth.user?.account||'','{{line_name}}':lineName,'{{line_product}}':products.value.map((p:any)=>p.name).join('、'), '{{form_name}}':name,'{{stage}}':kind==='first_inspection'?'首检':kind==='final_inspection'?'末检':''};for(const [k,v2] of Object.entries(map))v=v.replaceAll(k,v2);return v}
// 初始化每个字段的输入槽和多选默认值。
function initialize(){for(const f of fields.value){if(f.allowMultiple){values[f.key]=[resolveDefault(f)]}else{values[f.key]=[resolveDefault(f)]}multiValues[f.key]=f.type==='multi_select'&&f.defaultValue?f.defaultValue.split(','):[]}}
function onFocus(field:FieldDefinition,idx:number){const def=resolveDefault(field);if(values[field.key][idx]===def)values[field.key][idx]=''}
function onBlur(field:FieldDefinition,idx:number){if(!values[field.key][idx])values[field.key][idx]=resolveDefault(field)}
function outOfRange(field:FieldDefinition,value:string){const n=Number(value);if(!value||Number.isNaN(n))return false;return (field.min!==undefined&&n<field.min)||(field.max!==undefined&&n>field.max)}
// 同业务上下文已有填报时，读取最近值帮助现场快速补录。
async function loadLatest(){
  if(!shiftItemId.value) return
  try {
    const {data}=await api.get('/execution/submissions/current',{params:{lineId,versionId,businessDate:businessDate.value,shiftPlanItemId:shiftItemId.value}})
    if(!data.data) return
    const grouped:Record<string,any[]>={}
    for(const row of data.data.rows||[]){ for(const value of row.values||[]){ if(!grouped[value.fieldKey]) grouped[value.fieldKey]=[]; grouped[value.fieldKey].push(value) } }
    for(const field of fields.value){
      const list=grouped[field.key]||[]
      if(field.type==='multi_select'){ if(list.length){ multiValues[field.key]=list.map(value=>String(value.valueText ?? '')) } continue }
      if(field.allowMultiple){ values[field.key]=list.length?list.map(value=>value.valueNumber!=null?String(value.valueNumber):String(value.valueText??'')):[''] ; continue }
      const first=list[0]
      values[field.key]=first?[first.valueNumber!=null?String(first.valueNumber):String(first.valueText??'')]:[resolveDefault(field)]
    }
  } catch {}
}
// 加载产品、版本 Schema、班次和本地草稿，并判断是否重复首检。
async function load(){try{const prodRes=await api.get('/execution/products',{params:{lineId}});products.value=prodRes.data.items||[]}catch{}const {data}=await api.get(`/execution/forms/${versionId}`);const schema=typeof data.version.schema==='string'?JSON.parse(data.version.schema):data.version.schema;fields.value=(schema.fields||[]).sort((a:any,b:any)=>(a.sortOrder||0)-(b.sortOrder||0));const shiftRes=await api.get('/execution/shifts');shifts.value=shiftRes.data.items;shiftItemId.value=shiftRes.data.current?.id||shifts.value[0]?.id||0;repeatRequired.value=kind==='first_inspection'?Boolean((await api.get('/execution/repeat-check',{params:{lineId,businessDate:businessDate.value,shiftPlanItemId:shiftItemId.value}})).data.required):false;initialize();if(isInspection)await loadLatest();const draft=await getDraft(draftKey.value);if(draft){const payload=draft.payload as any || {}; for(const field of fields.value){ const raw=payload.values?.[field.key]; values[field.key]=Array.isArray(raw)?raw.map(value=>value==null?'':String(value)):[raw==null?resolveDefault(field):String(raw)]; const rawMulti=payload.multiValues?.[field.key]; multiValues[field.key]=Array.isArray(rawMulti)?rawMulti.map(value=>String(value)):[] } if(typeof payload.repeatReason==='string')repeatReason.value=payload.repeatReason; if(typeof payload.businessDate==='string')businessDate.value=payload.businessDate; if(Number(payload.shiftItemId))shiftItemId.value=Number(payload.shiftItemId);showToast('已恢复本地草稿')}}
function updateOnline(){online.value=navigator.onLine}
// 草稿只存浏览器本地，不写入服务器数据库。
async function save(){const payload={values:JSON.parse(JSON.stringify(values)),multiValues:JSON.parse(JSON.stringify(multiValues)),repeatReason:repeatReason.value,businessDate:businessDate.value,shiftItemId:shiftItemId.value};await saveDraft({key:draftKey.value,userId:auth.user?.id||0,versionId,lineId,payload,updatedAt:new Date().toISOString()});showToast('草稿已保存')}
// 提交必须在线；字段值铺平后由后端事务写入填报。
async function submit(){
  if(!online.value){showToast('断网时不能提交');return}
  await showConfirmDialog({title:'确认提交',message:'提交后日期和班次将锁定'})
  submitting.value=true
  try {
    const flatValues:any[]=[]
    for(const field of fields.value){
      if(field.type==='multi_select'){
        const selected=Array.isArray(multiValues[field.key])?multiValues[field.key]:[]
        selected.forEach((value,index)=>flatValues.push({fieldKey:field.key,valueType:field.type,valueText:String(value),repeatIndex:index,source:'manual'}))
        continue
      }
      const raw=values[field.key]
      const list=Array.isArray(raw)?raw:[raw ?? '']
      list.forEach((value,index)=>{
        const text=value == null ? '' : String(value)
        const number=field.type==='number' && text!=='' && !Number.isNaN(Number(text)) ? Number(text) : undefined
        flatValues.push({fieldKey:field.key,valueType:field.type,valueText:text,valueNumber:number,repeatIndex:index,source:text===resolveDefault(field)?'default':'manual'})
      })
    }
    await api.post('/execution/submissions',{templateId,versionId,lineId,businessDate:businessDate.value,shiftPlanItemId:shiftItemId.value,repeatReason:repeatReason.value,source:'pda',clientTime:new Date().toISOString(),rows:[{rowNo:1,values:flatValues}]})
    await deleteDraft(draftKey.value)
    showToast('提交成功')
    router.replace({ path: '/', query: { lineId: String(lineId) } })
  } catch(e:any){showToast(e.message)} finally {submitting.value=false}
}
function goBack(){router.back()}
onMounted(()=>{window.addEventListener('online',updateOnline);window.addEventListener('offline',updateOnline);load()})
onUnmounted(()=>{window.removeEventListener('online',updateOnline);window.removeEventListener('offline',updateOnline)})
</script>
<style scoped>.actions{display:flex;flex-direction:column;gap:12px;margin:20px 0 40px}</style>
