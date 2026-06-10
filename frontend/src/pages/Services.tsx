import { useEffect, useState } from 'react'
import { Table, Button, Space, Tag, message, Modal, Form, Input, InputNumber, Popconfirm, Switch } from 'antd'
import { PlusOutlined, ReloadOutlined, DeleteOutlined, EditOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import api, { KongService } from '../api/kong'
import { useAuth } from '../context/AuthContext'

const API = import.meta.env.VITE_API_BASE || 'http://localhost:8001'

export default function Services() {
  const [services, setServices] = useState<KongService[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<KongService | null>(null)
  const [form] = Form.useForm()
  const [submitting, setSubmitting] = useState(false)
  const { canWrite, canDelete } = useAuth()

  const fetchServices = () => {
    setLoading(true)
    api.listServices()
      .then(data => setServices(data.data || []))
      .catch(() => message.error('無法連接 Kong Admin API'))
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    fetchServices()
  }, [])

  const openCreate = () => { setEditing(null); form.resetFields(); setModalOpen(true) }
  const openEdit = (record: KongService) => {
    setEditing(record)
    form.setFieldsValue({
      name: record.name,
      protocol: record.protocol || 'http',
      host: record.host || '',
      port: record.port || 80,
      path: record.path || '',
      retries: record.retries ?? 5,
      connect_timeout: record.connect_timeout ?? 60000,
      read_timeout: record.read_timeout ?? 60000,
      write_timeout: record.write_timeout ?? 60000,
    })
    setModalOpen(true)
  }

  const handleDelete = async (id: string) => {
    try {
      await api.deleteService(id)
      message.success('刪除成功')
      fetchServices()
    } catch (err: any) {
      message.error('刪除失敗: ' + (err.message || ''))
    }
  }

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields()
      setSubmitting(true)
      const payload: any = {
        name: values.name,
        url: `${values.protocol}://${values.host}:${values.port}${values.path || ''}`,
        retries: values.retries ?? 5,
        connect_timeout: values.connect_timeout ?? 60000,
        read_timeout: values.read_timeout ?? 60000,
        write_timeout: values.write_timeout ?? 60000,
      }
      if (values.health_url) payload.health_url = values.health_url
      if (values.doc_url) payload.doc_url = values.doc_url
      if (editing?.id) {
        await api.updateService(editing.id, payload)
        message.success('更新成功')
      } else {
        await api.createService(payload)
        message.success('建立成功')
      }
      setModalOpen(false)
      fetchServices()
    } catch (e: any) {
      if (e.errorFields) return
      message.error('操作失敗: ' + (e.message || '未知錯誤'))
    } finally {
      setSubmitting(false)
    }
  }

  const splitUrl = (url: string) => {
    if (!url) return { host: '', port: 80, path: '', protocol: 'http' }
    const m = url.match(/^(https?):\/\/([^:]+):?(\d*)(.*)$/)
    return m ? { protocol: m[1], host: m[2], port: parseInt(m[3]) || 80, path: m[4] } : { protocol: 'http', host: url, port: 80, path: '', protocol2: 'http' }
  }

  const columns: ColumnsType<KongService> = [
    { title: '名稱', dataIndex: 'name', key: 'name', render: v => <Tag color="blue">{v}</Tag> },
    {
      title: '目標 URL',
      key: 'url',
      ellipsis: true,
      render: (_, r) => <code style={{ fontSize: 12 }}>{r.protocol}://{r.host}:{r.port}</code>
    },
    { title: '重試次數', dataIndex: 'retries', key: 'retries' },
    {
      title: '超時 (ms)',
      key: 'timeout',
      render: (_, r) => `${r.connect_timeout}/${r.read_timeout}/${r.write_timeout}`
    },
    {
      title: '操作',
      key: 'action',
      width: 160,
      render: (_, record) => (
        <Space>
          {canWrite('services') && (
            <Button size="small" icon={<EditOutlined />} onClick={() => openEdit(record)}>編輯</Button>
          )}
          {canDelete('services') && (
            <Popconfirm title="確認刪除？" onConfirm={() => record.id && handleDelete(record.id)}>
              <Button size="small" danger icon={<DeleteOutlined />}>刪除</Button>
            </Popconfirm>
          )}
        </Space>
      )
    }
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <h1>服務管理</h1>
        <Space>
          <Button icon={<ReloadOutlined />} onClick={fetchServices}>刷新</Button>
          {canWrite('services') && (
            <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>新增服務</Button>
          )}
        </Space>
      </div>

      <Table
        columns={columns}
        dataSource={services  as any}
        rowKey="id"
        loading={loading}
        pagination={{ pageSize: 10 }}
        locale={{ emptyText: '暫無服務，點擊「新增服務」建立第一個' }}
      />

      <Modal
        title={editing ? '編輯服務' : '新增服務'}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        width={560}
        footer={null}
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }} onFinish={handleSubmit}>
          <Form.Item name="name" label="服務名稱" rules={[{ required: true, message: '必填' }]}>
            <Input placeholder="my-api-service" />
          </Form.Item>
          <Form.Item name="protocol" label="協議" rules={[{ required: true }]} initialValue="http">
            <Input placeholder="http" />
          </Form.Item>
          <Form.Item name="host" label="目標主機" rules={[{ required: true, message: '必填' }]} tooltip="例: httpbin.org">
            <Input placeholder="httpbin.org" />
          </Form.Item>
          <Form.Item name="port" label="端口" rules={[{ required: true }]} initialValue={80}>
            <InputNumber min={1} max={65535} style={{ width: 120 }} />
          </Form.Item>
          <Form.Item name="path" label="路徑前綴" tooltip="可選，例: /api">
            <Input placeholder="/api" />
          </Form.Item>
          <Form.Item name="retries" label="重試次數" initialValue={5}>
            <InputNumber min={0} max={100} style={{ width: 120 }} />
          </Form.Item>
          <Form.Item name="connect_timeout" label="連接超時 (ms)" initialValue={60000}>
            <InputNumber min={100} max={600000} style={{ width: 140 }} />
          </Form.Item>
          <Form.Item name="read_timeout" label="讀取超時 (ms)" initialValue={60000}>
            <InputNumber min={100} max={600000} style={{ width: 140 }} />
          </Form.Item>
          <Form.Item name="write_timeout" label="寫入超時 (ms)" initialValue={60000}>
            <InputNumber min={100} max={600000} style={{ width: 140 }} />
          </Form.Item>
          <Form.Item name="health_url" label="健康檢查 URL" tooltip="例: https://api.example.com/health">
            <Input placeholder="https://api.example.com/health" />
          </Form.Item>
          <Form.Item name="doc_url" label="API 文件 URL">
            <Input placeholder="https://api.example.com/docs" />
          </Form.Item>
          <Form.Item style={{ marginBottom: 0, marginTop: 8 }}>
            <Space style={{ display: 'flex', justifyContent: 'flex-end' }}>
              <Button onClick={() => setModalOpen(false)}>取消</Button>
              <Button type="primary" htmlType="submit" loading={submitting}>
                {editing ? '更新' : '建立'}
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
