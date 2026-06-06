import { useEffect, useState } from 'react'
import { Table, Button, Space, Tag, message, Modal, Form, Input, Popconfirm, Checkbox } from 'antd'
import { PlusOutlined, ReloadOutlined, DeleteOutlined, EditOutlined, LockOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { AuthGroup, Resource, PermissionEntry, PermissionMode, getGroups, listResources, createGroup, updateGroup, deleteGroup } from '../api/kong'

// ===== Permission Matrix Component =====

interface PermissionMatrixProps {
  resources: Resource[]
  value: PermissionEntry[]
  onChange: (perms: PermissionEntry[]) => void
  disabled?: boolean
}

function PermissionMatrix({ resources, value, onChange, disabled }: PermissionMatrixProps) {
  const modes: { key: PermissionMode; label: string }[] = [
    { key: 'deny', label: '拒絕' },
    { key: 'read', label: '讀取' },
    { key: 'write', label: '寫入' },
  ]

  const getMode = (resourceId: string): PermissionMode | null => {
    const entry = value.find(p => p.resource_id === resourceId)
    return entry?.mode ?? null
  }

  const toggle = (resourceId: string, mode: PermissionMode) => {
    if (disabled) return
    const existing = value.find(p => p.resource_id === resourceId)
    if (existing?.mode === mode) {
      onChange(value.filter(p => p.resource_id !== resourceId))
    } else {
      const filtered = value.filter(p => p.resource_id !== resourceId)
      onChange([...filtered, { resource_id: resourceId, mode }])
    }
  }

  return (
    <div style={{ overflowX: 'auto' }}>
      <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
        <thead>
          <tr style={{ background: 'var(--accent)' }}>
            <th style={{ padding: '8px 12px', textAlign: 'left', color: 'var(--text)', minWidth: 160 }}>資源</th>
            <th style={{ padding: '8px 12px', textAlign: 'left', color: 'var(--text)', minWidth: 120 }}>路徑</th>
            {modes.map(m => (
              <th key={m.key} style={{ padding: '8px 12px', textAlign: 'center', color: 'var(--text)', width: 90 }}>
                {m.label}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {resources.map((r, i) => {
            const current = getMode(r.id)
            return (
              <tr key={r.id} style={{ background: i % 2 === 0 ? 'var(--secondary)' : 'transparent' }}>
                <td style={{ padding: '8px 12px' }}>
                  <Space>
                    <LockOutlined style={{ color: 'var(--muted)', fontSize: 11 }} />
                    <span style={{ color: 'var(--highlight)' }}>{r.name}</span>
                  </Space>
                </td>
                <td style={{ padding: '8px 12px' }}>
                  <code style={{ color: 'var(--muted)', fontSize: 11 }}>{r.path}</code>
                </td>
                {modes.map(m => (
                  <td key={m.key} style={{ textAlign: 'center', padding: '8px 12px' }}>
                    <Checkbox
                      checked={current === m.key}
                      disabled={disabled}
                      onChange={() => toggle(r.id, m.key)}
                      aria-label={`${r.name} - ${m.label}`}
                    />
                  </td>
                ))}
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}

// ===== Groups Page =====

export default function GroupsPage() {
  const [groups, setGroups] = useState<AuthGroup[]>([])
  const [resources, setResources] = useState<Resource[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editingGroup, setEditingGroup] = useState<AuthGroup | null>(null)
  const [form] = Form.useForm()
  const [permissions, setPermissions] = useState<PermissionEntry[]>([])
  const [submitting, setSubmitting] = useState(false)
  const [resourcesLoading, setResourcesLoading] = useState(false)

  const fetchGroups = () => {
    setLoading(true)
    getGroups()
      .then(r => setGroups(Array.isArray(r) ? r : []))
      .catch(() => message.error('無法連接 API'))
      .finally(() => setLoading(false))
  }

  const fetchResources = () => {
    setResourcesLoading(true)
    listResources()
      .then(r => setResources(Array.isArray(r) ? r : []))
      .catch(() => message.error('無法取得資源清單'))
      .finally(() => setResourcesLoading(false))
  }

  useEffect(() => {
    fetchGroups()
    fetchResources()
  }, [])

  const openModal = (group?: AuthGroup) => {
    if (group) {
      setEditingGroup(group)
      form.setFieldsValue({
        name: group.name,
        label: group.label,
        description: group.description,
      })
      setPermissions(group.permissions || [])
    } else {
      setEditingGroup(null)
      form.resetFields()
      setPermissions([])
    }
    setModalOpen(true)
  }

  const handleSubmit = async () => {
    if (submitting) return
    try {
      const values = await form.validateFields()
      setSubmitting(true)
      const payload = { ...values, permissions }

      if (editingGroup?.id) {
        await updateGroup(editingGroup.id, payload)
        message.success('群組更新成功')
      } else {
        await createGroup(payload)
        message.success('群組建立成功')
      }
      setModalOpen(false)
      fetchGroups()
    } catch (e: any) {
      if (!e.errorFields) {
        message.error('操作失敗: ' + (e?.response?.data?.message || e.message || ''))
      }
    } finally {
      setSubmitting(false)
    }
  }

  const handleDelete = async (id: string, name: string) => {
    try {
      await deleteGroup(id)
      message.success('刪除成功')
      fetchGroups()
    } catch (e: any) {
      message.error('刪除失敗: ' + (e.message || ''))
    }
  }

  const columns: ColumnsType<AuthGroup> = [
    {
      title: '群組名稱',
      dataIndex: 'name',
      key: 'name',
      render: (v) => <b style={{ color: 'var(--highlight)' }}>{v}</b>,
    },
    {
      title: '標籤',
      dataIndex: 'label',
      key: 'label',
      render: v => v ? <Tag color="cyan">{v}</Tag> : <span style={{ color: 'var(--muted)' }}>—</span>,
    },
    {
      title: '說明',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
      render: v => v || <span style={{ color: 'var(--muted)' }}>—</span>,
    },
    {
      title: '權限數',
      dataIndex: 'permissions',
      key: 'permissions',
      render: (p: PermissionEntry[]) => p?.length ? (
        <Tag color="purple">{p.length} 項</Tag>
      ) : (
        <span style={{ color: 'var(--muted)' }}>無</span>
      ),
    },
    {
      title: '建立時間',
      dataIndex: 'created_at',
      key: 'created_at',
      render: v => {
        if (!v) return '-'
        const d = typeof v === 'number' ? new Date(v * 1000) : new Date(v)
        return isNaN(d.getTime()) ? '-' : d.toLocaleString('zh-TW')
      },
    },
    {
      title: '操作',
      key: 'action',
      width: 180,
      render: (_, r) => (
        <Space>
          <Button size="small" icon={<EditOutlined />} onClick={() => openModal(r)}>
            編輯
          </Button>
          <Popconfirm
            title={`確認刪除群組「${r.name}」？`}
            onConfirm={() => r.id && handleDelete(r.id, r.name)}
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
        <h1>群組管理</h1>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={fetchGroups}>刷新</Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => openModal()}>
            新增群組
          </Button>
        </Space>
      </div>

      <Table
        columns={columns}
        dataSource={groups as any}
        rowKey="id"
        loading={loading}
        pagination={{ pageSize: 10 }}
        locale={{ emptyText: '尚無群組，點擊「新增群組」建立第一個群組' }}
      />

      <Modal
        title={editingGroup ? '編輯群組' : '新增群組'}
        open={modalOpen}
        onOk={handleSubmit}
        confirmLoading={submitting}
        onCancel={() => setModalOpen(false)}
        okText={editingGroup ? '更新' : '建立'}
        width={860}
        styles={{ body: { maxHeight: '70vh', overflow: 'auto' } }}
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
            <Form.Item
              name="name"
              label="群組名稱"
              rules={[{ required: true, message: '必填' }]}
            >
              <Input placeholder="admin" />
            </Form.Item>
            <Form.Item name="label" label="標籤">
              <Input placeholder="系統管理員" />
            </Form.Item>
          </div>
          <Form.Item name="description" label="說明">
            <Input.TextArea placeholder="可選，描述此群組的用途..." rows={2} />
          </Form.Item>

          <div style={{ marginTop: 8 }}>
            <div style={{ marginBottom: 8, color: 'var(--muted)', fontSize: 13 }}>
              權限矩陣：勾選資源與權限模式後，系統將自動套用相應的存取控制策略。
            </div>
            {resourcesLoading ? (
              <div style={{ color: 'var(--muted)', textAlign: 'center', padding: 20 }}>載入資源中...</div>
            ) : resources.length === 0 ? (
              <div style={{ color: 'var(--muted)', textAlign: 'center', padding: 20 }}>
                尚無可用資源，或後端未提供 /api/resources 端點。
              </div>
            ) : (
              <PermissionMatrix
                resources={resources}
                value={permissions}
                onChange={setPermissions}
                disabled={submitting}
              />
            )}
          </div>

          {permissions.length > 0 && (
            <div style={{ marginTop: 12, fontSize: 12, color: 'var(--muted)' }}>
              已設定 {permissions.length} 項權限
            </div>
          )}
        </Form>
      </Modal>
    </div>
  )
}