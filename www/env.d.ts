/// <reference types="vite/client" />

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<{}, {}, any>
  export default component
}

// WASM 模块声明
declare module '*.wasm' {
  const wasmUrl: string
  export default wasmUrl
}
