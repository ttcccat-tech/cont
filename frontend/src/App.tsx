import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { Layout, Button, Result } from 'antd'
import { useState } from 'react'
import React from 'react'
import Login from './pages/Login'
import Users from './pages/Users'
import Groups from './pages/Groups'
import Sidebar from './components/Sidebar'
import Dashboard from './pages/Dashboard'
import Services from './pages/Services'
import RoutesPage from './pages/Routes'
import Plugins from './pages/Plugins'
import Consumers from './pages/Consumers'
import Analytics from './pages/Analytics'
import Settings from './pages/Settings'
import AuditLog from './pages/AuditLog'
import ConfigVersioning from './pages/ConfigVersioning'
import HealthPortal from './pages/HealthPortal'
import AlertRules from './pages/AlertRules'
import AlertHistory from './pages/AlertHistory'
import ApiKeyRequests from './pages/ApiKeyRequests'
import ApiDocs from './pages/ApiDocs'
import Upstreams from './pages/Upstreams'
import GrpcServices from './pages/GrpcServices'
import WorkspaceDetail from './pages/WorkspaceDetail'
import Workspaces from './pages/Workspaces'
import WebhookDeliveries from './pages/WebhookDeliveries'
import { getToken, clearAuth } from './api/kong'
import { WorkspaceProvider } from './context/WorkspaceContext'
import { AuthProvider } from './context/AuthContext'
import { EventListener } from './components/EventListener'

const { Header, Content } = Layout

interface ProtectedRouteProps {
  children: React.ReactNode
}

class ErrorBoundary extends React.Component<{children: React.ReactNode}, {hasError: boolean; error: any}> {
  constructor(props: any) {
    super(props)
    this.state = { hasError: false, error: null }
  }
  static getDerivedStateFromError(error: any) {
    return { hasError: true, error }
  }
  componentDidCatch(error: any, info: any) {
    console.error('RENDER ERROR:', error?.message, error?.stack, 'COMPONENT STACK:', info?.componentStack)
  }
  render() {
    if (this.state.hasError) {
      return <Result status="error" title="Render Error" subTitle={this.state.error?.message} />
    }
    return this.props.children
  }
}

function ProtectedRoute({ children }: ProtectedRouteProps) {
  const token = getToken()
  if (!token) {
    return <Navigate to="/login" replace />
  }
  return <>{children}</>
}

function AppLayout({ children }: { children: React.ReactNode }) {
  return (
    <Layout style={{ overflow: 'hidden', height: '100vh' }}>
      <Sidebar />
      <Layout>
        <Header style={{
          background: 'var(--primary)',
          borderBottom: '1px solid var(--accent)',
          padding: '0 24px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          marginLeft: 240,
          position: 'fixed',
          top: 0,
          right: 0,
          left: 0,
          zIndex: 99,
        }}>
          <span style={{ color: 'var(--highlight)', fontSize: 18, fontWeight: 600 }}>
            cont
          </span>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <span style={{ color: 'var(--muted)', fontSize: 13 }}>v2.0</span>
            <Button
              size="small"
              onClick={() => {
                clearAuth()
                window.location.href = '/login'
              }}
              style={{ color: 'var(--muted)', borderColor: 'var(--accent)' }}
            >
              登出
            </Button>
          </div>
        </Header>
        <Content style={{
          padding: 24,
          paddingTop: 80,
          background: 'var(--primary)',
          marginLeft: 240,
          minHeight: '100vh',
          overflow: 'auto'
        }}>
          {children}
        </Content>
      </Layout>
    </Layout>
  )
}

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route path="/*" element={
          <ProtectedRoute>
            <ErrorBoundary>
            <WorkspaceProvider>
              <AuthProvider>
                <AppLayout>
                  <EventListener />
                  <Routes>
                  <Route path="/" element={<Navigate to="/dashboard" replace />} />
                  <Route path="/dashboard" element={<Dashboard />} />
                  <Route path="/services" element={<Services />} />
                  <Route path="/routes" element={<RoutesPage />} />
                  <Route path="/plugins" element={<Plugins />} />
                  <Route path="/consumers" element={<Consumers />} />
                  <Route path="/upstreams" element={<Upstreams />} />
                  <Route path="/grpc-services" element={<GrpcServices />} />
                  <Route path="/analytics" element={<Analytics />} />
                  <Route path="/settings" element={<Settings />} />
                  <Route path="/billing" element={<Settings />} />
                  <Route path="/audit" element={<AuditLog />} />
                  <Route path="/config-versioning" element={<ConfigVersioning />} />
                  <Route path="/config-snapshots" element={<ConfigVersioning />} />
                  <Route path="/health-portal" element={<HealthPortal />} />
                  <Route path="/alerts/rules" element={<AlertRules />} />
                  <Route path="/alert-history" element={<AlertHistory />} />
                  <Route path="/webhook-deliveries" element={<WebhookDeliveries />} />
                  <Route path="/api-key-requests" element={<ApiKeyRequests />} />
                  <Route path="/api-docs" element={<ApiDocs />} />
                  <Route path="/users" element={<Users />} />
                  <Route path="/groups" element={<Groups />} />
                  <Route path="/workspaces" element={<Workspaces />} />
                  <Route path="/workspaces/:id" element={<WorkspaceDetail />} />
                </Routes>
              </AppLayout>
            </AuthProvider>
          </WorkspaceProvider>
          </ErrorBoundary>
        </ProtectedRoute>
        } />
      </Routes>
    </BrowserRouter>
  )
}
