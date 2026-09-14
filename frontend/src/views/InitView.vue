<template>
  <div class="auth-page">
    <div class="theme-corner">
      <ThemeToggle />
    </div>
    <el-card class="auth-card">
      <template #header>
        <div class="auth-title">初始化 Waymark</div>
      </template>
      <p class="auth-tip">系统尚未初始化，请创建第一个管理员账号。</p>
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-form-item label="用户名" prop="username">
          <el-input v-model="form.username" placeholder="请输入管理员用户名" />
        </el-form-item>
        <el-form-item label="密码" prop="password">
          <el-input v-model="form.password" type="password" show-password placeholder="请输入密码" />
        </el-form-item>
        <el-form-item label="确认密码" prop="confirm">
          <el-input v-model="form.confirm" type="password" show-password placeholder="请再次输入密码" />
        </el-form-item>
        <el-form-item label="昵称">
          <el-input v-model="form.nickname" placeholder="选填" />
        </el-form-item>
        <el-button type="primary" class="auth-submit" :loading="loading" @click="submit">创建管理员并进入登录</el-button>
      </el-form>
    </el-card>
  </div>
</template>

<script setup>
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { initAdmin, markInitialized } from '../api'
import ThemeToggle from '../components/ThemeToggle.vue'

const router = useRouter()
const formRef = ref()
const loading = ref(false)
const form = reactive({ username: '', password: '', confirm: '', nickname: '' })

const rules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }],
  confirm: [
    { required: true, message: '请再次输入密码', trigger: 'blur' },
    {
      validator: (_rule, value, callback) => {
        if (value !== form.password) {
          callback(new Error('两次输入的密码不一致'))
          return
        }
        callback()
      },
      trigger: 'blur'
    }
  ]
}

async function submit() {
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return
  loading.value = true
  try {
    await initAdmin({ username: form.username, password: form.password, nickname: form.nickname })
    markInitialized()
    ElMessage.success('初始化完成，请登录')
    router.push('/login')
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.auth-page {
  position: relative;
  height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--el-bg-color-page);
}
/* 右上角主题切换按钮 */
.theme-corner {
  position: absolute;
  top: 16px;
  right: 16px;
  z-index: 10;
}
.auth-card {
  width: 420px;
}
.auth-title {
  font-size: 18px;
  font-weight: 600;
}
.auth-tip {
  color: var(--el-text-color-secondary);
  font-size: 13px;
  margin: 0 0 16px;
}
.auth-submit {
  width: 100%;
  margin-top: 4px;
}
</style>
