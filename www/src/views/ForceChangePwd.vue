<template>
  <div class="force-container">
    <div class="card">
      <h2>首次登录 - 请修改密码</h2>
      <p class="hint">首次登录请修改密码。</p>
      <n-form ref="formRef" :model="form" :rules="rules">
        <n-form-item path="password">
          <n-input v-model:value="form.password" type="password" placeholder="新密码（至少8位，含大小写+数字+特殊字符）" size="large" />
        </n-form-item>
        <n-form-item path="confirm">
          <n-input v-model:value="form.confirm" type="password" placeholder="确认新密码" size="large" />
        </n-form-item>
        <n-button type="primary" block size="large" :loading="loading" @click="handleSubmit">
          修改密码
        </n-button>
      </n-form>
      <p v-if="errorMsg" class="error">{{ errorMsg }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { NForm, NFormItem, NInput, NButton } from 'naive-ui'
import { useUserStore } from '../stores/user'

const router = useRouter()
const userStore = useUserStore()
const formRef = ref()
const loading = ref(false)
const errorMsg = ref('')

const form = reactive({ password: '', confirm: '' })
const rules = {
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 8, message: '密码长度不能少于8位', trigger: 'blur' }
  ],
  confirm: [
    { required: true, message: '请确认密码', trigger: 'blur' },
    { validator: (_: any, v: string) => v === form.password, message: '两次密码不一致', trigger: 'blur' }
  ]
}

async function handleSubmit() {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }
  loading.value = true
  errorMsg.value = ''
  try {
    await userStore.changePassword(form.password)
    await userStore.logout()
    router.push('/login')
  } catch (e: any) {
    errorMsg.value = e.message || '修改失败'
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.force-container {
  display: flex; align-items: center; justify-content: center;
  height: 100vh; background: var(--bg-primary);
}
.card {
  width: 400px; padding: 40px;
  background: var(--bg-card); border-radius: 12px;
  border: 1px solid var(--border);
}
h2 { text-align: center; color: var(--accent-warning); margin: 0 0 8px; }
.hint { text-align: center; color: var(--text-secondary); margin: 0 0 24px; font-size: 13px; }
.error { color: var(--offline); text-align: center; margin-top: 16px; }
</style>
