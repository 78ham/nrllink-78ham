<template>
  <div class="page">
    <div class="toolbar">
      <h2>用户管理</h2>
      <div class="toolbar-right">
        <span v-if="isDefaultAdmin" class="tip">建议先创建您自己的管理员账号</span>
        <n-button :type="isDefaultAdmin ? 'warning' : 'primary'" @click="showModal = true">新建用户</n-button>
      </div>
    </div>
    <n-data-table :columns="columns" :data="users" :bordered="false" :loading="loading" :row-key="(r: any) => r.id" />

    <n-modal v-model:show="showModal" preset="card" title="新建用户" style="width:440px;" :mask-closable="false">
      <n-form ref="formRef" :model="form" :rules="rules" label-placement="left" label-width="72">
        <n-form-item label="姓名" path="name">
          <n-input v-model:value="form.name" placeholder="请输入姓名" />
        </n-form-item>
        <n-form-item label="呼号" path="callsign">
          <n-input v-model:value="form.callsign" placeholder="请输入呼号" />
        </n-form-item>
        <n-form-item label="手机号" path="phone">
          <n-input v-model:value="form.phone" placeholder="请输入手机号" />
        </n-form-item>
        <n-form-item label="密码" path="password">
          <n-input v-model:value="form.password" type="password" show-password-on="click" placeholder="至少8位，含大小写字母+数字+特殊字符" />
        </n-form-item>
        <p class="pwd-hint">密码规则：≥8位，须包含大小写字母、数字和特殊字符，且不能包含 nrl1234、nrl888、admin123、password、12345678 等弱密码。</p>
        <n-form-item label="角色" path="roles">
          <n-select v-model:value="form.roles" multiple :options="roleOptions" placeholder="请选择角色" />
        </n-form-item>
        <n-form-item label="状态" path="status">
          <n-select v-model:value="form.status" :options="[{ label: '正常', value: 1 }, { label: '禁用', value: 0 }]" />
        </n-form-item>
      </n-form>
      <template #footer>
        <div style="display:flex;justify-content:flex-end;gap:8px;">
          <n-button @click="showModal = false">取消</n-button>
          <n-button type="primary" :loading="submitting" @click="handleCreate">创建</n-button>
        </div>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import { useRouter } from 'vue-router'
import { NDataTable, NTag, NButton, NModal, NForm, NFormItem, NInput, NSelect, NPopconfirm, useMessage } from 'naive-ui'
import { useUserStore } from '../stores/user'

const userStore = useUserStore()
const router = useRouter()
const message = useMessage()

const loading = ref(false)
const submitting = ref(false)
const showModal = ref(false)
const formRef = ref()
const users = ref<any[]>([])

const isDefaultAdmin = computed(() => userStore.user?.default_admin === true)

const roleLabel: Record<string, string> = { admin: '管理员', ham: 'HAM用户', view: '观察员' }

const roleOptions = [
  { label: '管理员', value: 'admin' },
  { label: 'HAM用户', value: 'ham' },
  { label: '观察员', value: 'view' },
]

const form = ref({
  name: '',
  callsign: '',
  phone: '',
  password: '',
  roles: ['ham'] as string[],
  status: 1,
})

const rules = {
  name: [{ required: true, message: '请输入姓名', trigger: 'blur' }],
  callsign: [{ required: true, message: '请输入呼号', trigger: 'blur' }],
  phone: [{ required: true, message: '请输入手机号', trigger: 'blur' }],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 8, message: '密码长度不能少于8位', trigger: 'blur' }
  ],
  roles: [{ required: true, type: 'array' as const, min: 1, message: '请选择角色', trigger: ['blur', 'change'] }],
}

const columns: any[] = [
  { title: 'ID', key: 'id', width: 70 },
  { title: '呼号', key: 'callsign' },
  { title: '姓名', key: 'name' },
  { title: '手机号', key: 'phone' },
  {
    title: '角色', key: 'roles',
    render: (r: any) => (r.roles || []).map((role: string) =>
      h(NTag, { size: 'small', type: role === 'admin' ? 'warning' : 'info', style: 'margin-right:4px;' }, { default: () => roleLabel[role] || role })
    )
  },
  {
    title: '状态', key: 'status',
    render: (r: any) => h(NTag, { type: r.status === 1 ? 'success' : 'default' }, { default: () => r.status === 1 ? '正常' : '禁用' })
  },
  {
    title: '操作', key: 'actions',
    render: (r: any) => h(NPopconfirm, {
      positiveText: '删除',
      negativeText: '取消',
      onPositiveClick: () => handleDelete(r)
    }, {
      trigger: () => h(NButton, {
        size: 'small',
        quaternary: true,
        type: 'error',
      }, { default: () => '删除' }),
      default: () => `确定要删除用户「${r.callsign || r.name}」吗？此操作不可恢复。`
    })
  },
]

async function loadUsers() {
  loading.value = true
  try {
    const data = await userStore.fetchUsers()
    users.value = data?.items || []
  } catch (e: any) {
    message.error(e.message || '获取用户列表失败')
  } finally {
    loading.value = false
  }
}

async function handleCreate() {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }
  submitting.value = true
  try {
    const data = await userStore.createUser({
      name: form.value.name,
      callsign: form.value.callsign,
      phone: form.value.phone,
      password: form.value.password,
      roles: form.value.roles,
      status: form.value.status,
      sex: 0,
    })
    message.success(data?.message || '新增用户成功')
    showModal.value = false
    form.value = { name: '', callsign: '', phone: '', password: '', roles: ['ham'], status: 1 }
    await loadUsers()
  } catch (e: any) {
    message.error(e.message || '新增用户失败')
  } finally {
    submitting.value = false
  }
}

async function handleDelete(row: any) {
  try {
    const data = await userStore.deleteUser(row.id)
    message.success(data?.message || '用户删除成功')
    if (row.id === userStore.user?.id) {
      userStore.logout()
      router.push('/login')
      return
    }
    await loadUsers()
  } catch (e: any) {
    message.error(e.message || '删除用户失败')
  }
}

onMounted(loadUsers)
</script>

<style scoped>
.page { padding: 24px; }
.toolbar { display: flex; align-items: center; justify-content: space-between; margin-bottom: 12px; }
.toolbar-right { display: flex; align-items: center; gap: 10px; }
.tip { color: var(--accent-warning); font-size: 12px; }
.pwd-hint { margin: 0 0 12px; font-size: 12px; line-height: 1.6; color: var(--text-secondary); }
</style>
