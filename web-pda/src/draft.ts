/** 文件说明：PDA 本地草稿存储，使用 IndexedDB 保存未提交表单。 */

export interface DraftRecord { key:string; userId:number; versionId:number; lineId:number; payload:unknown; updatedAt:string }
const DB='bearing-inspection-pda', STORE='drafts'
// 打开或升级 IndexedDB；草稿以用户、版本和产线的组合 key 隔离。
function openDB():Promise<IDBDatabase>{return new Promise((resolve,reject)=>{const req=indexedDB.open(DB,1);req.onupgradeneeded=()=>{if(!req.result.objectStoreNames.contains(STORE))req.result.createObjectStore(STORE,{keyPath:'key'})};req.onsuccess=()=>resolve(req.result);req.onerror=()=>reject(req.error)})}
// saveDraft 覆盖同一表单上下文的最新本地草稿。
export async function saveDraft(record:DraftRecord){const db=await openDB();return new Promise<void>((resolve,reject)=>{const tx=db.transaction(STORE,'readwrite');tx.objectStore(STORE).put(record);tx.oncomplete=()=>resolve();tx.onerror=()=>reject(tx.error)})}
// getDraft 在重新进入表单时恢复未提交内容。
export async function getDraft(key:string):Promise<DraftRecord|undefined>{const db=await openDB();return new Promise((resolve,reject)=>{const req=db.transaction(STORE,'readonly').objectStore(STORE).get(key);req.onsuccess=()=>resolve(req.result);req.onerror=()=>reject(req.error)})}
// 提交成功后删除草稿，避免下次误恢复旧数据。
export async function deleteDraft(key:string){const db=await openDB();return new Promise<void>((resolve,reject)=>{const tx=db.transaction(STORE,'readwrite');tx.objectStore(STORE).delete(key);tx.oncomplete=()=>resolve();tx.onerror=()=>reject(tx.error)})}
