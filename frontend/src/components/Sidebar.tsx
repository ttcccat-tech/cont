import { Layout, Menu, Select } from 'antd'
import {
  DashboardOutlined,
  ApiOutlined,
  UserOutlined,
  SettingOutlined,
  BarChartOutlined,
  KeyOutlined,
  ThunderboltOutlined,
  AuditOutlined,
  TeamOutlined,
  SafetyOutlined,
  HeartOutlined,
  HistoryOutlined,
  FileTextOutlined,
  BellOutlined,
  BookOutlined,
  NodeIndexOutlined,
  CloudOutlined,
} from '@ant-design/icons'
import { useNavigate, useLocation } from 'react-router-dom'
import { getUserPerms } from '../api/kong'
import { useWorkspace } from '../context/WorkspaceContext'

const { Sider } = Layout

export default function Sidebar() {
  const navigate = useNavigate()
  const location = useLocation()
  const perms = getUserPerms()
  const { workspaces, currentWorkspace, setCurrentWorkspace, loading } = useWorkspace()

  const baseItems = [
    { key: '/dashboard', icon: <DashboardOutlined />, label: 'dashboard 儀表板' },
    { key: '/services', icon: <ApiOutlined />, label: 'api 前端' },
    { key: '/routes', icon: <ThunderboltOutlined />, label: 'routes 路由' },
    { key: '/plugins', icon: <KeyOutlined />, label: 'plugins 插件' },
    { key: '/consumers', icon: <UserOutlined />, label: 'consumers 消費者' },
    { key: '/upstreams', icon: <NodeIndexOutlined />, label: 'api 負載平衡後端' },
    { key: '/grpc-services', icon: <CloudOutlined />, label: 'grpc gRPC 服務' },
    { key: '/analytics', icon: <BarChartOutlined />, label: 'analytics 統計報告' },
    { key: '/audit', icon: <AuditOutlined />, label: 'audit 審計日誌' },
    { key: '/config-versioning', icon: <HistoryOutlined />, label: 'versions 設定版本' },
    { key: '/health-portal', icon: <HeartOutlined />, label: 'health 服務健康度' },
    { key: '/alerts/rules', icon: <BellOutlined />, label: 'alerts 告警規則' },
    { key: '/api-key-requests', icon: <FileTextOutlined />, label: 'api keys API Key 申請' },
    { key: '/api-docs', icon: <BookOutlined />, label: 'api docs API 文件' },
    { key: '/settings', icon: <SettingOutlined />, label: 'settings 系統設置' },
  ]

  const adminItems = [
    { key: '/users', icon: <UserOutlined />, label: 'users 使用者' },
    { key: '/groups', icon: <TeamOutlined />, label: 'groups 群組' },
    { key: '/workspaces', icon: <SafetyOutlined />, label: 'workspaces 工作區' },
  ]

  const menuItems = perms.users || perms.groups ? [...baseItems, ...adminItems] : baseItems

  return (
    <Sider
      width={240}
      style={{
        background: 'var(--secondary)',
        borderRight: '1px solid var(--accent)',
        overflow: 'auto',
        height: '100vh',
        position: 'fixed',
        left: 0,
        top: 0,
        bottom: 0,
        zIndex: 100,
      }}
    >
      <div style={{ padding: '20px 16px', borderBottom: '1px solid var(--accent)' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
          <div style={{
            width: 36,
            height: 36,
            borderRadius: 8,
            background: 'linear-gradient(135deg, var(--highlight), var(--accent))',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontSize: 16,
            fontWeight: 700
          }}>
            K
          </div>
          <div>
            <div style={{ fontSize: 15, fontWeight: 600, color: 'var(--text)' }}>Cont</div>
            <div style={{ fontSize: 11, color: 'var(--muted)' }}>企業級 API 閘道器管理</div>
          </div>
        </div>
      </div>

      {!loading && workspaces.length > 0 && (
        <div style={{ padding: '8px 16px', borderBottom: '1px solid var(--accent)' }}>
          <Select
            value={currentWorkspace?.id || 'all'}
            onChange={(val) => {
              if (val === 'all') setCurrentWorkspace(null)
              else {
                const ws = workspaces.find(w => w.id === val)
                setCurrentWorkspace(ws || null)
              }
            }}
            style={{ width: '100%' }}
            options={[
              { value: 'all', label: '全部工作區' },
              ...workspaces.map(w => ({ value: w.id, label: w.name }))
            ]}
          />
        </div>
      )}

      <Menu
        mode="inline"
        selectedKeys={[location.pathname]}
        onClick={({ key }) => navigate(key)}
        style={{
          background: 'transparent',
          border: 'none',
          color: 'var(--text)'
        }}
        items={menuItems.map(item => ({
          ...item,
          style: {
            background: location.pathname === item.key ? 'var(--accent)' : 'transparent',
            borderRadius: 6,
            margin: '2px 8px',
            width: 'calc(100% - 16px)'
          }
        }))}
      />
    </Sider>
  )
}