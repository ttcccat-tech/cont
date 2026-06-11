import { useEffect, useState } from 'react'
import { Table, Button, Space, Tag, message, Modal, Form, Input, Popconfirm, Divider, Drawer, Card, Descriptions, Alert } from 'antd'
import { PlusOutlined, ReloadOutlined, DeleteOutlined, KeyOutlined, LockOutlined, EyeInvisibleOutlined, EyeTwoTone, CopyOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import api, { KongConsumer } from '../api/kong'
import { useAuth } from '../context/AuthContext'

interface JWTCredential {
  id: string
  key: string
  algorithm: string
  rsa_public_key?: string
  secret?: string
  consumer_id?: string
  created_at?: number
}

interface KeyAuthCredential {
  id: string
  key: string
  consumer_id?: string
  created_at?: number
}

export default function ConsumersPage() {
  const [consumers, setConsumers] = useState<KongConsumer[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [form] = Form.useForm()
  const [submitting, setSubmitting] = useState(false)
  const { canWrite, canDelete } = useAuth()

  // Credential drawer
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [selectedConsumer, setSelectedConsumer] = useState<KongConsumer | null>(null)
  const [jwtCreds, setJwtCreds] = useState<JWTCredential[]>([])
  const [keyCreds, setKeyCreds] = useState<KeyAuthCredential[]>([])
  const [credLoading, setCredLoading] = useState(false)
  const [showSecret, setShowSecret] = useState<Record<string, boolean>>({})
  const [showKey, setShowKey] = useState<Record<string, boolean>>({})
  const [showJwtKey, setShowJwtKey] = useState<Record<string, boolean>>({})

  const fetchConsumers = () => {
    setLoading(true)
    api.listConsumers()
      .then(r => setConsumers(r))
      .catch(() => message.error('無法連接 Kong Admin API'))
      .finally(() => setLoading(false))
  }

  useEffect(() => { fetchConsumers() }, [])

  const openCredentials = (consumer: KongConsumer) => {
    setSelectedConsumer(consumer)
    setDrawerOpen(true)
    fetchCredentials(consumer.id!)
  }

  const fetchCredentials = async (consumerId: string) => {
    setCredLoading(true)
    try {
      const [jwtRes, keyRes] = await Promise.all([
        api.listJWTCredentials(consumerId),
        api.listKeyAuthCredentials(consumerId),
      ])
      setJwtCreds(Array.isArray(jwtRes) ? jwtRes : jwtRes?.data || [])
      setKeyCreds(Array.isArray(keyRes) ? keyRes : keyRes?.data || [])
    } catch (e: any) {
      console.error('fetchCredentials error:', e)
      // Show error toast but don't rethrow — prevents whole-page crash
      const errMsg = e?.response?.data?.message || e?.message || '未知錯誤'
      message.error('無法取得憑證：' + errMsg)
    } finally {
      setCredLoading(false)
    }
  }

  const handleDelete = async (id: string) => {
    try {
      await api.deleteConsumer(id)
      message.success('刪除成功')
      fetchConsumers()
    } catch (e: any) {
      const status = e?.response?.status
      if (status === 204 || status === 404) {
        message.success('刪除成功')
        fetchConsumers()
      } else {
        message.error('刪除失敗: ' + (e.message || ''))
      }
    }
  }

  const handleCreate = async () => {
    if (submitting) return
    try {
      const values = await form.validateFields()
      setSubmitting(true)
      await api.createConsumer(values)
      message.success('消費者建立成功')
      setModalOpen(false); fetchConsumers()
    } catch (e: any) {
      if (!e.errorFields) {
        const status = e?.response?.status
        if (status === 409) {
          message.error('此 Username 已存在')
        } else {
          message.error('操作失敗: ' + (e.message || ''))
        }
      }
    } finally { setSubmitting(false) }
  }

  // JWT methods
  const handleCreateJWT = async () => {
    if (!selectedConsumer?.id) return
    try {
      setCredLoading(true)
      // Use analytics-api to generate OpenSSL-format RSA key pair (Kong requires SubjectPublicKeyInfo PEM)
      const { publicKey, privateKey } = await api.generateRSAKeyPair()

      const result = await api.createJWTCredential(selectedConsumer.id, {
        algorithm: 'RS256',
        rsa_public_key: publicKey,
      })
      // Store private key with the credential so we can display it for the user
      const cred = { ...result, key: result.key || result.id, _privateKey: privateKey }
      setJwtCreds(prev => [cred, ...prev])
      message.success('JWT 憑證已建立（RS256）')
    } catch (e: any) {
      message.error('建立失敗: ' + (e.message || ''))
    } finally {
      setCredLoading(false)
    }
  }

  const handleDeleteJWT = async (consumerId: string, credId: string) => {
    try {
      await api.deleteJWTCredential(consumerId, credId)
      message.success('JWT 憑證已刪除')
      fetchCredentials(consumerId)
    } catch (e: any) { message.error('刪除失敗: ' + (e.message || '')) }
  }

  // Key-Auth methods
  const handleCreateKeyAuth = async () => {
    if (!selectedConsumer?.id) return
    try {
      const data = await api.createKeyAuthCredential(selectedConsumer.id, {})
      message.success('API Key 已建立：' + (data as any)?.key)
      fetchCredentials(selectedConsumer.id)
    } catch (e: any) { message.error('建立失敗: ' + (e.message || '')) }
  }

  const handleDeleteKeyAuth = async (consumerId: string, credId: string) => {
    try {
      await api.deleteKeyAuthCredential(consumerId, credId)
      message.success('API Key 已刪除')
      fetchCredentials(consumerId)
    } catch (e: any) { message.error('刪除失敗: ' + (e.message || '')) }
  }

  const copyToClipboard = (text: string, label: string) => {
    navigator.clipboard.writeText(text).then(() => message.success(`${label} 已複製`))
  }

  const jwtColumns: ColumnsType<JWTCredential> = [
    { title: '演算法', dataIndex: 'algorithm', render: v => <Tag color="cyan">{v || 'RS256'}</Tag> },
    { title: 'Private Key (JWK)', dataIndex: 'key', ellipsis: false, render: (v, r) => (
      <Space direction="vertical" size={4} style={{ width: '100%' }}>
        <Input.Password
          value={v}
          readOnly
          size="small"
          style={{ fontFamily: 'monospace', fontSize: 10 }}
          iconRender={(visible) => visible ? <EyeTwoTone /> : <EyeInvisibleOutlined />}
          onChange={(e) => {}}
        />
        <Button
          size="small"
          icon={<CopyOutlined />}
          onClick={() => copyToClipboard(v, 'JWT Private Key')}
          style={{ alignSelf: 'flex-start', marginTop: 4 }}
        >
          複製
        </Button>
      </Space>
    )},
    { title: '建立時間', dataIndex: 'created_at', render: v => v ? new Date(v).toLocaleString('zh-TW') : '-' },
    {
      title: '操作', width: 100,
      render: (_, r) => (
        <Popconfirm title="刪除此 JWT 憑證？" onConfirm={() => selectedConsumer?.id && handleDeleteJWT(selectedConsumer.id, r.id)}>
          <Button size="small" danger icon={<DeleteOutlined />}>刪除</Button>
        </Popconfirm>
      )
    }
  ]

  const keyColumns: ColumnsType<KeyAuthCredential> = [
    {
      title: 'API Key', dataIndex: 'key', render: (v, r) => (
        <Space direction="vertical" size={4}>
          <Input.Password
            value={v}
            readOnly
            size="small"
            style={{ fontFamily: 'monospace', fontSize: 11 }}
            iconRender={(visible) => visible ? <EyeTwoTone /> : <EyeInvisibleOutlined />}
            onChange={(e) => {}}
          />
          <Button
            size="small"
            icon={<CopyOutlined />}
            onClick={() => copyToClipboard(v, 'API Key')}
            style={{ alignSelf: 'flex-start' }}
          >
            複製
          </Button>
        </Space>
      )
    },
    { title: '建立時間', dataIndex: 'created_at', render: v => v ? new Date(v).toLocaleString('zh-TW') : '-' },
    {
      title: '操作', width: 100,
      render: (_, r) => (
        <Popconfirm title="刪除此 API Key？" onConfirm={() => selectedConsumer?.id && handleDeleteKeyAuth(selectedConsumer.id, r.id)}>
          <Button size="small" danger icon={<DeleteOutlined />}>刪除</Button>
        </Popconfirm>
      )
    }
  ]

  return (
    <div>
      <div style={{ display:'flex', justifyContent:'space-between', marginBottom:16 }}>
        <h1>消費者管理</h1>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={fetchConsumers}>刷新</Button>
          {canWrite('consumers') && (
            <Button type="primary" icon={<PlusOutlined />} onClick={() => { form.resetFields(); setModalOpen(true) }}>新增消費者</Button>
          )}
        </Space>
      </div>

      <Table
        columns={[
          { title: 'Username', dataIndex: 'username', key: 'username', render: v => <b style={{color:'var(--highlight)'}}>{v}</b> },
          { title: 'Custom ID', dataIndex: 'custom_id', key: 'custom_id', render: v => v || <span style={{color:'var(--muted)'}}>—</span> },
          { title: 'ID', dataIndex: 'id', key: 'id', ellipsis: true, render: v => <code style={{fontSize:11,color:'var(--muted)'}}>{v}</code> },
          { title: '建立時間', dataIndex: 'created_at', render: v => v ? new Date(v).toLocaleString('zh-TW') : '-' },
          {
            title: '操作', width: 250,
            render: (_, r) => (
              <Space>
                <Button size="small" icon={<LockOutlined />} onClick={() => openCredentials(r)}>憑證管理</Button>
                {canDelete('consumers') && (
                  <Popconfirm title={`確認刪除消費者「${r.username}」？`} onConfirm={() => r.id && handleDelete(r.id)}>
                    <Button size="small" danger icon={<DeleteOutlined />}>刪除</Button>
                  </Popconfirm>
                )}
              </Space>
            )
          }
        ]}
        dataSource={consumers as any}
        rowKey="id"
        loading={loading}
        pagination={{ pageSize: 10 }}
        locale={{ emptyText: '暫無消費者，點擊「新增消費者」開始' }}
      />

      {/* 新增消費者 Modal */}
      <Modal title="新增消費者" open={modalOpen} onOk={handleCreate} confirmLoading={submitting}
        onCancel={() => setModalOpen(false)} okText="建立" width={440}>
        <Form form={form} layout="vertical" style={{ marginTop:16 }}>
          <Form.Item name="username" label="Username" rules={[{ required: true, message: '必填' }]}>
            <Input placeholder="my-app-client" />
          </Form.Item>
          <Form.Item name="custom_id" label="Custom ID（可選）">
            <Input placeholder="12345（第三方系統 ID）" />
          </Form.Item>
        </Form>
      </Modal>

      {/* 憑證管理 Drawer */}
      <Drawer
        title={<Space><LockOutlined /> 憑證管理：<Tag color="red">{selectedConsumer?.username}</Tag></Space>}
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        width={640}
      >
        {selectedConsumer && (
          <div style={{ display:'flex', flexDirection:'column', gap:24 }}>
            {/* JWT Section */}
            <Card
              title={<Space><KeyOutlined /> JWT 憑證</Space>}
              extra={<Button type="primary" size="small" icon={<PlusOutlined />} onClick={handleCreateJWT}>建立 JWT</Button>}
              style={{ background:'var(--secondary)', border:'1px solid var(--accent)' }}
            >
              <Alert
                message="JWT 適用於 API 身份驗證。Kong 會驗證 client 提供的 JWT 簽名。演算法建議使用 RS256（RSA）。"
                type="info"
                style={{ marginBottom: 12, fontSize: 12 }}
              />
              <Table
                columns={jwtColumns}
                dataSource={jwtCreds as any}
                rowKey="id"
                loading={credLoading}
                size="small"
                pagination={false}
                locale={{ emptyText: '尚無 JWT 憑證' }}
              />
            </Card>

            {/* API Key Section */}
            <Card
              title={<Space><KeyOutlined /> API Key</Space>}
              extra={<Button type="primary" size="small" icon={<PlusOutlined />} onClick={handleCreateKeyAuth}>建立 API Key</Button>}
              style={{ background:'var(--secondary)', border:'1px solid var(--accent)' }}
            >
              <Alert
                message="API Key 適用於簡單的 key=VALUE 身份驗證。Key 由系統自動生成。建議用於 Server-to-Server 場景。"
                type="info"
                style={{ marginBottom: 12, fontSize: 12 }}
              />
              <Table
                columns={keyColumns}
                dataSource={keyCreds as any}
                rowKey="id"
                loading={credLoading}
                size="small"
                pagination={false}
                locale={{ emptyText: '尚無 API Key' }}
              />
            </Card>

            {/* Usage Guide */}
            <Card
              title="使用方式"
              style={{ background:'var(--secondary)', border:'1px solid var(--accent)' }}
            >
              <Descriptions column={1} size="small" labelStyle={{ color:'var(--muted)', width:140 }}>
                <Descriptions.Item label="JWT Header">
                  <code>Authorization: Bearer &lt;jwt-token&gt;</code>
                </Descriptions.Item>
                <Descriptions.Item label="API Key Header">
                  <code>apikey: &lt;your-key&gt;</code>
                </Descriptions.Item>
                <Descriptions.Item label="API Key Query">
                  <code>?apikey=&lt;your-key&gt;</code>
                </Descriptions.Item>
              </Descriptions>
            </Card>
          </div>
        )}
      </Drawer>
    </div>
  )
}