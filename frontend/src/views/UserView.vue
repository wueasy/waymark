<template>
  <div>
    <el-card class="mb">
      <template #header>
        <div class="card-header">
          <span>用户管理</span>
          <div class="toolbar">
            <el-button @click="loadUsers">刷新</el-button>
            <el-button type="primary" @click="openCreate">新建用户</el-button>
          </div>
        </div>
      </template>

      <el-skeleton v-if="loading" animated class="table-skeleton">
        <template #template>
          <div v-for="i in 5" :key="i" class="table-skeleton-row">
            <el-skeleton-item variant="text" :style="{ flex: 16 }" />
            <el-skeleton-item variant="text" :style="{ flex: 16 }" />
            <el-skeleton-item variant="text" :style="{ flex: 20 }" />
            <el-skeleton-item variant="text" :style="{ flex: 10 }" />
            <el-skeleton-item variant="text" :style="{ flex: 17 }" />
            <el-skeleton-item variant="text" :style="{ flex: 22 }" />
          </div>
        </template>
      </el-skeleton>
      <el-table v-else :data="users" size="small" empty-text="暂无用户">
        <el-table-column prop="username" label="用户名" min-width="160" />
        <el-table-column prop="nickname" label="昵称" min-width="160" />
        <el-table-column label="角色" min-width="200">
          <template #default="{ row }">
            <el-tag v-for="role in row.roles" :key="role.id" size="small" class="role-tag">
              {{ role.name }}
            </el-tag>
            <span v-if="!row.roles || !row.roles.length" class="meta">未分配</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag size="small" :type="row.status === 1 ? 'success' : 'info'">
              {{ row.status === 1 ? '启用' : '禁用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="170">
          <template #default="{ row }">{{ formatTime(row.createTime) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="220">
          <template #default="{ row }">
            <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
            <el-button link type="primary" @click="openPassword(row)">重置密码</el-button>
            <el-popconfirm title="确认删除该用户？" @confirm="remove(row)">
              <template #reference>
                <el-button link type="danger">删除</el-button>
              </template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="editVisible" :title="editing ? '编辑用户' : '新建用户'" width="480px">
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="90px">
        <el-form-item label="用户名" prop="username">
          <el-input v-model="form.username" :disabled="editing" placeholder="登录账号" />
        </el-form-item>
        <el-form-item v-if="!editing" label="密码" prop="password">
          <el-input v-model="form.password" type="password" show-password placeholder="登录密码" />
        </el-form-item>
        <el-form-item label="昵称">
          <el-input v-model="form.nickname" placeholder="默认与用户名相同" />
        </el-form-item>
        <el-form-item label="角色">
          <el-select v-model="form.roleIds" multiple style="width: 100%" placeholder="可分配多个角色">
            <el-option v-for="role in roles" :key="role.id" :label="role.name" :value="role.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-switch v-model="form.enabled" active-text="启用" inactive-text="禁用" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submit">确定</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="passwordVisible" title="重置密码" width="420px">
      <el-form ref="passwordFormRef" :model="passwordForm" :rules="passwordRules" label-width="70px">
        <el-form-item label="新密码" prop="password">
          <el-input v-model="passwordForm.password" type="password" show-password placeholder="请输入新密码" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="passwordVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitPassword">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import {
  createUser,
  deleteUser,
  listRoles,
  listUsers,
  resetPassword,
  updateUser
} from '../api'
import { formatTime } from '../utils'

const users = ref([])
const roles = ref([])
const loading = ref(false)

const editVisible = ref(false)
const editing = ref(false)
const submitting = ref(false)
const formRef = ref()
const form = reactive({ id: 0, username: '', password: '', nickname: '', roleIds: [], enabled: true })

const formRules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }]
}

const passwordVisible = ref(false)
const passwordFormRef = ref()
const passwordForm = reactive({ id: 0, password: '' })
const passwordRules = {
  password: [{ required: true, message: '请输入新密码', trigger: 'blur' }]
}

onMounted(async () => {
  await Promise.all([loadUsers(), loadRoles()])
})

async function loadUsers() {
  loading.value = true
  try {
    users.value = await listUsers()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

async function loadRoles() {
  try {
    roles.value = await listRoles()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

function openCreate() {
  editing.value = false
  form.id = 0
  form.username = ''
  form.password = ''
  form.nickname = ''
  form.roleIds = []
  form.enabled = true
  editVisible.value = true
}

function openEdit(row) {
  editing.value = true
  form.id = row.id
  form.username = row.username
  form.password = ''
  form.nickname = row.nickname
  form.roleIds = (row.roles || []).map((r) => r.id)
  form.enabled = row.status === 1
  editVisible.value = true
}

async function submit() {
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return
  submitting.value = true
  try {
    const status = form.enabled ? 1 : 0
    if (editing.value) {
      await updateUser(form.id, { nickname: form.nickname, roleIds: form.roleIds, status })
    } else {
      await createUser({
        username: form.username,
        password: form.password,
        nickname: form.nickname,
        roleIds: form.roleIds,
        status
      })
    }
    ElMessage.success('保存成功')
    editVisible.value = false
    await loadUsers()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    submitting.value = false
  }
}

async function remove(row) {
  try {
    await deleteUser(row.id)
    ElMessage.success('已删除')
    await loadUsers()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

function openPassword(row) {
  passwordForm.id = row.id
  passwordForm.password = ''
  passwordVisible.value = true
}

async function submitPassword() {
  const valid = await passwordFormRef.value.validate().catch(() => false)
  if (!valid) return
  submitting.value = true
  try {
    await resetPassword(passwordForm.id, passwordForm.password)
    ElMessage.success('密码已重置')
    passwordVisible.value = false
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.role-tag {
  margin-right: 6px;
}
</style>
