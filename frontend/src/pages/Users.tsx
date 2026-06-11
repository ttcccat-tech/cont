import { useEffect, useState } from 'react'
import { Table, Button, Space, Tag, message, Modal, Form, Input, Popconfirm, Select } from 'antd'
import { PlusOutlined, ReloadOutlined, DeleteOutlined, EditOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { Workspace, listWorkspaces, getUsers, createUser, updateUser, deleteUser } from '../api/kong'

interface User {
  id: string
  username: string
  display_name?: string
  email?: string
  role: string
  enabled: boolean
  created_at: string
  groups: { name: string; label: string }[]
}

export default function Users() {
  const [users, setUsers] = useState<User[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editingUser, setEditingUser] = useState<User | null>(null)
  const [form] = Form.useForm()
  const [submitting, setSubmitting] = useState(false)
  const [allWorkspaces, setAllWorkspaces] = useState<Workspace[]>([])
  const [workspaceIds, setWorkspaceIds] = useState<string[]>([])

  const fetchUsers = () => {
    setLoading(true)
    getUsers()
      .then(r => setUsers(Array.isArray(r) ? r : []))
      .catch(() => message.error('無法連接 API'))
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    fetchUsers()
    listWorkspaces().then(setAllWorkspaces).catch(() => {})
  }, [])

  const openModal = (user?: User) => {
    if (user) {
      setEditingUser(user)
      form.setFieldsValue({
        username: user.username,
        display_name: user.display_name,
        email: user.email,
        role: user.role,
        enabled: user.enabled,
      })
      setWorkspaceIds((user as any).workspace_ids || [])
    } else {
      setEditingUser(null)
      form.resetFields()
      setWorkspaceIds([])
    }
    setModalOpen(true)
  }

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields()
      setSubmitting(true)
      const payload = { ...values, workspace_ids: workspaceIds }
      if (editingUser?.id) {
        await updateUser(editingUser.id, payload)
        message.success('更新成功')
      } else {
        await createUser({ ...payload, password: values.password || 'ChangeMe123' })
        message.success('建立成功')
      }
      setModalOpen(false)
      fetchUsers()
    } catch (e: any) {
      if (!e.errorFields) {
        message.error(e?.response?.data?.error || '操作失敗')
      }
    } finally {
      setSubmitting(false)
    }
  }

  const handleDelete = async (id: string, username: string) => {
    try {
      await deleteUser(id)
      message.success('刪除成功')
      fetchUsers()
    } catch (e: any) {
      message.error(e?.response?.data?.error || '刪除失敗')
    }
  }

  const columns: ColumnsType<User> = [
    {
      title: 'Username',
      dataIndex: 'username',
      key: 'username',
      render: v => <b style={{ color: 'var(--highlight)' }}>{v}</b>,
    },
    {
      title: '顯示名稱',
      dataIndex: 'display_name',
      key: 'display_name',
      render: v => v || <span style={{ color: 'var(--muted)' }}>—</span>,
    },
    {
      title: 'Email',
      dataIndex: 'email',
      key: 'email',
    },
    {
      title: '角色',
      dataIndex: 'role',
      key: 'role',
      render: v => {
        const colors: Record<string, string> = { admin: 'red', editor: 'blue', viewer: 'green' }
        return<Tag color={colors[v] || 'default'}>{v}</Tag>
      },
    },
    {
      title: '啟用',
      dataIndex: 'enabled',
      key: 'enabled',
      render: v => v ? <Tag color="green">是</Tag> : <Tag color="red">否</Tag>,
    },
    {
      title: '群組',
      dataIndex: 'groups',
      key: 'groups',
      render: (g: User['groups']) => g?.length
        ? g.map(gr => <Tag key={gr.name} color="cyan">{gr.label || gr.name}</Tag>)
        : <span style={{ color: 'var(--muted)' }}>無</span>,
    },
    {
      title: '建立時間',
      dataIndex: 'created_at',
      key: 'created_at',
      render: v => {
        if (!v) return '-'
        const d = new Date(v)
        return isNaN(d.getTime()) ? '-' : d.toLocaleString('zh-TW')
      },
    },
    {
      title: '操作',
      key: 'action',
      width: 150,
      render: (_, r) => (
        <Space>
          <Button size="small" icon={<EditOutlined />} onClick={() => openModal(r)}>編輯</Button>
          <Popconfirm title={`確認刪除使用者「${r.username}」？`} onConfirm={() => r.id && handleDelete(r.id, r.username)}>
            <Button size="small" danger icon={<DeleteOutlined />}>刪除</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <h1 style={{ margin: 0 }}>使用者管理</h1>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={fetchUsers}>刷新</Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => openModal()}>新增使用者</Button>
        </Space>
      </div>
      <Table
        columns={columns}
        dataSource={users as any}
        rowKey="id"
        loading={loading}
        pagination={{ pageSize: 10 }}
        locale={{ emptyText: '尚無使用者' }}
      />
      <Modal
        title={editingUser ? '編輯使用者' : '新增使用者'}
        open={modalOpen}
        onOk={handleSubmit}
        confirmLoading={submitting}
        onCancel={() => setModalOpen(false)}
        okText={editingUser ? '更新' : '建立'}
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="username" label="Username" rules={[{ required: true, message: '必填' }]}>
            <Input disabled={!!editingUser} />
          </Form.Item>
          <Form.Item name="display_name" label="顯示名稱">
            <Input />
          </Form.Item>
          <Form.Item name="email" label="Email">
            <Input type="email" />
          </Form.Item>
          <Form.Item name="role" label="角色" rules={[{ required: true, message: '必填' }]}>
            <Select options={[
              { value: 'admin', label: 'Admin' },
              { value: 'editor', label: 'Editor' },
              { value: 'viewer', label: 'Viewer' },
            ]} />
          </Form.Item>
          <Form.Item name="enabled" label="啟用" initialValue={true}>
            <Select options={[
              { value: true, label: '是' },
              { value: false, label: '否' },
            ]} />
          </Form.Item>
          <Form.Item label="工作區">
            <Select
              mode="multiple"
              value={workspaceIds}
              onChange={setWorkspaceIds}
              style={{ width: '100%' }}
              placeholder="選擇所屬工作區"
              options={allWorkspaces.map(w => ({ value: w.id, label: w.label || w.name }))}
            />
          </Form.Item>
          {!editingUser && (
            <Form.Item name="password" label="密碼">
              <Input.Password placeholder="預設: ChangeMe123" />
            </Form.Item>
          )}
        </Form>
      </Modal>
    </div>
  )
}
