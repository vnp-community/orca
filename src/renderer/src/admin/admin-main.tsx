import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { AdminApp } from '../components/admin/AdminApp'

// Guard: redirect to login if no session
// (Backend handles this via 401 redirect — frontend just mounts)
createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <AdminApp />
  </StrictMode>
)
