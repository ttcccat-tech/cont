import { useEffect, useState } from 'react'
import { Table, Button, Space, Tag, message, Modal, Form, Input, Popconfirm, Card } from 'antd'
import { PlusOutlined, DeleteOutlined, ReloadOutlined, TeamOutlined, SettingOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { listWorkspaces, createWorkspace, deleteWorkspace, Workspace } from '../api/kong'
import { useNavigate } from 'react-router-dom'

export default function WorkspacesPage() {
  const [workspaces, setWorkspaces] = useState<Workspace[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [form] = Form.useForm()
  const [submitting, setSubmitting] = useState(false)
  const navigate = useNavigate()

  const fetchWorkspaces = () => {
    setLoading(true)
    listWorkspaces()
      .then(r => setWorkspaces(Array.isArray(r) ? r : []))
      .catch(() => message.error('無法載入 workspace 清單'))
      .finally(() => setLoading(false))
  }

  useEffect(() => { fetchWorkspaces() }, [])

  const handleCreate = async () => {
    try {
      const values = await form.validateFields()
      setSubmitting(true)
      await createWorkspace(values)
      message.success('Workspace 建立成功')
      setModalOpen(false)
      form.resetFields()
      fetchWorkspaces()
    } catch (e: any) {
      if (!e.errorFields) {
        message.error('建立失敗: ' + (e?.response?.data?.message || e.message || ''))
      }
    } finally {
      setSubmitting(false)
    }
  }

  const handleDelete = async (id: string) => {
    try {
      await deleteWorkspace(id)
      message.success('刪除成功')
      fetchWorkspaces()
    } catch (e: any) {
      message.error('刪除失敗: ' + (e?.message || ''))
    }
  }

  const columns: ColumnsType<Workspace> = [
    {
      title: '名稱',
      dataIndex: 'name',
      key: 'name',
      render: (v, r) => (
        <Button type="link" onClick={() => navigate(`/workspaces/${r.id}`)} style={{ padding: 0, height: 'auto' }}>
          <b style={{ color: 'var(--highlight)', fontSize: 14 }}>{v}</b>
        </Button>
      ),
    },
    {
      title: '標籤',
      dataIndex: 'label',
      key: 'label',
      render: v => v ? <Tag color="cyan">{v}</Tag> : <span style={{ color: 'var(--muted)' }}>—</span>,
    },
    {
      title: '描述',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
      render: v => v || <span style={{ color: 'var(--muted)' }}>—</span>,
    },
    {
      title: '操作',
      key: 'action',
      width: 200,
      render: (_, r) => (
        <Space>
          <Button size="small" icon={<SettingOutlined />} onClick={() => navigate(`/workspaces/${r.id}`)}>
            成員管理
          </Button>
          <Popconfirm
            title={`確認刪除 Workspace「${r.name}」？`}
            onConfirm={() => r.id && handleDelete(r.id)}
          >
            <Button size="small" danger icon={<DeleteOutlined />}>
              刪除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <h1>Workspace 管理</h1>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={fetchWorkspaces}>刷新</Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setModalOpen(true)}>
            新增 Workspace
          </Button>
        </Space>
      </div>

      <Table
        columns={columns}
        dataSource={workspaces as any}
        rowKey="id"
        loading={loading}
        pagination={{ pageSize: 10 }}
        locale={{ emptyText: '尚無 Workspace，點擊「新增 Workspace」建立第一個工作區' }}
      />

      <Modal
        title="新增 Workspace"
        open={modalOpen}
        onOk={handleCreate}
        confirmLoading={submitting}
        onCancel={() => { setModalOpen(false); form.resetFields() }}
        okText="建立"
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="name" label="名稱" rules={[{ required: true, message: '必填' }]}>
            <Input placeholder="production" />
          </Form.Item>
          <Form.Item name="label" label="標籤">
            <Input placeholder="生產環境" />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <Input.TextArea placeholder="可選，描述此 Workspace 的用途..." rows={2} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}