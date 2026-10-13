import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import './index.css'
import App from './App.tsx'
import { resolveConfig } from './lib/config.ts'

// Resolved before the first render so every page sees settled config values rather than ones that
// could change underneath it - see lib/config.ts.
resolveConfig().then((config) => {
  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <BrowserRouter>
        <App config={config} />
      </BrowserRouter>
    </StrictMode>,
  )
})
