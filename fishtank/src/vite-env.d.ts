/// <reference types="bun-types" />

interface ImportMetaEnv {
  readonly BUN_PUBLIC_API_BASE_URL?: string
  readonly PROD: boolean
  readonly DEV: boolean
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}

declare module '*.png' {
  const src: string
  export default src
}
