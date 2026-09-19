export type ClientType = 'pc' | 'pda'
export interface Department { id:number; code:string; name:string; enabled:boolean; createdAt?:string; updatedAt?:string }
export interface User { id:number; account:string; name:string; enabled:boolean; role:Role; departmentId?:number; department?:Department }
export interface Role { id:number; code:string; name:string; allowPc:boolean; allowPda:boolean }
export interface Line { id:number; departmentId:number; department?:Department; parentId?:number; code:string; name:string; level:number; enabled:boolean }
export interface Product { id:number; departmentId:number; department?:Department; code:string; name:string; enabled:boolean; lines?:Line[] }
export interface FieldOption { label:string; value:string }
export interface FieldDefinition { key:string; label:string; type:'text'|'number'|'single_select'|'multi_select'|'datetime'|'static_text'|'product'; required?:boolean; options?:FieldOption[]; defaultValue?:string; unit?:string; min?:number; max?:number; allowMultiple?:boolean; repeatable?:boolean; sortOrder?:number }
export interface FormSchema { fields:FieldDefinition[] }
export interface FormDefinition { templateId:number; code:string; name:string; kind:string; versionId:number; versionNo:number; schema:FormSchema }
export interface SubmissionValueInput { fieldKey:string; valueType:string; valueText?:string; valueNumber?:number; repeatIndex?:number; source?:'manual'|'default'|'instrument' }
export interface SubmissionRowInput { rowNo?:number; operatorId?:number; operatedAt?:string; values:SubmissionValueInput[] }
export interface SubmissionInput { templateId:number; versionId:number; lineId:number; businessDate:string; shiftPlanItemId:number; repeatReason?:string; source?:string; clientTime?:string; rows:SubmissionRowInput[] }
export interface ApiError { code:string; message:string; details?:unknown; requestId?:string }
export function formatDateTime(value: string | Date | null | undefined): string {
  if (!value) return '-'
  const date = value instanceof Date ? value : new Date(value)
  if (Number.isNaN(date.getTime())) return String(value)
  const pad = (part: number) => String(part).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}
