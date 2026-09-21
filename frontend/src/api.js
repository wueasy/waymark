const TOKEN_KEY = 'waymark_token'
const USER_KEY = 'waymark_user'

export function getToken() {
  return localStorage.getItem(TOKEN_KEY) || ''
}

export function setToken(token) {
  localStorage.setItem(TOKEN_KEY, token)
}

export function getUser() {
  try {
    return JSON.parse(localStorage.getItem(USER_KEY) || 'null')
  } catch {
    return null
  }
}

export function setUser(user) {
  localStorage.setItem(USER_KEY, JSON.stringify(user || null))
}

export function clearAuth() {
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(USER_KEY)
}

let initializedCache = null

/** 查询系统是否已初始化（结果进程内缓存）。 */
export async function checkInit(force = false) {
  if (initializedCache !== null && !force) {
    return initializedCache
  }
  const data = await request('/api/auth/init-status', { skipAuthRedirect: true })
  initializedCache = !!data.initialized
  return initializedCache
}

export function markInitialized() {
  initializedCache = true
}

async function request(path, options = {}) {
  const { skipAuthRedirect = false, ...rest } = options
  const headers = { 'Content-Type': 'application/json', ...(rest.headers || {}) }
  const token = getToken()
  if (token) {
    headers.Authorization = `Bearer ${token}`
  }

  const res = await fetch(path, { ...rest, headers })

  if (res.status === 401 && !skipAuthRedirect) {
    clearAuth()
    if (!window.location.pathname.startsWith('/login')) {
      window.location.href = '/login'
    }
    throw new Error('登录已失效，请重新登录')
  }

  let body = null
  try {
    body = await res.json()
  } catch {
    throw new Error(`请求失败：HTTP ${res.status}`)
  }

  if (!body || body.successful !== true) {
    const err = new Error((body && body.msg) || `请求失败：HTTP ${res.status}`)
    err.code = body && body.code
    throw err
  }
  return body.data
}

/** 统一请求入口，成功时返回 data，失败时抛出含有 msg/code 的错误。 */
export function api(path, options = {}) {
  return request(path, options)
}

/** 带认证的 fetch，401 时统一跳转登录页。 */
async function authFetch(path, options = {}) {
  const headers = { ...(options.headers || {}) }
  const token = getToken()
  if (token) {
    headers.Authorization = `Bearer ${token}`
  }
  const res = await fetch(path, { ...options, headers })
  if (res.status === 401) {
    clearAuth()
    if (!window.location.pathname.startsWith('/login')) {
      window.location.href = '/login'
    }
    throw new Error('登录已失效，请重新登录')
  }
  return res
}

/** 读取失败响应中的错误信息。 */
async function readErrorMessage(res, fallback) {
  try {
    const body = await res.json()
    if (body && body.msg) return body.msg
  } catch {
    // 非 JSON 响应，回退到通用提示
  }
  return `${fallback}：HTTP ${res.status}`
}

/** 从 Content-Disposition 解析下载文件名。 */
function parseAttachmentName(disposition) {
  if (!disposition) return ''
  const utf8 = /filename\*=UTF-8''([^;]+)/i.exec(disposition)
  if (utf8) {
    try {
      return decodeURIComponent(utf8[1])
    } catch {
      return utf8[1]
    }
  }
  const plain = /filename="?([^";]+)"?/i.exec(disposition)
  return plain ? plain[1] : ''
}

/** 触发浏览器下载 Blob。 */
function downloadBlob(blob, filename) {
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

function withQuery(path, params = {}) {
  const query = new URLSearchParams()
  Object.entries(params).forEach(([k, v]) => {
    if (v !== undefined && v !== null && v !== '') {
      query.append(k, v)
    }
  })
  const qs = query.toString()
  return qs ? `${path}?${qs}` : path
}

// ---------- 认证 ----------

export function initAdmin(payload) {
  return api('/api/auth/init', { method: 'POST', body: JSON.stringify(payload) })
}

export function login(payload) {
  return api('/api/auth/login', { method: 'POST', body: JSON.stringify(payload) })
}

export function profile() {
  return api('/api/auth/profile')
}

/** 修改当前登录用户的密码，需提供原密码。 */
export function changePassword(payload) {
  return api('/api/auth/password', { method: 'PUT', body: JSON.stringify(payload) })
}

// ---------- 命名空间 ----------

export function listNamespaces() {
  return api('/api/namespaces')
}

export function createNamespace(payload) {
  return api('/api/namespaces', { method: 'POST', body: JSON.stringify(payload) })
}

export function updateNamespace(name, payload) {
  return api(`/api/namespaces/${encodeURIComponent(name)}`, { method: 'PUT', body: JSON.stringify(payload) })
}

export function deleteNamespace(name) {
  return api(`/api/namespaces/${encodeURIComponent(name)}`, { method: 'DELETE' })
}

// ---------- 用户 ----------

export function listUsers() {
  return api('/api/users')
}

export function createUser(payload) {
  return api('/api/users', { method: 'POST', body: JSON.stringify(payload) })
}

export function updateUser(id, payload) {
  return api(`/api/users/${id}`, { method: 'PUT', body: JSON.stringify(payload) })
}

export function deleteUser(id) {
  return api(`/api/users/${id}`, { method: 'DELETE' })
}

export function resetPassword(id, password) {
  return api(`/api/users/${id}/password`, { method: 'PUT', body: JSON.stringify({ password }) })
}

// ---------- 角色 ----------

export function listRoles() {
  return api('/api/roles')
}

export function createRole(payload) {
  return api('/api/roles', { method: 'POST', body: JSON.stringify(payload) })
}

export function updateRole(id, payload) {
  return api(`/api/roles/${id}`, { method: 'PUT', body: JSON.stringify(payload) })
}

export function deleteRole(id) {
  return api(`/api/roles/${id}`, { method: 'DELETE' })
}

// ---------- 注册中心 ----------

export function listServices(params) {
  return api(withQuery('/api/registry/services', params))
}

export function listInstances(params) {
  return api(withQuery('/api/registry/instances', params))
}

export function updateInstance(payload) {
  return api('/api/registry/instance', { method: 'PUT', body: JSON.stringify(payload) })
}

export function beatInstance(params) {
  return api(withQuery('/api/registry/beat', params), { method: 'PUT' })
}

// ---------- 订阅端 ----------

export function listSubscribers(params) {
  return api(withQuery('/api/subscribe/subscribers', params))
}

// ---------- 配置中心 ----------

export function listConfigs(params) {
  return api(withQuery('/api/configs', params))
}

export function getConfig(params) {
  return api(withQuery('/api/configs/detail', params))
}

export function publishConfig(payload) {
  return api('/api/configs', { method: 'POST', body: JSON.stringify(payload) })
}

/** 保存配置草稿，草稿发布前不会同步到下游。 */
export function saveConfigDraft(payload) {
  return api('/api/configs/draft', { method: 'PUT', body: JSON.stringify(payload) })
}

/** 查询配置草稿，草稿不存在时返回 null。 */
export async function getConfigDraft(params) {
  try {
    return await api(withQuery('/api/configs/draft', params))
  } catch (e) {
    if (e.code === 1001 && /不存在/.test(e.message)) {
      return null
    }
    throw e
  }
}

export function discardConfigDraft(params) {
  return api(withQuery('/api/configs/draft', params), { method: 'DELETE' })
}

/** 预览草稿相对已发布版本的变更内容。 */
export function previewConfigDraft(params) {
  return api(withQuery('/api/configs/draft/diff', params))
}

/** 发布草稿，force 为 true 时强制覆盖已被他人更新的配置。 */
export function publishConfigDraft(payload) {
  return api('/api/configs/publish', { method: 'POST', body: JSON.stringify(payload) })
}

export function deleteConfig(params) {
  return api(withQuery('/api/configs', params), { method: 'DELETE' })
}

export function configHistory(params) {
  return api(withQuery('/api/configs/history', params))
}

export function restoreConfig(payload) {
  return api('/api/configs/restore', { method: 'POST', body: JSON.stringify(payload) })
}

/** 导出配置为 zip 压缩包并触发浏览器下载。 */
export async function exportConfigs(payload) {
  const res = await authFetch('/api/configs/export', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload)
  })
  if (!res.ok) {
    throw new Error(await readErrorMessage(res, '导出失败'))
  }
  const blob = await res.blob()
  const filename = parseAttachmentName(res.headers.get('Content-Disposition')) || 'configs.zip'
  downloadBlob(blob, filename)
}

/** 上传 zip 压缩包导入配置，可指定目标分组。 */
export async function importConfigs(formData) {
  const res = await authFetch('/api/configs/import', { method: 'POST', body: formData })
  let body = null
  try {
    body = await res.json()
  } catch {
    throw new Error(`请求失败：HTTP ${res.status}`)
  }
  if (!body || body.successful !== true) {
    throw new Error((body && body.msg) || `请求失败：HTTP ${res.status}`)
  }
  return body.data
}

// ---------- 集群 ----------

export function clusterNodes() {
  return api('/api/cluster/nodes')
}

export function clusterLeader() {
  return api('/api/cluster/leader')
}

/** 构建 SSE 订阅地址（EventSource 无法自定义请求头，需通过 token 查询参数认证）。 */
export function subscribeUrl(path, params = {}) {
  return withQuery(path, { ...params, token: getToken() })
}
