import { parser as xmlParser } from '@lezer/xml'
import { load as parseYAML } from 'js-yaml'

/** 支持的配置类型，与后端 configcenter 内保持一致。 */
export const CONFIG_TYPES = ['text', 'json', 'yaml', 'properties', 'xml']

/** 类型展示名。 */
export const CONFIG_TYPE_LABELS = {
  text: 'Text',
  json: 'JSON',
  yaml: 'YAML',
  properties: 'Properties',
  xml: 'XML'
}

/** XML 预定义实体；其余命名实体后端一律拒绝（与 encoding/xml 行为一致）。 */
const XML_ENTITIES = new Set(['amp', 'lt', 'gt', 'quot', 'apos'])

/** lezer XML 解析器的容错节点，出现即说明标签未正确配对。 */
const XML_RECOVERY_NODES = new Set(['MissingCloseTag', 'MismatchedCloseTag'])

/**
 * 校验配置内容格式，返回 CodeMirror 诊断数组（空数组表示合法）。
 * text 类型与空内容不做校验。
 */
export function validateContent(type, content) {
  const text = content ?? ''
  if (!text.trim()) return []
  if (type === 'json') return validateJSON(text)
  if (type === 'yaml') return validateYAML(text)
  if (type === 'xml') return validateXML(text)
  if (type === 'properties') return validateProperties(text)
  return []
}

/** 格式化内容，目前仅 JSON 支持；解析失败时返回 null。 */
export function formatContent(type, content) {
  if (type !== 'json') return null
  try {
    return JSON.stringify(JSON.parse(content), null, 2)
  } catch {
    return null
  }
}

function validateJSON(text) {
  try {
    JSON.parse(text)
    return []
  } catch (e) {
    const raw = String(e?.message || 'JSON 语法错误')
    const position = /position (\d+)/.exec(raw)
    const offset = position ? Number(position[1]) : 0
    const point = posToLineColumn(text, offset)
    // V8 的错误信息已带位置描述，去掉重复的尾巴只保留原因。
    const reason = raw.replace(/\s+in JSON at position \d+[\s\S]*$/, '')
    return [makeDiagnostic(text, offset, offset + 1, point, `JSON 格式错误：${reason}`)]
  }
}

// YAML 使用 js-yaml 严格解析，与后端 yaml.v3 的行为保持一致；
// lezer 的容错解析器会放过制表符缩进、未闭合引号、重复键等问题。
function validateYAML(text) {
  try {
    parseYAML(text)
    return []
  } catch (e) {
    const mark = e?.mark
    const offset = Number.isFinite(mark?.position) ? mark.position : 0
    const point = mark && Number.isFinite(mark.line)
      ? { line: mark.line + 1, column: mark.column + 1 }
      : posToLineColumn(text, offset)
    const reason = String(e?.reason || e?.message || '语法错误').split('\n')[0]
    return [makeDiagnostic(text, offset, offset + 1, point, `YAML 格式错误：${reason}`)]
  }
}

// XML 仍基于 lezer 语法树，但额外检查它的容错节点与实体引用，
// 否则标签不匹配、未声明的实体这类错误会被漏判。
function validateXML(text) {
  const issues = []
  xmlParser.parse(text).cursor().iterate((node) => {
    const name = node.type.name
    if (node.type.isError) {
      pushIssue(issues, node.from, node.to, 'XML 格式错误：此处语法无法解析')
    } else if (XML_RECOVERY_NODES.has(name)) {
      pushIssue(issues, node.from, node.to, 'XML 格式错误：标签未正确闭合')
    } else if (name === 'EntityReference') {
      const raw = text.slice(node.from, node.to)
      if (raw.endsWith(';') && !XML_ENTITIES.has(raw.slice(1, -1))) {
        issues.push({ from: node.from, to: node.to, message: `XML 格式错误：未知的实体引用 ${raw}` })
      }
    }
  })
  return issues.map((issue) =>
    makeDiagnostic(text, issue.from, issue.to, posToLineColumn(text, issue.from), issue.message)
  )
}

/** 相邻且同类的问题合并成一条，避免刷屏。 */
function pushIssue(issues, from, to, message) {
  const last = issues[issues.length - 1]
  if (last && last.message === message && from <= last.to + 1) {
    last.to = Math.max(last.to, to)
    return
  }
  issues.push({ from, to: Math.max(to, from + 1), message })
}

function validateProperties(text) {
  const diagnostics = []
  let offset = 0
  text.split('\n').forEach((raw, index) => {
    const line = raw.trim()
    const start = offset
    offset += raw.length + 1
    if (!line || line.startsWith('#') || line.startsWith('!')) return
    if (line.startsWith('=') || line.startsWith(':')) {
      diagnostics.push({
        from: start,
        to: start + raw.length,
        line: index + 1,
        column: 1,
        message: 'Properties 格式错误：缺少键名',
        severity: 'error'
      })
    }
  })
  return diagnostics
}

function makeDiagnostic(text, from, to, point, message) {
  const start = Math.max(0, Math.min(from, text.length))
  return {
    from: start,
    to: Math.min(Math.max(to, start + 1), text.length),
    line: point.line,
    column: point.column,
    message,
    severity: 'error'
  }
}

function posToLineColumn(text, position) {
  const offset = Math.max(0, Math.min(position, text.length))
  const lines = text.slice(0, offset).split('\n')
  return { line: lines.length, column: lines[lines.length - 1].length + 1 }
}
