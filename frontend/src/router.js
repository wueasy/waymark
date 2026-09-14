import { createRouter, createWebHistory } from 'vue-router'
import { checkInit, getToken } from './api'
import { loadProfile } from './store'

const routes = [
  { path: '/', redirect: '/services' },
  { path: '/init', component: () => import('./views/InitView.vue'), meta: { public: true } },
  { path: '/login', component: () => import('./views/LoginView.vue'), meta: { public: true } },
  {
    path: '/',
    component: () => import('./views/LayoutView.vue'),
    children: [
      { path: 'services', component: () => import('./views/ServiceView.vue') },
      { path: 'subscribers', component: () => import('./views/SubscriberView.vue') },
      { path: 'configs', component: () => import('./views/ConfigView.vue') },
      { path: 'namespaces', component: () => import('./views/NamespaceView.vue') },
      { path: 'users', component: () => import('./views/UserView.vue'), meta: { admin: true } },
      { path: 'roles', component: () => import('./views/RoleView.vue'), meta: { admin: true } },
      { path: 'cluster', component: () => import('./views/ClusterView.vue'), meta: { admin: true } }
    ]
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach(async (to) => {
  let initialized = true
  try {
    initialized = await checkInit()
  } catch {
    // 初始化状态查询失败时不阻塞跳转，交由页面自行提示
  }

  if (!initialized) {
    return to.path === '/init' ? true : '/init'
  }
  if (to.path === '/init') {
    return getToken() ? '/services' : '/login'
  }
  if (to.meta.public) {
    return true
  }
  if (!getToken()) {
    return '/login'
  }
  if (to.meta.admin) {
    // 本地缓存可能滞后于管理员的最新调整，进入管理员页面前重新拉取个人信息再判定。
    let isAdmin = false
    try {
      isAdmin = !!(await loadProfile())?.isAdmin
    } catch {
      // 拉取失败（如网络异常）时按非管理员处理，避免误放行。
      isAdmin = false
    }
    if (!isAdmin) {
      return '/services'
    }
  }
  return true
})

export default router
