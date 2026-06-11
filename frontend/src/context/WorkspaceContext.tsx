import { createContext, useContext, useState, useEffect, ReactNode } from 'react'
import api, { Workspace, listWorkspaces, listMyWorkspaces } from '../api/kong'

const WS_KEY = 'cont_ws'

export function getKongWorkspace(): string {
  return sessionStorage.getItem(WS_KEY) || 'default'
}

const WorkspaceContext = createContext<{
  workspaces: Workspace[]
  currentWorkspace: Workspace | null
  setCurrentWorkspace: (ws: Workspace | null) => void
  loading: boolean
}>({
  workspaces: [],
  currentWorkspace: null,
  setCurrentWorkspace: () => {},
  loading: true,
})

export function WorkspaceProvider({ children }: { children: ReactNode }) {
  const [workspaces, setWorkspaces] = useState<Workspace[]>([])
  const [currentWorkspace, setCurrentWorkspaceState] = useState<Workspace | null>(null)
  const [loading, setLoading] = useState(true)

  // Restore workspace from sessionStorage on mount
  useEffect(() => {
    const saved = sessionStorage.getItem(WS_KEY)
    if (saved) {
      listWorkspaces().then(res => {
        // Backend returns {data: [...]} or array — normalize to array
        const list = Array.isArray(res) ? res : (res?.data ?? [])
        const found = list.find((w: Workspace) => w.name === saved)
        if (found) setCurrentWorkspaceState(found)
      }).catch(() => {})
    }
  }, [])

  useEffect(() => {
    listMyWorkspaces()
      .then(res => {
        // Backend returns {data: [...]} or array — normalize to array
        const list = Array.isArray(res) ? res : (res?.data ?? [])
        setWorkspaces(list)
      })
      .catch(() => setWorkspaces([]))
      .finally(() => setLoading(false))
  }, [])

  const setCurrentWorkspace = (ws: Workspace | null) => {
    setCurrentWorkspaceState(ws)
    if (ws) {
      sessionStorage.setItem(WS_KEY, ws.name)
    } else {
      sessionStorage.removeItem(WS_KEY)
    }
  }

  return (
    <WorkspaceContext.Provider value={{ workspaces, currentWorkspace, setCurrentWorkspace, loading }}>
      {children}
    </WorkspaceContext.Provider>
  )
}

export const useWorkspace = () => useContext(WorkspaceContext)