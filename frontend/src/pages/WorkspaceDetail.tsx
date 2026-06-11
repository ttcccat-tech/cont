import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { Card, Table, Button, Space, Tag, message, Modal, Form, Input, Select, Popconfirm, Tabs, Divider, Row, Col, Descriptions } from 'antd'
import { PlusOutlined, DeleteOutlined, ReloadOutlined, ArrowLeftOutlined, UserOutlined, EditOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { getWorkspaceUsers, setWorkspaceUser, removeWorkspaceUser, getUsers, updateWorkspace, Workspace, WorkspaceUserAssignment, getGroups } from '../api/kong'
import { useWorkspace } from '../context/WorkspaceContext'

export default function WorkspaceDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { workspaces } = useWorkspace()

  const [workspace, setWorkspace] = useState<Workspace | null>(null)
  const [members, setMembers] = useState<WorkspaceUserAssignment[]>([])
  const [allUsers, setAllUsers] = useState<{id: string; username: string; display_name?: string; email?: string; role: string}[]>([])
  const [loading, setLoading] = useState(false)
  const [membersLoading, setMembersLoading] = useState(false)
  const [addModalOpen, setAddModalOpen] = useState(false)
  const [editingMember, setEditingMember] = useState<WorkspaceUserAssignment | null>(null)
  const [editRole, setEditRole] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [activeTab, setActiveTab] = useState('members')
  const [form] = Form.useForm()

  useEffect(() => {
    if (!id) return
    const found = workspaces.find(w => w.id === id)
    if (found) setWorkspace(found)
  }, [id, workspaces])

  const fetchMembers = () => {
    if (!id) return
    setMembersLoading(true)
    getWorkspaceUsers(id)
      .then(setMembers)
      .catch(() => message.error('無法載入成員清單'))
      .finally(() => setMembersLoading(false))
  }

  const loadAllUsers = () => {
    getUsers().then(r => {
      const users = Array.isArray(r) ? r : []
      setAllUsers(users.map((u: any) => ({
        id: u.id,
        username: u.username,
        display_name: u.display_name,
        email: u.email,
        role: u.role,
      })))
    }).catch(() => {})
  }

  useEffect(() => {
    if (id) fetchMembers()
  }, [id])

  useEffect(() => {
    if (addModalOpen) loadAllUsers()
  }, [addModalOpen])

  const handleAddMember = async () => {
    try {
      const values = await form.validateFields()
      if (!id) return
      setSubmitting(true)
      await setWorkspaceUser(id, values.user_id, values.role)
      message.success('已指派成員')
      setAddModalOpen(false)
      form.resetFields()
      fetchMembers()
    } catch (e: any) {
      if (!e.errorFields) {
        message.error('操作失敗: ' + (e?.response?.data?.message || e.message || ''))
      }
    } finally {
      setSubmitting(false)
    }
  }

  const handleUpdateRole = async (userId: string) => {
    if (!id || !editRole) return
    setSubmitting(true)
    try {
      await setWorkspaceUser(id, userId, editRole)
      message.success('角色已更新')
      setEditingMember(null)
      setEditRole('')
      fetchMembers()
    } catch (e: any) {
      message.error('更新失敗: ' + (e?.message || ''))
    } finally {
      setSubmitting(false)
    }
  }

  const handleRemoveMember = async (userId: string) => {
    if (!id) return
    try {
      await removeWorkspaceUser(id, userId)
      message.success('已移除成員')
      fetchMembers()
    } catch (e: any) {
      message.error('移除失敗: ' + (e?.message || ''))
    }
  }

  const memberColumns: ColumnsType<WorkspaceUserAssignment> = [
    {
      title: '使用者',
      key: 'user',
      render: (_, m) => (
        <Space>
          <UserOutlined style={{ color: 'var(--muted)' }} />
          <span style={{ color: 'var(--highlight)', fontWeight: 600 }}>{m.username}</span>
          {m.display_name && <span style={{ color: 'var(--muted)' }}>({m.display_name})</span>}
        </Space>
      ),
    },
    {
      title: 'Email',
      dataIndex: 'email',
      key: 'email',
      render: v => v || <span style={{ color: 'var(--muted)' }}>—</span>,
    },
    {
      title: 'Workspace 角色',
      dataIndex: 'role',
      key: 'role',
      render: (v, record) => {
        if (editingMember?.user_id === record.user_id) {
          return (
            <Space>
              <Select
                value={editRole}
                onChange={setEditRole}
                style={{ width: 120 }}
                options={[
                  { value: 'viewer', label: 'viewer' },
                  { value: 'editor', label: 'editor' },
                  { value: 'admin', label: 'admin' },
                ]}
              />
              <Button size="small" type="primary" onClick={() => handleUpdateRole(record.user_id)} loading={submitting}>儲存</Button>
              <Button size="small" onClick={() => { setEditingMember(null); setEditRole('') }}>取消</Button>
            </Space>
          )
        }
        const colorMap: Record<string, string> = { admin: 'red', editor: 'orange', viewer: 'blue' }
        return (
          <Space>
            <Tag color={colorMap[v] || 'default'}>{v}</Tag>
            <Button size="small" icon={<EditOutlined />} onClick={() => { setEditingMember(record); setEditRole(v) }} />
          </Space>
        )
      },
    },
    {
      title: '指派時間',
      dataIndex: 'assigned_at',
      key: 'assigned_at',
      render: v => {
        if (!v) return '-'
        const d = new Date(v)
        return isNaN(d.getTime()) ? '-' : d.toLocaleString('zh-TW')
      },
    },
    {
      title: '操作',
      key: 'action',
      width: 120,
      render: (_, m) => (
        <Popconfirm
          title={`確認移除 ${m.username} 的 workspace 存取？`}
          onConfirm={() => handleRemoveMember(m.user_id)}
        >
          <Button size="small" danger icon={<DeleteOutlined />}>
            移除
          </Button>
        </Popconfirm>
      ),
    },
  ]

  // Available users not yet in this workspace
  const availableUsers = allUsers.filter(u => !members.find(m => m.user_id === u.id))

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 24 }}>
        <Space>
          <Button icon={<ArrowLeftOutlined />} onClick={() => navigate(-1)}>返回</Button>
          <h1 style={{ margin: 0 }}>{workspace?.name || 'Workspace'}</h1>
        </Space>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={fetchMembers}>刷新</Button>
        </Space>
      </div>

      <Tabs
        activeKey={activeTab}
        onChange={setActiveTab}
        items={[
          {
            key: 'members',
            label: '成員管理',
            children: (
              <div>
                <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
                  <span style={{ color: 'var(--muted)' }}>
                    已指派 <b>{members.length}</b> 位成員至此 Workspace
                  </span>
                  <Button type="primary" icon={<PlusOutlined />} onClick={() => setAddModalOpen(true)}>
                    新增成員
                  </Button>
                </div>

                <Table
                  columns={memberColumns}
                  dataSource={members}
                  rowKey="user_id"
                  loading={membersLoading}
                  pagination={{ pageSize: 10 }}
                  locale={{ emptyText: '尚無成員，點擊「新增成員」指派第一位成員' }}
                />
              </div>
            ),
          },
          {
            key: 'info',
            label: '基本資訊',
            children: (
              <Card style={{ background: 'var(--secondary)', border: 'none' }}>
                <Descriptions column={2}>
                  <Descriptions.Item label="Workspace ID">{id}</Descriptions.Item>
                  <Descriptions.Item label="名稱">{workspace?.name}</Descriptions.Item>
                  <Descriptions.Item label="標籤">{workspace?.label || '—'}</Descriptions.Item>
                  <Descriptions.Item label="描述">{workspace?.description || '—'}</Descriptions.Item>
                </Descriptions>
              </Card>
            ),
          },
        ]}
      />

      <Modal
        title="新增 Workspace 成員"
        open={addModalOpen}
        onOk={handleAddMember}
        confirmLoading={submitting}
        onCancel={() => { setAddModalOpen(false); form.resetFields() }}
        okText="指派"
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item
            name="user_id"
            label="選擇使用者"
            rules={[{ required: true, message: '請選擇使用者' }]}
          >
            <Select
              showSearch
              placeholder="搜尋使用者..."
              options={availableUsers.map(u => ({
                value: u.id,
                label: `${u.username}${u.display_name ? ` (${u.display_name})` : ''}${u.email ? ` - ${u.email}` : ''}`,
              }))}
              filterOption={(input, option) =>
                (option?.label ?? '').toLowerCase().includes(input.toLowerCase())
              }
            />
          </Form.Item>
          <Form.Item
            name="role"
            label="Workspace 角色"
            rules={[{ required: true, message: '請選擇角色' }]}
            initialValue="viewer"
          >
            <Select
              options={[
                { value: 'viewer', label: 'viewer — 唯讀' },
                { value: 'editor', label: 'editor — 可讀寫' },
                { value: 'admin', label: 'admin — 管理員' },
              ]}
            />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
