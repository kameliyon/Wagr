/// <reference types="vite/client" />

// Polyfills for Node.js globals
import type { Buffer } from 'buffer'

declare global {
  interface Window {
    Buffer: typeof Buffer
    process: NodeJS.Process
  }

  var Buffer: typeof Buffer
  var process: NodeJS.Process
}
