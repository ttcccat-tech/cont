import { createContext, useContext, useState, useEffect, ReactNode } from 'react'
import { getMe } from '../api/kong'

export type Role = 'admin' | 'editor' | 'viewer'

export interface AuthUser {
  id: string
  username: string
  role: Role
  permissions: Record<string, { mode: string; level: number }>
}

interface AuthContextValue {
  user: AuthUser | null
  loading: boolean
  canWrite: (entity: string) => boolean
  canDelete: (entity: string) => boolean
  refetch: () => void
}

const AuthContext = createContext<AuthContextValue>({
  user: null,
  loading: true,
  canWrite: () => false,
  canDelete: () => false,
  refetch: () => {},
})

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null)
  const [loading, setLoading] = useState(true)

  const fetchMe = () => {
    setLoading(true)
    getMe()
      .then((data: any) => {
        setUser({
          id: data.id || '',
          username: data.username || '',
          role: (data.role as Role) || 'viewer',
          permissions: data.permissions || {},
        })
      })
      .catch(() => {
        setUser(null)
      })
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    fetchMe()
  }, [])

  // level 3 = admin (full), level 2 = editor (write), level 1 = viewer (read-only)
  const canWrite = (entity: string) => {
    if (!user) return false
    const perm = user.permissions[entity]
    if (!perm) return false
    // For plugins/upstreams, editor has level2 but should be read-only
    // The backend distinguishes these via CanWrite(role, entity)
    // We use level >= 2 AND entity-specific check
    if (entity === 'plugins' || entity === 'upstreams') {
      return user.role === 'admin'
    }
    return perm.level >= 2
  }

  const canDelete = (entity: string) => {
    if (!user) return false
    // Delete is same as write for now (admin/editor only)
    if (entity === 'plugins' || entity === 'upstreams') {
      return user.role === 'admin'
    }
    return user.role === 'admin'
  }

  return (
    <AuthContext.Provider value={{ user, loading, canWrite, canDelete, refetch: fetchMe }}>
      {children}
    </AuthContext.Provider>
  )
}

export const useAuth = () => useContext(AuthContext)
