// Polyfills for Node.js globals required by @hashgraph/hashconnect
import { Buffer } from 'buffer'
import process from 'process'

// Make them available globally
window.Buffer = Buffer
window.process = process

import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import App from './App'
import { WalletConfigProvider } from './providers/WalletConfig'
import { WalletProvider } from './providers/WalletProvider'
import './index.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <BrowserRouter>
      <WalletConfigProvider>
        <WalletProvider>
          <App />
        </WalletProvider>
      </WalletConfigProvider>
    </BrowserRouter>
  </React.StrictMode>,
)
