import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import App from './App'
import { MidnightProvider } from './providers/MidnightProvider'
import './index.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <BrowserRouter>
      <MidnightProvider>
        <App />
      </MidnightProvider>
    </BrowserRouter>
  </React.StrictMode>,
)
