import { createContext, useCallback, useContext, useEffect, useState } from 'react'
import type { ReactNode } from 'react'
import { adminAuthApi } from '@entities/admin-auth'

interface AdminSessionState {
  login: string | null
  loading: boolean
  signIn: (login: string, password: string) => Promise<void>
  signOut: () => Promise<void>
}

const AdminSessionContext = createContext<AdminSessionState | null>(null)

export function AdminSessionProvider({ children }: { children: ReactNode }) {
  const [login, setLogin] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    adminAuthApi
      .me()
      .then((me) => { if (!cancelled) setLogin(me.login) })
      .catch(() => { if (!cancelled) setLogin(null) })
      .finally(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [])

  const signIn = useCallback(async (loginValue: string, password: string) => {
    const me = await adminAuthApi.login(loginValue, password)
    setLogin(me.login)
  }, [])

  const signOut = useCallback(async () => {
    try {
      await adminAuthApi.logout()
    } finally {
      setLogin(null)
    }
  }, [])

  return (
    <AdminSessionContext.Provider value={{ login, loading, signIn, signOut }}>
      {children}
    </AdminSessionContext.Provider>
  )
}

export function useAdminSession() {
  const ctx = useContext(AdminSessionContext)
  if (!ctx) throw new Error('useAdminSession должен использоваться внутри AdminSessionProvider')
  return ctx
}
