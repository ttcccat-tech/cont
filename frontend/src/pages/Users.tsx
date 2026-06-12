import { useEffect, useState } from 'react'
import { Table, Button, Space, Tag, message, Modal, Form, Input, Popconfirm, Select, Drawer, Checkbox, List, Divider, Alert, Tabs } from 'antd'
import { PlusOutlined, ReloadOutlined, DeleteOutlined, EditOutlined, GroupOutlined, TeamOutlined, LockOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { Workspace, listWorkspaces, getUsers, createUser, updateUser, deleteUser, getGroups, getGroupMembers, setGroupMembers, getUserWorkspaces, setWorkspaceUser, removeWorkspaceUser, AuthGroup, Resource, ResourcePermission, getUserResourcePermissions, setUserResourcePermissions, listResources } from '../api/kong'

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
  // Group assignment
  const [groupDrawerOpen, setGroupDrawerOpen] = useState(false)
  const [assigningUser, setAssigningUser] = useState<User | null>(null)
  const [allGroups, setAllGroups] = useState<AuthGroup[]>([])
  const [selectedGroupIds, setSelectedGroupIds] = useState<string[]>([])
  const [groupLoading, setGroupLoading] = useState(false)
  const [groupSaving, setGroupSaving] = useState(false)

  // Workspace assignment
  const [wsDrawerOpen, setWsDrawerOpen] = useState(false)
  const [wsAssigningUser, setWsAssigningUser] = useState<User | null>(null)
  const [userWorkspaces, setUserWorkspaces] = useState<{ id: string; name: string; role: string }[]>([])
  const [wsLoading, setWsLoading] = useState(false)
  const [wsSaving, setWsSaving] = useState(false)

  // Resource Permissions assignment
  const [resPermDrawerOpen, setResPermDrawerOpen] = useState(false)
  const [resPermUser, setResPermUser] = useState<User | null>(null)
  const [allResources, setAllResources] = useState<Resource[]>([])
  const [userResPerms, setUserResPerms] = useState<ResourcePermission[]>([])
  const [resPermLoading, setResPermLoading] = useState(false)
  const [resPermSaving, setResPermSaving] = useState(false)

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

  // ── Group assignment ───────────────────────────────────
  const openGroupDrawer = async (user: User) => {
    setAssigningUser(user)
    setGroupDrawerOpen(true)
    setGroupLoading(true)
    try {
      const [groups, ...memberResults] = await Promise.all([
        getGroups(),
        ...(user.groups || []).map(g =>
          g.name ? getGroupMembers(g.name).catch(() => ({ members: [] })) : Promise.resolve({ members: [] })
        ),
      ])
      setAllGroups(groups)
      // Build set of group names this user belongs to
      const memberGroups = new Set<string>()
      for (const res of memberResults) {
        const m = (res as any)?.members || []
        if (m.some((u: any) => u.id === user.id)) {
          // Find which group's member list contained this user
          // We need to map back — use group names from user.groups
        }
      }
      // Use the groups array from user object directly
      setSelectedGroupIds(user.groups?.map(g => g.name) || [])
    } catch (e) {
      message.error('無法載入群組列表')
    } finally {
      setGroupLoading(false)
    }
  }

  const handleSaveGroups = async () => {
    if (!assigningUser?.id) return
    setGroupSaving(true)
    try {
      // For each group, set its members
      // We need to know which groups to add user to vs remove from
      // Simple approach: get current members of each group, diff
      const currentGroupIds = assigningUser.groups?.map(g => g.name) || []
      const toAdd = selectedGroupIds.filter(id => !currentGroupIds.includes(id))
      const toRemove = currentGroupIds.filter(id => !selectedGroupIds.includes(id))
      await Promise.all([
        ...toAdd.map(groupId => getGroupMembers(groupId).then(res => {
          const members: string[] = (res as any)?.members?.map((m: any) => m.id) || []
          if (!members.includes(assigningUser.id!)) {
            return setGroupMembers(groupId, [...members, assigningUser.id!])
          }
        }).catch(() => {})),
        ...toRemove.map(groupId => getGroupMembers(groupId).then(res => {
          const members: string[] = (res as any)?.members?.map((m: any) => m.id) || []
          return setGroupMembers(groupId, members.filter(id => id !== assigningUser.id))
        }).catch(() => {})),
      ])
      message.success('群組指派已更新')
      setGroupDrawerOpen(false)
      fetchUsers()
    } catch (e: any) {
      message.error('儲存失敗: ' + (e?.message || ''))
    } finally {
      setGroupSaving(false)
    }
  }

  // ── Workspace assignment ──────────────────────────────────
  const openWsDrawer = async (user: User) => {
    setWsAssigningUser(user)
    setWsDrawerOpen(true)
    setWsLoading(true)
    try {
      const ws = await getUserWorkspaces(user.id)
      setUserWorkspaces(ws.map((w: any) => ({ id: w.workspace_id, name: w.workspace_name || w.name || 'Unknown', role: w.role })))
    } catch (e) {
      message.error('無法載入工作區列表')
    } finally {
      setWsLoading(false)
    }
  }

  const handleSaveWorkspaces = async () => {
    if (!wsAssigningUser?.id) return
    setWsSaving(true)
    try {
      // For each selected workspace, upsert the assignment
      const selectedWsIds = userWorkspaces.map(w => w.id)
      // Get current workspaces for this user
      const currentWsIds = selectedWsIds // Already computed
      await Promise.all(
        allWorkspaces.map(async (ws) => {
          const isSelected = selectedWsIds.includes(ws.id)
          const isCurrentlyAssigned = userWorkspaces.some(uw => uw.id === ws.id)
          if (isSelected && !isCurrentlyAssigned) {
            // Add assignment
            const role = userWorkspaces.find(uw => uw.id === ws.id)?.role || 'viewer'
            await setWorkspaceUser(ws.id, wsAssigningUser.id, role)
          } else if (!isSelected && isCurrentlyAssigned) {
            // Remove assignment
            await removeWorkspaceUser(ws.id, wsAssigningUser.id)
          }
        })
      )
      message.success('工作區指派已更新')
      setWsDrawerOpen(false)
      fetchUsers()
    } catch (e: any) {
      message.error('儲存失敗: ' + (e?.message || ''))
    } finally {
      setWsSaving(false)
    }
  }

  // ── Resource Permissions ──────────────────────────────────
  const openResPermDrawer = async (user: User) => {
    setResPermUser(user)
    setResPermDrawerOpen(true)
    setResPermLoading(true)
    try {
      const [resources, perms] = await Promise.all([
        listResources(),
        getUserResourcePermissions(user.id),
      ])
      setAllResources(Array.isArray(resources) ? resources : [])
      setUserResPerms(Array.isArray(perms) ? perms : [])
    } catch (e) {
      message.error('無法載入資源權限')
    } finally {
      setResPermLoading(false)
    }
  }

  const handleSaveResPerms = async () => {
    if (!resPermUser?.id) return
    setResPermSaving(true)
    try {
      await setUserResourcePermissions(resPermUser.id, userResPerms)
      message.success('資源權限已更新')
      setResPermDrawerOpen(false)
    } catch (e: any) {
      message.error('儲存失敗: ' + (e?.message || ''))
    } finally {
      setResPermSaving(false)
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
      width: 220,
      render: (_, r) => (
        <Space>
          <Button size="small" icon={<LockOutlined />} onClick={() => openResPermDrawer(r)}>資源權限</Button>
          <Button size="small" icon={<TeamOutlined />} onClick={() => openWsDrawer(r)}>指派工作區</Button>
          <Button size="small" icon={<GroupOutlined />} onClick={() => openGroupDrawer(r)}>指派群組</Button>
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

      {/* Group Assignment Drawer */}
      <Drawer
        title={<Space><GroupOutlined /> 指派群組：<Tag color="blue">{assigningUser?.username}</Tag></Space>}
        open={groupDrawerOpen}
        onClose={() => setGroupDrawerOpen(false)}
        width={480}
        extra={
          <Button type="primary" loading={groupSaving} onClick={handleSaveGroups}>
            儲存
          </Button>
        }
      >
        {groupLoading ? (
          <div style={{ textAlign: 'center', padding: 32 }}>載入中...</div>
        ) : (
          <>
            <Alert
              message="勾選使用者所屬的群組。變更會即時更新。"
              type="info"
              style={{ marginBottom: 16 }}
            />
            <List
              dataSource={allGroups}
              renderItem={group => (
                <List.Item
                  key={group.id || group.name}
                  extra={
                    <Checkbox
                      checked={selectedGroupIds.includes(group.name)}
                      onChange={e => {
                        if (e.target.checked) {
                          setSelectedGroupIds(prev => [...prev, group.name])
                        } else {
                          setSelectedGroupIds(prev => prev.filter(id => id !== group.name))
                        }
                      }}
                    />
                  }
                >
                  <List.Item.Meta
                    title={<b style={{ color: 'var(--highlight)' }}>{group.label || group.name}</b>}
                    description={group.description || ''}
                  />
                </List.Item>
              )}
              locale={{ emptyText: '尚無可用群組' }}
            />
          </>
        )}
      </Drawer>

      {/* Workspace Assignment Drawer */}
      <Drawer
        title={<Space><TeamOutlined /> 指派工作區：<Tag color="blue">{wsAssigningUser?.username}</Tag></Space>}
        open={wsDrawerOpen}
        onClose={() => setWsDrawerOpen(false)}
        width={480}
        extra={
          <Button type="primary" loading={wsSaving} onClick={handleSaveWorkspaces}>
            儲存
          </Button>
        }
      >
        {wsLoading ? (
          <div style={{ textAlign: 'center', padding: 32 }}>載入中...</div>
        ) : (
          <>
            <Alert
              message="勾選使用者可存取的工作區。每個工作區可選擇角色（檢視/編輯/管理）。"
              type="info"
              style={{ marginBottom: 16 }}
            />
            <List
              dataSource={allWorkspaces}
              renderItem={ws => {
                const assigned = userWorkspaces.find(uw => uw.id === ws.id)
                return (
                  <List.Item
                    key={ws.id}
                    extra={
                      <Checkbox
                        checked={!!assigned}
                        onChange={e => {
                          if (e.target.checked) {
                            setUserWorkspaces(prev => [...prev, { id: ws.id, name: ws.label || ws.name, role: 'viewer' }])
                          } else {
                            setUserWorkspaces(prev => prev.filter(uw => uw.id !== ws.id))
                          }
                        }}
                      />
                    }
                  >
                    <List.Item.Meta
                      title={<b style={{ color: 'var(--highlight)' }}>{ws.label || ws.name}</b>}
                      description={
                        assigned ? (
                          <Select
                            size="small"
                            value={assigned.role}
                            style={{ width: 120, marginTop: 4 }}
                            onChange={val => setUserWorkspaces(prev => prev.map(uw => uw.id === ws.id ? { ...uw, role: val } : uw))}
                            options={[
                              { value: 'viewer', label: '檢視' },
                              { value: 'editor', label: '編輯' },
                              { value: 'admin', label: '管理' },
                            ]}
                          />
                        ) : <span style={{ color: 'var(--muted)' }}>未指派</span>
                      }
                    />
                  </List.Item>
                )
              }}
              locale={{ emptyText: '尚無可用工作區' }}
            />
          </>
        )}
      </Drawer>

      {/* Resource Permissions Drawer */}
      <Drawer
        title={<Space><LockOutlined /> 資源權限：<Tag color="blue">{resPermUser?.username}</Tag></Space>}
        open={resPermDrawerOpen}
        onClose={() => setResPermDrawerOpen(false)}
        width={560}
        extra={
          <Button type="primary" loading={resPermSaving} onClick={handleSaveResPerms}>
            儲存
          </Button>
        }
      >
        {resPermLoading ? (
          <div style={{ textAlign: 'center', padding: 32 }}>載入中...</div>
        ) : (
          <>
            <Alert
              message="針對特定資源（service/route/upstream/consumer）設定使用者的精細權限。覆寫 workspace-level 預設角色。"
              type="info"
              style={{ marginBottom: 16 }}
            />
            <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
              <thead>
                <tr style={{ background: 'var(--accent)' }}>
                  <th style={{ padding: '8px 12px', textAlign: 'left', color: 'var(--text)' }}>資源</th>
                  <th style={{ padding: '8px 12px', textAlign: 'left', color: 'var(--text)' }}>類型</th>
                  <th style={{ padding: '8px 12px', textAlign: 'center', color: 'var(--text)', width: 80 }}>拒絕</th>
                  <th style={{ padding: '8px 12px', textAlign: 'center', color: 'var(--text)', width: 80 }}>讀取</th>
                  <th style={{ padding: '8px 12px', textAlign: 'center', color: 'var(--text)', width: 80 }}>寫入</th>
                </tr>
              </thead>
              <tbody>
                {allResources.map((r, i) => {
                  const current = userResPerms.find(p => p.resource_id === r.id)
                  return (
                    <tr key={r.id} style={{ background: i % 2 === 0 ? 'var(--secondary)' : 'transparent' }}>
                      <td style={{ padding: '8px 12px' }}>
                        <Space>
                          <LockOutlined style={{ color: 'var(--muted)', fontSize: 11 }} />
                          <span style={{ color: 'var(--highlight)' }}>{r.name}</span>
                        </Space>
                      </td>
                      <td style={{ padding: '8px 12px', color: 'var(--muted)', textTransform: 'capitalize' }}>{r.type || '—'}</td>
                      {(['deny', 'read', 'write'] as const).map(perm => (
                        <td key={perm} style={{ textAlign: 'center', padding: '8px 12px' }}>
                          <Checkbox
                            checked={current?.permission === perm}
                            disabled={resPermSaving}
                            onChange={() => {
                              if (current?.permission === perm) {
                                setUserResPerms(userResPerms.filter(p => p.resource_id !== r.id))
                              } else {
                                const filtered = userResPerms.filter(p => p.resource_id !== r.id)
                                setUserResPerms([...filtered, { resource_id: r.id, permission: perm }])
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
            {userResPerms.length > 0 && (
              <div style={{ marginTop: 12, fontSize: 12, color: 'var(--muted)' }}>
                已設定 {userResPerms.length} 項資源權限覆寫
              </div>
            )}
          </>
        )}
      </Drawer>
    </div>
  )
}
