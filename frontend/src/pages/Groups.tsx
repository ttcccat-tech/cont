import { useEffect, useState } from 'react'
import { Table, Button, Space, Tag, message, Modal, Form, Input, Popconfirm, Checkbox, Select, Tabs } from 'antd'
import { PlusOutlined, ReloadOutlined, DeleteOutlined, EditOutlined, LockOutlined, UserOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { AuthGroup, Resource, PermissionEntry, PermissionMode, ResourcePermission, getGroups, listResources, createGroup, updateGroup, deleteGroup, getGroupMembers, setGroupMembers, getGroupResourcePermissions, setGroupResourcePermissions, getUsers } from '../api/kong'

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
  const [activeTab, setActiveTab] = useState('permissions')
  const [members, setMembers] = useState<{id:string;username:string;display_name:string;email:string;role:string}[]>([])
  const [membersLoading, setMembersLoading] = useState(false)
  const [allUsers, setAllUsers] = useState<{id:string;username:string;display_name:string;email:string;role:string}[]>([])
  const [selectedUserIds, setSelectedUserIds] = useState<string[]>([])
  const [resPerms, setResPerms] = useState<ResourcePermission[]>([])
  const [resPermsLoading, setResPermsLoading] = useState(false)

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
      // Load current members
      if (group.id) {
        setMembersLoading(true)
        getGroupMembers(group.id).then(r => {
          setMembers(r.members || [])
          setSelectedUserIds((r.members || []).map((m: any) => m.id))
        }).catch(() => {}).finally(() => setMembersLoading(false))
        // Load resource permissions
        setResPermsLoading(true)
        getGroupResourcePermissions(group.id).then(r => {
          setResPerms(Array.isArray(r) ? r : [])
        }).catch(() => setResPerms([])).finally(() => setResPermsLoading(false))
      }
    } else {
      setEditingGroup(null)
      form.resetFields()
      setPermissions([])
      setMembers([])
      setSelectedUserIds([])
      setResPerms([])
    }
    setActiveTab('permissions')
    setModalOpen(true)
  }

  const loadAllUsers = () => {
    getUsers().then(r => setAllUsers(Array.isArray(r) ? r : [])).catch(() => {})
  }

  useEffect(() => {
    if (modalOpen && editingGroup) {
      loadAllUsers()
    }
  }, [modalOpen])

  const handleSubmit = async () => {
    if (submitting) return
    try {
      const values = await form.validateFields()
      setSubmitting(true)
      const payload = { ...values, permissions }

      if (editingGroup?.id) {
        await updateGroup(editingGroup.id, payload)
        // Save members
        await setGroupMembers(editingGroup.id, selectedUserIds)
        // Save resource permissions
        await setGroupResourcePermissions(editingGroup.id, resPerms)
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
        <Tabs
          activeKey={activeTab}
          onChange={setActiveTab}
          style={{ marginTop: 16 }}
          items={[
            {
              key: 'permissions',
              label: '權限配置',
              children: (
                <Form form={form} layout="vertical">
                  <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
                    <Form.Item
                      name="name"
                      label="群組名稱"
                      rules={[{ required: true, message: '必填' }]}
                    >
                      <Input placeholder="admin" disabled={!!editingGroup} />
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
              ),
            },
            {
              key: 'members',
              label: '群組成員',
              children: (
                <div>
                  <div style={{ marginBottom: 12, color: 'var(--muted)', fontSize: 13 }}>
                    選擇此群組的成員（目前已有 {members.length} 位成員）：
                  </div>
                  <Select
                    mode="multiple"
                    style={{ width: '100%', marginBottom: 16 }}
                    placeholder="選擇使用者..."
                    value={selectedUserIds}
                    onChange={setSelectedUserIds}
                    loading={membersLoading}
                    options={allUsers.map(u => ({
                      value: u.id,
                      label: `${u.username}${u.display_name ? ` (${u.display_name})` : ''}`,
                    }))}
                  />
                  {members.length > 0 && (
                    <div>
                      <div style={{ fontSize: 12, color: 'var(--muted)', marginBottom: 8 }}>
                        目前成員：
                      </div>
                      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
                        {members.map(m => (
                          <Tag key={m.id} color="cyan" icon={<UserOutlined />}>
                            {m.username}{m.display_name ? ` (${m.display_name})` : ''}
                          </Tag>
                        ))}
                      </div>
                    </div>
                  )}
                </div>
              ),
            },
            {
              key: 'resource-permissions',
              label: '資源權限',
              children: (
                <div>
                  <div style={{ marginBottom: 12, color: 'var(--muted)', fontSize: 13 }}>
                    針對特定資源（service/route/upstream）設定群組層級的權限覆寫：</div>
                  {resPermsLoading ? (
                    <div style={{ color: 'var(--muted)', textAlign: 'center', padding: 20 }}>載入中...</div>
                  ) : (
                    <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
                      <thead>
                        <tr style={{ background: 'var(--accent)' }}>
                          <th style={{ padding: '8px 12px', textAlign: 'left', color: 'var(--text)' }}>資源</th>
                          <th style={{ padding: '8px 12px', textAlign: 'left', color: 'var(--text)' }}>路徑</th>
                          <th style={{ padding: '8px 12px', textAlign: 'center', color: 'var(--text)', width: 90 }}>拒絕</th>
                          <th style={{ padding: '8px 12px', textAlign: 'center', color: 'var(--text)', width: 90 }}>讀取</th>
                          <th style={{ padding: '8px 12px', textAlign: 'center', color: 'var(--text)', width: 90 }}>寫入</th>
                        </tr>
                      </thead>
                      <tbody>
                        {resources.map((r, i) => {
                          const current = resPerms.find(p => p.resource_id === r.id)
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
                              {(['deny', 'read', 'write'] as const).map(perm => (
                                <td key={perm} style={{ textAlign: 'center', padding: '8px 12px' }}>
                                  <Checkbox
                                    checked={current?.permission === perm}
                                    disabled={submitting}
                                    onChange={() => {
                                      if (current?.permission === perm) {
                                        setResPerms(resPerms.filter(p => p.resource_id !== r.id))
                                      } else {
                                        const filtered = resPerms.filter(p => p.resource_id !== r.id)
                                        setResPerms([...filtered, { resource_id: r.id, permission: perm }])
                                      }
                                    }}
                                    aria-label={`${r.name} - ${perm}`}
                                  />
                                </td>
                              ))}
                            </tr>
                          )
                        })}
                      </tbody>
                    </table>
                  )}
                  {resPerms.length > 0 && (
                    <div style={{ marginTop: 12, fontSize: 12, color: 'var(--muted)' }}>
                      已設定 {resPerms.length} 項資源權限覆寫
                    </div>
                  )}
                </div>
              ),
            },
          ]}
        />
      </Modal>
    </div>
  )
}