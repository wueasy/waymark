/** 时间戳（毫秒）格式化。 */
export function formatTime(ts) {
  if (!ts) return '-'
  const d = new Date(Number(ts))
  const pad = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

/** 相对时间描述，用于展示心跳/变更距今多久。 */
export function fromNow(ts) {
  if (!ts) return '-'
  const diff = Date.now() - Number(ts)
  if (diff < 0) return '刚刚'
  const sec = Math.floor(diff / 1000)
  if (sec < 60) return `${sec} 秒前`
  const min = Math.floor(sec / 60)
  if (min < 60) return `${min} 分钟前`
  const hour = Math.floor(min / 60)
  if (hour < 24) return `${hour} 小时前`
  return `${Math.floor(hour / 24)} 天前`
}

/** 角色权限等级中文名。 */
export function permissionLabel(permission) {
  return { read: '只读', write: '读写' }[permission] || permission
}
