import React from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import App from './App.jsx'
import { SpectatorProvider } from './state/useSpectatorStore.jsx'
import './styles.css'

createRoot(document.getElementById('root')).render(
  <BrowserRouter>
    <SpectatorProvider>
      <App />
    </SpectatorProvider>
  </BrowserRouter>
)
