<template>
  <div class="auth-page">
    <div class="theme-corner">
      <ThemeToggle />
    </div>
    <aside class="auth-brand">
      <span class="brand-glow brand-glow-a"></span>
      <span class="brand-glow brand-glow-b"></span>
      <div class="brand-inner">
        <div class="brand-mark">
          <svg viewBox="0 0 48 48" aria-hidden="true">
            <circle cx="24" cy="24" r="19" fill="none" stroke="currentColor" stroke-width="2.4" opacity="0.5" />
            <path d="M33 15 27 27 15 33 21 21Z" fill="currentColor" />
          </svg>
        </div>
        <h1 class="brand-name">Waymark</h1>
        <p class="brand-desc">注册中心 &amp; 配置中心</p>
        <ul class="brand-points">
          <li>服务注册与发现</li>
          <li>配置集中管理与热更新</li>
          <li>命名空间与权限隔离</li>
        </ul>
      </div>
    </aside>

    <main class="auth-panel">
      <div class="auth-box">
        <div class="auth-head">
          <div class="auth-mark">
            <svg viewBox="0 0 48 48" aria-hidden="true">
              <circle cx="24" cy="24" r="19" fill="none" stroke="currentColor" stroke-width="2.4" opacity="0.5" />
              <path d="M33 15 27 27 15 33 21 21Z" fill="currentColor" />
            </svg>
          </div>
          <h2 class="auth-title">欢迎回来</h2>
          <p class="auth-sub">请登录以继续使用控制台</p>
        </div>

        <el-form ref="formRef" :model="form" :rules="rules" class="auth-form" @keyup.enter="submit">
          <el-form-item prop="username">
            <el-input v-model="form.username" size="large" class="auth-input" placeholder="请输入用户名">
              <template #prefix>
                <svg class="input-icon" viewBox="0 0 24 24" aria-hidden="true">
                  <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2" />
                  <circle cx="12" cy="7" r="4" />
                </svg>
              </template>
            </el-input>
          </el-form-item>
          <el-form-item prop="password">
            <el-input
              v-model="form.password"
              type="password"
              size="large"
              class="auth-input"
              show-password
              placeholder="请输入密码"
            >
              <template #prefix>
                <svg class="input-icon" viewBox="0 0 24 24" aria-hidden="true">
                  <rect x="3" y="11" width="18" height="11" rx="2" />
                  <path d="M7 11V7a5 5 0 0 1 10 0v4" />
                </svg>
              </template>
            </el-input>
          </el-form-item>
          <el-button type="primary" class="auth-submit" :loading="loading" @click="submit">登 录</el-button>
        </el-form>
      </div>
    </main>
  </div>
</template>

<script setup>
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { login, setToken, setUser } from '../api'
import ThemeToggle from '../components/ThemeToggle.vue'

const router = useRouter()
const formRef = ref()
const loading = ref(false)
const form = reactive({ username: '', password: '' })

const rules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }]
}

async function submit() {
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return
  loading.value = true
  try {
    const data = await login({ username: form.username, password: form.password })
    setToken(data.token)
    setUser(data.user)
    ElMessage.success('登录成功')
    router.push('/services')
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
  display: flex;
  min-height: 100vh;
  background: var(--el-bg-color);
}

/* 右上角主题切换按钮 */
.theme-corner {
  position: absolute;
  top: 16px;
  right: 16px;
  z-index: 10;
}

/* ---------- 左侧品牌区 ---------- */
.auth-brand {
  position: relative;
  flex: 0 0 46%;
  max-width: 640px;
  display: flex;
  align-items: center;
  overflow: hidden;
  padding: 48px 56px;
  color: #fff;
  background: linear-gradient(140deg, #1e3a8a 0%, #312e81 48%, #4c1d95 100%);
}
.auth-brand::before {
  content: '';
  position: absolute;
  inset: 0;
  background-image: linear-gradient(rgba(255, 255, 255, 0.06) 1px, transparent 1px),
    linear-gradient(90deg, rgba(255, 255, 255, 0.06) 1px, transparent 1px);
  background-size: 34px 34px;
  mask-image: radial-gradient(circle at 30% 40%, #000 0%, transparent 78%);
}
.brand-glow {
  position: absolute;
  border-radius: 50%;
  filter: blur(70px);
  opacity: 0.55;
}
.brand-glow-a {
  width: 340px;
  height: 340px;
  top: -110px;
  right: -90px;
  background: #6366f1;
}
.brand-glow-b {
  width: 300px;
  height: 300px;
  bottom: -120px;
  left: -80px;
  background: #06b6d4;
  opacity: 0.35;
}
.brand-inner {
  position: relative;
  z-index: 1;
}
.brand-mark {
  width: 56px;
  height: 56px;
  color: #fff;
  margin-bottom: 22px;
}
.brand-mark svg {
  width: 100%;
  height: 100%;
  display: block;
}
.brand-name {
  margin: 0;
  font-size: 34px;
  font-weight: 700;
  letter-spacing: 1px;
}
.brand-desc {
  margin: 10px 0 30px;
  font-size: 15px;
  color: rgba(255, 255, 255, 0.72);
}
.brand-points {
  margin: 0;
  padding: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.brand-points li {
  position: relative;
  padding-left: 22px;
  font-size: 14px;
  color: rgba(255, 255, 255, 0.85);
}
.brand-points li::before {
  content: '';
  position: absolute;
  left: 0;
  top: 6px;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #7dd3fc;
  box-shadow: 0 0 0 4px rgba(125, 211, 252, 0.18);
}

/* ---------- 右侧表单区 ---------- */
.auth-panel {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px 24px;
  background: radial-gradient(circle at 80% 10%, var(--el-fill-color-light) 0%, var(--el-bg-color) 55%);
}
.auth-box {
  width: 100%;
  max-width: 360px;
}
.auth-head {
  margin-bottom: 28px;
}
.auth-mark {
  display: none;
  width: 44px;
  height: 44px;
  color: var(--el-color-primary);
  margin-bottom: 16px;
}
.auth-mark svg {
  width: 100%;
  height: 100%;
  display: block;
}
.auth-title {
  margin: 0;
  font-size: 26px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}
.auth-sub {
  margin: 8px 0 0;
  font-size: 14px;
  color: var(--el-text-color-secondary);
}

.auth-form :deep(.el-form-item) {
  margin-bottom: 20px;
}
.auth-input :deep(.el-input__wrapper) {
  padding: 4px 14px;
  border-radius: 10px;
  background: var(--el-fill-color-light);
  box-shadow: 0 0 0 1px var(--el-border-color-light) inset;
  transition: box-shadow 0.2s, background 0.2s;
}
.auth-input :deep(.el-input__wrapper:hover) {
  box-shadow: 0 0 0 1px var(--el-color-primary-light-5) inset;
}
.auth-input :deep(.el-input__wrapper.is-focus) {
  background: var(--el-bg-color);
  box-shadow: 0 0 0 1px var(--el-color-primary) inset, 0 0 0 4px var(--el-color-primary-light-9);
}
.auth-input :deep(.el-input__prefix) {
  color: var(--el-text-color-placeholder);
}
.input-icon {
  width: 18px;
  height: 18px;
  display: block;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.8;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.auth-submit {
  width: 100%;
  height: 44px;
  margin-top: 4px;
  border: none;
  border-radius: 10px;
  font-size: 15px;
  font-weight: 500;
  letter-spacing: 2px;
  color: #fff;
  background: linear-gradient(120deg, #4f46e5 0%, #6366f1 55%, #3b82f6 100%);
  box-shadow: 0 10px 22px rgba(79, 70, 229, 0.25);
  transition: transform 0.2s, box-shadow 0.2s, opacity 0.2s;
}
.auth-submit:not(.is-disabled):hover {
  color: #fff;
  background: linear-gradient(120deg, #4338ca 0%, #4f46e5 55%, #2563eb 100%);
  box-shadow: 0 12px 26px rgba(79, 70, 229, 0.34);
  transform: translateY(-1px);
}
.auth-submit:not(.is-disabled):active {
  transform: translateY(0);
  box-shadow: 0 6px 14px rgba(79, 70, 229, 0.28);
}

/* ---------- 窄屏适配 ---------- */
@media (max-width: 900px) {
  .auth-brand {
    display: none;
  }
  .auth-mark {
    display: block;
  }
  .auth-panel {
    padding: 32px 20px;
  }
}
</style>
