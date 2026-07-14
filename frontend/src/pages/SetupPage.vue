<script setup lang="ts">
import { reactive, ref } from 'vue';
import { useRouter } from 'vue-router';
import { ElMessage } from 'element-plus';
import { api } from '../api/client';

const router = useRouter();
const saving = ref(false);
const form = reactive({
  admin_username: '', admin_password: '', opds_enabled: true,
  opds_access_mode: 'https_only', opds_username: '', opds_password: '',
  public_base_url: '', confirm_insecure_http: false,
});
const warning = 'HTTP 不提供传输加密，OPDS 用户名、密码、书籍信息和下载内容可能被网络监听。请使用独立密码。';

async function submit() {
  if (form.opds_access_mode === 'http_and_https' && !form.confirm_insecure_http) {
    ElMessage.error('请先确认 HTTP 安全风险'); return;
  }
  saving.value = true;
  try {
    await api.post('/setup', form);
    ElMessage.success('初始化完成，请登录');
    await router.push('/login');
  } catch (error: any) {
    ElMessage.error(error.response?.data?.error || '初始化失败');
  } finally { saving.value = false; }
}
</script>

<template>
  <main>
    <el-card>
      <h1>初始化 Bookshelf</h1>
      <el-form label-position="top" @submit.prevent="submit">
        <el-form-item label="管理员用户名" required><el-input v-model="form.admin_username" /></el-form-item>
        <el-form-item label="管理员密码" required><el-input v-model="form.admin_password" type="password" show-password /></el-form-item>
        <el-divider>OPDS</el-divider>
        <el-form-item label="启用 OPDS"><el-switch v-model="form.opds_enabled" /></el-form-item>
        <template v-if="form.opds_enabled">
          <el-form-item label="OPDS 访问模式" required>
            <el-radio-group v-model="form.opds_access_mode">
              <el-radio value="https_only">仅 HTTPS（推荐）</el-radio>
              <el-radio value="http_and_https">HTTP 和 HTTPS</el-radio>
            </el-radio-group>
          </el-form-item>
          <el-alert v-if="form.opds_access_mode==='http_and_https'" :title="warning" type="warning" :closable="false" />
          <el-checkbox v-if="form.opds_access_mode==='http_and_https'" v-model="form.confirm_insecure_http">我已了解并确认上述风险</el-checkbox>
          <el-form-item label="OPDS 用户名" required><el-input v-model="form.opds_username" /></el-form-item>
          <el-form-item label="OPDS 密码" required><el-input v-model="form.opds_password" type="password" show-password /></el-form-item>
        </template>
        <el-form-item label="公开访问地址（可留空）"><el-input v-model="form.public_base_url" placeholder="https://books.example.com" /></el-form-item>
        <el-button type="primary" native-type="submit" :loading="saving">完成初始化</el-button>
      </el-form>
    </el-card>
  </main>
</template>

<style scoped>main{max-width:42rem;margin:5vh auto}.el-alert,.el-checkbox{margin-bottom:1rem}</style>
