import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import './index.css'
import App from './App'
import { appConfig } from './config'

// The Go server serves the UI under /app (consts.URLPathUI), optionally
// nested below a configured base path.
const basename = `${appConfig.baseURL}/app`

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <BrowserRouter basename={basename}>
      <App />
    </BrowserRouter>
  </StrictMode>,
)
