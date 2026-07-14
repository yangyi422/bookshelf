<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue';
import { ElMessage } from 'element-plus';
import { api } from '../api/client';

interface Info { books:number; files:number; data_dir:string; opds_url:string }
interface Backup { id:string; file_path:string; file_size:number; checksum:string; created_at:string }
interface Issue { code:string; book_id?:string; file_id?:string; path?:string; message:string }
interface Report { status:string; started_at:string; finished_at?:string; checked_books:number; checked_files:number; trash_entries:number; issues:Issue[] }
interface OPDSSettings { opds_enabled:boolean; opds_access_mode:string; opds_username:string; opds_password_configured:boolean; public_base_url:string; opds_url:string }

const info=ref<Info>(), backups=ref<Backup[]>([]), report=ref<Report>(), busy=ref('');
const password=reactive({current_password:'',new_password:''});
const opds=reactive({opds_enabled:true,opds_access_mode:'https_only',opds_username:'',opds_password:'',opds_password_configured:false,public_base_url:'',opds_url:'',confirm_insecure_http:false});
const warning='HTTP 不提供传输加密，OPDS 用户名、密码、书籍信息和下载内容可能被网络监听。请使用独立密码。';

async function load(){const[i,b,s,o]=await Promise.all([api.get<Info>('/system/info'),api.get<Backup[]>('/system/backups'),api.get<Report>('/system/scan/status'),api.get<OPDSSettings>('/system/opds')]);info.value=i.data;backups.value=b.data;report.value=s.data;Object.assign(opds,o.data,{opds_password:'',confirm_insecure_http:false})}
function toggleOPDS(enabled:boolean){if(enabled&&opds.opds_access_mode==='disabled')opds.opds_access_mode='https_only'}
async function saveOPDS(){if(opds.opds_access_mode==='http_and_https'&&!opds.confirm_insecure_http){ElMessage.error('请确认 HTTP 安全风险');return}busy.value='opds';try{const{data}=await api.put<OPDSSettings>('/system/opds',opds);Object.assign(opds,data,{opds_password:'',confirm_insecure_http:false});if(info.value)info.value.opds_url=data.opds_url;ElMessage.success('OPDS 设置已立即生效')}catch(e:any){ElMessage.error(e.response?.data?.error||'保存失败')}finally{busy.value=''}}
async function testOPDS(){if(!opds.opds_password){ElMessage.warning('请输入当前或新 OPDS 密码后测试');return}try{const{data}=await api.post('/system/opds/test',{username:opds.opds_username,password:opds.opds_password});ElMessage.success(`连通性测试通过：${data.opds_url}`)}catch(e:any){ElMessage.error(e.response?.data?.error||'连通性测试失败')}}
async function createBackup(){busy.value='backup';try{await api.post('/system/backups');ElMessage.success('备份已创建');backups.value=(await api.get<Backup[]>('/system/backups')).data}catch{ElMessage.error('备份失败')}finally{busy.value=''}}
async function scan(){busy.value='scan';try{report.value=(await api.post<Report>('/system/scan')).data;ElMessage.success(`扫描完成，发现 ${report.value.issues.length} 个问题`)}catch{ElMessage.error('扫描失败')}finally{busy.value=''}}
async function manifest(){const{data}=await api.get('/system/manifest');const blob=new Blob([JSON.stringify(data,null,2)],{type:'application/json'});const href=URL.createObjectURL(blob);const a=document.createElement('a');a.href=href;a.download='manifest.json';a.click();URL.revokeObjectURL(href)}
async function changePassword(){try{await api.post('/auth/change-password',password);ElMessage.success('密码已修改，请重新登录');location.href='/login'}catch(e:any){ElMessage.error(e.response?.data?.error||'修改失败')}}
function size(n:number){return(n/1024/1024).toFixed(1)+' MB'}
onMounted(load)
</script>

<template><section><h1>系统设置</h1><el-descriptions v-if="info" border :column="2"><el-descriptions-item label="书籍">{{info.books}}</el-descriptions-item><el-descriptions-item label="文件">{{info.files}}</el-descriptions-item><el-descriptions-item label="数据目录">{{info.data_dir}}</el-descriptions-item><el-descriptions-item label="OPDS">{{info.opds_url||'已关闭'}}</el-descriptions-item></el-descriptions><div class="panels">
<el-card><template #header><strong>OPDS 访问</strong></template><el-form label-position="top"><el-form-item label="启用 OPDS"><el-switch v-model="opds.opds_enabled" @change="toggleOPDS"/></el-form-item><template v-if="opds.opds_enabled"><el-form-item label="访问模式"><el-radio-group v-model="opds.opds_access_mode"><el-radio value="https_only">仅 HTTPS（推荐）</el-radio><el-radio value="http_and_https">HTTP 和 HTTPS</el-radio></el-radio-group></el-form-item><el-alert v-if="opds.opds_access_mode==='http_and_https'" :title="warning" type="warning" :closable="false"/><el-checkbox v-if="opds.opds_access_mode==='http_and_https'" v-model="opds.confirm_insecure_http">我已了解并确认上述风险</el-checkbox><el-form-item label="OPDS 用户名"><el-input v-model="opds.opds_username"/></el-form-item><el-form-item label="重置 OPDS 密码"><el-input v-model="opds.opds_password" type="password" show-password :placeholder="opds.opds_password_configured?'留空表示不修改':'请输入独立密码'"/></el-form-item></template><el-form-item label="公开访问地址（可留空）"><el-input v-model="opds.public_base_url" placeholder="https://books.example.com"/></el-form-item><p v-if="opds.opds_url">当前地址：<a :href="opds.opds_url" target="_blank">{{opds.opds_url}}</a></p><el-button type="primary" :loading="busy==='opds'" @click="saveOPDS">保存并立即生效</el-button><el-button :disabled="!opds.opds_enabled" @click="testOPDS">连通性测试</el-button></el-form></el-card>
<el-card><template #header><div class="head"><strong>备份</strong><el-button type="primary" :loading="busy==='backup'" @click="createBackup">创建备份</el-button></div></template><el-table :data="backups" empty-text="暂无备份"><el-table-column prop="created_at" label="创建时间"/><el-table-column label="大小"><template #default="s">{{size(s.row.file_size)}}</template></el-table-column><el-table-column prop="checksum" label="SHA-256" show-overflow-tooltip/></el-table></el-card>
<el-card><template #header><div class="head"><strong>一致性扫描</strong><div><el-button @click="manifest">导出 Manifest</el-button><el-button type="primary" :loading="busy==='scan'" @click="scan">开始扫描</el-button></div></div></template><template v-if="report"><p>状态：{{report.status}}；检查 {{report.checked_books}} 本书、{{report.checked_files}} 个文件；回收站目录 {{report.trash_entries}} 个。</p><el-table :data="report.issues" empty-text="未发现问题"><el-table-column prop="code" label="类型"/><el-table-column prop="path" label="相对路径" show-overflow-tooltip/><el-table-column prop="message" label="说明"/></el-table></template></el-card>
<el-card><template #header><strong>修改密码</strong></template><el-form label-position="top"><el-form-item label="当前密码"><el-input v-model="password.current_password" type="password" show-password/></el-form-item><el-form-item label="新密码"><el-input v-model="password.new_password" type="password" show-password/></el-form-item><el-button type="primary" @click="changePassword">修改密码</el-button></el-form></el-card>
</div></section></template>
<style scoped>.panels{display:grid;gap:1.5rem;margin-top:1.5rem}.head{display:flex;align-items:center;justify-content:space-between}.el-form{max-width:560px}.el-alert,.el-checkbox{margin-bottom:1rem}</style>
