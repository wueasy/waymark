// 旧浏览器运行时 API polyfill。
// core-js 只覆盖 ES 语言特性，依赖用到的 DOM API 需在此补齐。
// 仅在浏览器缺失原生实现时生效，不会覆盖现代浏览器的原生实现。
import { ResizeObserver as ResizeObserverPolyfill } from '@juggle/resize-observer'
import 'intersection-observer'

// ResizeObserver：Chrome 64+ / Safari 13.1+ 才支持，
// Element Plus、CodeMirror、VueUse 均大量依赖它
if (typeof window !== 'undefined' && !window.ResizeObserver) {
  window.ResizeObserver = ResizeObserverPolyfill
}
