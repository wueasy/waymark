<template>
  <div ref="host" class="code-editor" :style="{ height }"></div>
</template>

<script setup>
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Compartment, EditorState } from '@codemirror/state'
import { EditorView } from '@codemirror/view'
import { HighlightStyle, StreamLanguage, syntaxHighlighting } from '@codemirror/language'
import { tags } from '@lezer/highlight'
import { setDiagnostics, lintGutter } from '@codemirror/lint'
import { basicSetup } from 'codemirror'
import { json } from '@codemirror/lang-json'
import { yaml } from '@codemirror/lang-yaml'
import { xml } from '@codemirror/lang-xml'
import { properties } from '@codemirror/legacy-modes/mode/properties'
import { validateContent } from '../configFormat'
import { themeStore } from '../store'

const props = defineProps({
  modelValue: { type: String, default: '' },
  type: { type: String, default: 'text' },
  readonly: { type: Boolean, default: false },
  height: { type: String, default: '360px' }
})
const emit = defineEmits(['update:modelValue', 'errors'])

const host = ref()
const languageSlot = new Compartment()
const readonlySlot = new Compartment()
const themeSlot = new Compartment()

let view = null
let timer = null

/** 按配置类型选择语言扩展，text 类型不做高亮。 */
function languageFor(type) {
  switch (type) {
    case 'json':
      return json()
    case 'yaml':
      return yaml()
    case 'xml':
      return xml()
    case 'properties':
      return StreamLanguage.define(properties)
    default:
      return []
  }
}

function readonlyExtensions(readonly) {
  return readonly ? [EditorState.readOnly.of(true), EditorView.editable.of(false)] : []
}

/** 暗黑模式下的语法高亮，保证深色背景上的可读性。 */
const darkHighlight = HighlightStyle.define([
  { tag: tags.comment, color: '#7f848e', fontStyle: 'italic' },
  { tag: [tags.keyword, tags.modifier, tags.operatorKeyword], color: '#c678dd' },
  { tag: [tags.string, tags.special(tags.string)], color: '#98c379' },
  { tag: [tags.number, tags.bool, tags.null], color: '#d19a66' },
  { tag: [tags.propertyName, tags.attributeName, tags.variableName], color: '#e06c75' },
  { tag: [tags.typeName, tags.className, tags.tagName], color: '#e5c07b' },
  { tag: [tags.operator, tags.punctuation], color: '#abb2bf' }
])

/** 编辑器外观：使用 Element Plus 变量，颜色随 html.dark 自动切换。 */
function editorTheme(dark) {
  return EditorView.theme(
    {
      '&': {
        fontSize: '13px',
        border: '1px solid var(--el-border-color)',
        backgroundColor: 'var(--el-bg-color)',
        color: 'var(--el-text-color-primary)'
      },
      '&.cm-focused': { outline: 'none', borderColor: 'var(--el-color-primary)' },
      '.cm-scroller': { fontFamily: 'Consolas, Monaco, "Courier New", monospace' },
      '.cm-content': { caretColor: 'var(--el-text-color-primary)' },
      '.cm-gutters': {
        backgroundColor: 'var(--el-fill-color-light)',
        color: 'var(--el-text-color-secondary)',
        border: 'none'
      },
      '.cm-activeLine': { backgroundColor: 'var(--el-fill-color-lighter)' },
      '.cm-activeLineGutter': { backgroundColor: 'var(--el-fill-color)' },
      '.cm-selectionBackground, &.cm-focused .cm-selectionBackground': {
        backgroundColor: 'var(--el-color-primary-light-7)'
      }
    },
    { dark }
  )
}

/** 主题扩展：暗黑模式额外挂上深色语法高亮。 */
function themeExtensions(dark) {
  return [editorTheme(dark), dark ? syntaxHighlighting(darkHighlight) : []]
}

/** 重新计算格式诊断：同步到编辑器的同时抛给父组件，用于保存前拦截。 */
function refreshDiagnostics() {
  if (!view) return
  const errors = validateContent(props.type, view.state.doc.toString())
  view.dispatch(setDiagnostics(view.state, errors))
  emit('errors', errors)
}

// 逐字输入时不必每次都重新解析，稍作延迟。
function scheduleDiagnostics() {
  clearTimeout(timer)
  timer = setTimeout(refreshDiagnostics, 150)
}

onMounted(() => {
  view = new EditorView({
    parent: host.value,
    state: EditorState.create({
      doc: props.modelValue,
      extensions: [
        basicSetup,
        lintGutter(),
        languageSlot.of(languageFor(props.type)),
        readonlySlot.of(readonlyExtensions(props.readonly)),
        themeSlot.of(themeExtensions(themeStore.dark)),
        EditorView.lineWrapping,
        EditorView.updateListener.of((update) => {
          if (!update.docChanged) return
          emit('update:modelValue', update.state.doc.toString())
          scheduleDiagnostics()
        })
      ]
    })
  })
  refreshDiagnostics()
})

watch(
  () => props.modelValue,
  (value) => {
    if (!view || value === view.state.doc.toString()) return
    view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: value } })
    refreshDiagnostics()
  }
)

watch(
  () => props.type,
  (type) => {
    if (!view) return
    view.dispatch({ effects: languageSlot.reconfigure(languageFor(type)) })
    refreshDiagnostics()
  }
)

watch(
  () => props.readonly,
  (readonly) => {
    if (view) view.dispatch({ effects: readonlySlot.reconfigure(readonlyExtensions(readonly)) })
  }
)

watch(
  () => themeStore.dark,
  (dark) => {
    if (view) view.dispatch({ effects: themeSlot.reconfigure(themeExtensions(dark)) })
  }
)

onBeforeUnmount(() => {
  clearTimeout(timer)
  view?.destroy()
  view = null
})
</script>

<style scoped>
.code-editor {
  width: 100%;
  min-width: 0;
  /* 固定高度内滚动，内容再多也不撑破容器 */
  overflow: hidden;
}
.code-editor :deep(.cm-editor) {
  height: 100%;
  border-radius: 4px;
}
.code-editor :deep(.cm-scroller) {
  overflow: auto;
}
</style>
