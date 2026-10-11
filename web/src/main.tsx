import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'

// Apply the stored theme before the first paint so a dark-mode user never sees
// a light flash. App.tsx re-applies it on mount and on every toggle.
try {
  const stored = window.localStorage.getItem('serverwatch_theme')
  const dark =
    stored === 'dark' ||
    (stored !== 'light' &&
      window.matchMedia('(prefers-color-scheme: dark)').matches)
  document.documentElement.setAttribute('data-theme', dark ? 'dark' : 'light')
} catch {
  // Ignore: the default light theme still renders correctly.
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
