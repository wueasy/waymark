import { reactive } from 'vue'
import { getUser, listNamespaces, profile, setUser } from './api'

const NS_KEY = 'waymark_namespace'
const THEME_KEY = 'waymark_theme'

/** 主题状态：dark 为 true 时启用暗黑模式。 */
export const themeStore = reactive({
  dark: false
})

/** 把主题应用到根元素，Element Plus 暗黑变量依赖 html.dark。 */
function applyTheme(dark) {
  document.documentElement.classList.toggle('dark', dark)
}

/** 启动时应用主题：优先使用本地保存值，未保存时跟随系统偏好。 */
export function initTheme() {
  const saved = localStorage.getItem(THEME_KEY)
  themeStore.dark = saved ? saved === 'dark' : window.matchMedia('(prefers-color-scheme: dark)').matches
  applyTheme(themeStore.dark)
}

/** 设置主题并持久化。 */
export function setDark(dark) {
  themeStore.dark = dark
  localStorage.setItem(THEME_KEY, dark ? 'dark' : 'light')
  applyTheme(dark)
}

/** 在浅色与暗黑模式之间切换。 */
export function toggleDark() {
  setDark(!themeStore.dark)
}

/** 当前登录用户：登录后从后端拉取，包含角色与命名空间权限，供全局按需判断。 */
export const userStore = reactive({
  profile: getUser()
})

/** 写入当前用户并同步本地缓存。 */
export function setProfile(p) {
  userStore.profile = p
  setUser(p)
}

/** 拉取最新的用户信息（角色与权限可能已被管理员调整）。 */
export async function loadProfile() {
  const p = await profile()
  setProfile(p)
  return p
}

/** 全局命名空间状态：顶部切换器与各业务页面共享同一份数据。 */
export const namespaceStore = reactive({
  current: localStorage.getItem(NS_KEY) || 'public',
  list: []
})

/** 切换当前命名空间并持久化。 */
export function setNamespace(namespace) {
  namespaceStore.current = namespace
  localStorage.setItem(NS_KEY, namespace)
}

/** 拉取当前用户可见的命名空间列表，并在当前选中项失效时回退到第一项。 */
export async function loadNamespaces() {
  const list = await listNamespaces()
  namespaceStore.list = list
  if (!list.length) {
    // 当前用户没有任何命名空间授权：清空选中项，页面据此展示空态而非发起越权请求。
    namespaceStore.current = ''
    localStorage.removeItem(NS_KEY)
    return list
  }
  if (!list.some((n) => n.namespace === namespaceStore.current)) {
    setNamespace(list[0].namespace)
  }
  return list
}
