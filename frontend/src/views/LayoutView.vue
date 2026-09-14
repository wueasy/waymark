<template>
  <el-container class="layout">
    <el-header class="header">
      <div class="brand">Waymark</div>
      <div class="brand-sub">注册中心 &amp; 配置中心</div>
      <div class="header-right">
        <ThemeToggle />
        <el-tag size="small" :type="modeTagType">{{ modeLabel }}</el-tag>
        <span class="user">{{ nickname }}（{{ roleText }}）</span>
        <el-button size="small" @click="openChangePassword">修改密码</el-button>
        <el-button size="small" @click="logout">退出登录</el-button>
      </div>
    </el-header>

    <el-container class="body">
      <el-aside width="200px" class="aside">
        <el-menu :default-active="activeMenu" class="menu" router>
          <el-menu-item index="/services">
            <el-icon><Connection /></el-icon>
            <span>服务注册</span>
          </el-menu-item>
          <el-menu-item index="/configs">
            <el-icon><Document /></el-icon>
            <span>配置管理</span>
          </el-menu-item>
          <el-menu-item index="/subscribers">
            <el-icon><Bell /></el-icon>
            <span>订阅列表</span>
          </el-menu-item>
          <el-menu-item index="/namespaces">
            <el-icon><Collection /></el-icon>
            <span>命名空间</span>
          </el-menu-item>
          <el-menu-item v-if="isAdmin" index="/users">
            <el-icon><User /></el-icon>
            <span>用户管理</span>
          </el-menu-item>
          <el-menu-item v-if="isAdmin" index="/roles">
            <el-icon><Avatar /></el-icon>
            <span>角色管理</span>
          </el-menu-item>
          <el-menu-item v-if="isAdmin" index="/cluster">
            <el-icon><Grid /></el-icon>
            <span>集群节点</span>
          </el-menu-item>
        </el-menu>
        <div class="version" :title="`构建时间：${buildTime}`">v{{ appVersion }}</div>
      </el-aside>

      <el-main class="main">
        <router-view />
      </el-main>
    </el-container>

    <el-dialog v-model="passwordVisible" title="修改密码" width="420px">
      <el-form ref="passwordFormRef" :model="passwordForm" :rules="passwordRules" label-width="90px">
        <el-form-item label="原密码" prop="oldPassword">
          <el-input v-model="passwordForm.oldPassword" type="password" show-password placeholder="请输入原密码" />
        </el-form-item>
        <el-form-item label="新密码" prop="newPassword">
          <el-input v-model="passwordForm.newPassword" type="password" show-password placeholder="请输入新密码" />
        </el-form-item>
        <el-form-item label="确认密码" prop="confirmPassword">
          <el-input v-model="passwordForm.confirmPassword" type="password" show-password placeholder="请再次输入新密码" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="passwordVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitPassword">确定</el-button>
      </template>
    </el-dialog>
  </el-container>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Avatar, Bell, Collection, Connection, Document, Grid, User } from '@element-plus/icons-vue'
import { changePassword, clearAuth, clusterLeader } from '../api'
import { loadProfile, userStore } from '../store'
import ThemeToggle from '../components/ThemeToggle.vue'

const route = useRoute()
const router = useRouter()
const currentUser = computed(() => userStore.profile)
const isAdmin = computed(() => !!currentUser.value?.isAdmin)
const nickname = computed(() => currentUser.value?.nickname || currentUser.value?.username || '-')
const roleText = computed(() => {
  const roles = currentUser.value?.roles || []
  return roles.length ? roles.map((r) => r.name).join('、') : '无角色'
})
const activeMenu = computed(() => route.path)

// 构建时注入的版本号与构建时间（见 vite.config.js 的 define）
const appVersion = __APP_VERSION__
const buildTime = __BUILD_TIME__

const mode = ref('')
const modeLabel = computed(() => (mode.value === 'cluster' ? '集群模式' : '单机模式'))
const modeTagType = computed(() => (mode.value === 'cluster' ? 'success' : 'info'))

// 修改密码弹窗
const passwordVisible = ref(false)
const submitting = ref(false)
const passwordFormRef = ref()
const passwordForm = reactive({ oldPassword: '', newPassword: '', confirmPassword: '' })
const passwordRules = {
  oldPassword: [{ required: true, message: '请输入原密码', trigger: 'blur' }],
  newPassword: [{ required: true, message: '请输入新密码', trigger: 'blur' }],
  confirmPassword: [
    { required: true, message: '请再次输入新密码', trigger: 'blur' },
    {
      validator: (rule, value, callback) => {
        if (value !== passwordForm.newPassword) {
          callback(new Error('两次输入的新密码不一致'))
          return
        }
        callback()
      },
      trigger: 'blur'
    }
  ]
}

function openChangePassword() {
  passwordForm.oldPassword = ''
  passwordForm.newPassword = ''
  passwordForm.confirmPassword = ''
  passwordVisible.value = true
}

async function submitPassword() {
  const valid = await passwordFormRef.value.validate().catch(() => false)
  if (!valid) return
  submitting.value = true
  try {
    await changePassword({ oldPassword: passwordForm.oldPassword, newPassword: passwordForm.newPassword })
    ElMessage.success('密码修改成功')
    passwordVisible.value = false
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    submitting.value = false
  }
}

onMounted(async () => {
  try {
    await loadProfile()
  } catch {
    // 个人信息刷新失败不阻塞页面，沿用本地缓存的登录信息
  }
  try {
    const data = await clusterLeader()
    mode.value = data.mode
  } catch {
    // 忽略模式探测失败，页面其它功能不受影响
  }
})

function logout() {
  clearAuth()
  ElMessage.success('已退出登录')
  router.push('/login')
}
</script>

<style scoped>
.layout {
  height: 100vh;
}
.header {
  display: flex;
  align-items: center;
  gap: 12px;
  background: var(--el-bg-color);
  border-bottom: 1px solid var(--el-border-color-light);
  height: 56px;
  box-sizing: border-box;
}
.brand {
  font-size: 18px;
  font-weight: 600;
}
.brand-sub {
  color: var(--el-text-color-secondary);
  font-size: 13px;
}
.header-right {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 12px;
}
.user {
  color: var(--el-text-color-regular);
  font-size: 13px;
}
.body {
  height: calc(100vh - 56px);
}
.aside {
  background: var(--el-bg-color);
  border-right: 1px solid var(--el-border-color-light);
  display: flex;
  flex-direction: column;
}
.menu {
  border-right: none;
  flex: 1;
  overflow-y: auto;
}
.version {
  flex-shrink: 0;
  padding: 10px 16px;
  text-align: center;
  color: var(--el-text-color-secondary);
  font-size: 12px;
  border-top: 1px solid var(--el-border-color-lighter);
}
.main {
  background: var(--el-bg-color-page);
  overflow-y: auto;
}
</style>
