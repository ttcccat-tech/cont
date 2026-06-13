import { useEffect, useState } from 'react'
import { Table, Tag, Space, Button, Select, Card, Spin, message, DatePicker, Row, Col } from 'antd'
import { ReloadOutlined, FilterOutlined, DownloadOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { getAuditLogs, exportAuditLogsCSV } from '../api/kong'
import dayjs, { Dayjs } from 'dayjs'

const { Option } = Select
const { RangePicker } = DatePicker

// Local view model — maps API fields to page display fields
interface AuditView {
  id: number
  timestamp: string
  action: 'CREATE' | 'UPDATE' | 'DELETE'
  resource: string
  target: string
  detail: string
  user: string
}

interface AuditFilters {
  audit_type?: string
  target_type?: string
  actor?: string
  start_time?: string
  end_time?: string
}

export default function AuditLogPage() {
  const [entries, setEntries] = useState<AuditView[]>([])
  const [filtered, setFiltered] = useState<AuditView[]>([])
  const [filterAction, setFilterAction] = useState<string>('all')
  const [filterResource, setFilterResource] = useState<string>('all')
  const [filterActor, setFilterActor] = useState<string>('')
  const [dateRange, setDateRange] = useState<[Dayjs | null, Dayjs | null] | null>(null)
  const [loading, setLoading] = useState(false)
  const [total, setTotal] = useState(0)

  const buildParams = (): Record<string, string> => {
    const p: Record<string, string> = {}
    if (filterAction !== 'all') p.audit_type = filterAction
    if (filterResource !== 'all') p.target_type = filterResource
    if (filterActor.trim()) p.actor = filterActor.trim()
    if (dateRange && dateRange[0] && dateRange[1]) {
      p.start_time = dateRange[0].startOf('day').toISOString()
      p.end_time = dateRange[1].endOf('day').toISOString()
    }
    return p
  }

  const load = async () => {
    setLoading(true)
    try {
      const params = buildParams()
      const result = await getAuditLogs(params)
      const rows = result?.data ?? []
      const totalCount = result?.total ?? rows.length
      setTotal(totalCount)
      const mapped: AuditView[] = rows.map(r => ({
        id: r.id,
        timestamp: r.created_at,
        action: r.audit_type as 'CREATE' | 'UPDATE' | 'DELETE',
        resource: r.target_type,
        target: r.target_id || r.description,
        detail: r.description,
        user: r.actor_username || r.actor_user_id || 'system',
      }))
      setEntries(mapped)
      setFiltered(mapped)
    } catch (err) {
      message.error('載入審計日誌失敗')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [])

  // Client-side secondary filter (audit_type / target_type already applied server-side)
  useEffect(() => {
    let result = entries
    if (filterAction !== 'all') result = result.filter(e => e.action === filterAction)
    if (filterResource !== 'all') result = result.filter(e => e.resource === filterResource)
    setFiltered(result)
  }, [filterAction, filterResource, entries])

  const handleExport = () => {
    const params = buildParams()
    exportAuditLogsCSV(params)
  }

  const actionColor = (a: string) => {
    if (a === 'CREATE') return 'green'
    if (a === 'UPDATE') return 'blue'
    return 'red'
  }

  const resourceColor = (r: string) => {
    const map: Record<string, string> = {
      User: 'orange', Group: 'purple', Workspace: 'cyan',
      Service: 'blue', Route: 'geekblue', Plugin: 'magenta',
      Consumer: 'green', Credential: 'gold',
    }
    return map[r] || 'default'
  }

  const columns: ColumnsType<AuditView> = [
    {
      title: '時間',
      dataIndex: 'timestamp',
      key: 'timestamp',
      width: 180,
      render: v => (
        <span style={{ fontFamily: 'monospace', fontSize: 12, color: 'var(--muted)' }}>
          {new Date(v).toLocaleString('zh-TW', { hour12: false })}
        </span>
      ),
      sorter: (a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime(),
    },
    {
      title: '操作',
      dataIndex: 'action',
      key: 'action',
      width: 100,
      render: v => <Tag color={actionColor(v)}>{v}</Tag>
    },
    {
      title: '資源',
      dataIndex: 'resource',
      key: 'resource',
      width: 120,
      render: v => <Tag color={resourceColor(v)}>{v}</Tag>
    },
    {
      title: '目標',
      dataIndex: 'target',
      key: 'target',
      ellipsis: true,
      render: v => <b style={{color:'var(--text)'}}>{v}</b>
    },
    {
      title: '詳情',
      dataIndex: 'detail',
      key: 'detail',
      render: v => <span style={{color:'var(--muted)', fontSize:12}}>{v || '—'}</span>
    },
    {
      title: '操作者',
      dataIndex: 'user',
      key: 'user',
      width: 120,
      render: v => <Tag>{v}</Tag>
    }
  ]

  return (
    <div>
      <div style={{ display:'flex', justifyContent:'space-between', marginBottom:16 }}>
        <h1>審計日誌</h1>
        <Space>
          <Button icon={<DownloadOutlined />} onClick={handleExport}>匯出 CSV</Button>
          <Button icon={<ReloadOutlined />} onClick={load} loading={loading}>刷新</Button>
        </Space>
      </div>

      {/* Filters */}
      <Card style={{ background:'var(--secondary)', border:'none', marginBottom:16, padding:'12px 16px' }}>
        <Row gutter={[12, 12]} align="middle">
          <Col>
            <span style={{color:'var(--muted)', fontSize:13}}><FilterOutlined /> 篩選：</span>
          </Col>
          <Col>
            <RangePicker
              value={dateRange}
              onChange={(dates) => setDateRange(dates as [Dayjs | null, Dayjs | null] | null)}
              style={{ width: 280 }}
              placeholder={['起始日期', '結束日期']}
            />
          </Col>
          <Col>
            <Select value={filterAction} onChange={v => { setFilterAction(v); load() }} style={{width:120}}
              dropdownStyle={{ background:'var(--secondary)' }}>
              <Option value="all">全部操作</Option>
              <Option value="CREATE">新增</Option>
              <Option value="UPDATE">更新</Option>
              <Option value="DELETE">刪除</Option>
            </Select>
          </Col>
          <Col>
            <Select value={filterResource} onChange={v => { setFilterResource(v); load() }} style={{width:140}}
              dropdownStyle={{ background:'var(--secondary)' }}>
              <Option value="all">全部資源</Option>
              <Option value="User">使用者</Option>
              <Option value="Group">群組</Option>
              <Option value="Workspace">工作區</Option>
              <Option value="Service">服務</Option>
              <Option value="Route">路由</Option>
              <Option value="Plugin">插件</Option>
              <Option value="Consumer">消費者</Option>
              <Option value="Credential">憑證</Option>
            </Select>
          </Col>
          <Col>
            <DatePicker
              value={null}
              picker="week"
              placeholder="週"
              style={{display:'none'}}
            />
          </Col>
          <Col>
            <Space>
              <input
                placeholder="操作者"
                value={filterActor}
                onChange={e => setFilterActor(e.target.value)}
                onKeyDown={e => e.key === 'Enter' && load()}
                style={{
                  background:'var(--primary)', border:'1px solid var(--accent)',
                  color:'var(--text)', borderRadius:6, padding:'4px 10px', width:120, fontSize:13
                }}
              />
              <Button size="small" onClick={load} style={{background:'var(--secondary)', color:'var(--text)'}}>搜尋</Button>
            </Space>
          </Col>
          <Col>
            <span style={{color:'var(--muted)', fontSize:12}}>共 {total} 筆（顯示 {filtered.length} 筆）</span>
          </Col>
        </Row>
      </Card>

      <Spin spinning={loading}>
        <Table
          columns={columns}
          dataSource={filtered as any}
          rowKey="id"
          pagination={{ pageSize: 15, showSizeChanger: true, total }}
          locale={{ emptyText: '尚無操作記錄' }}
          size="small"
          onChange={(pagination) => {
            // Simple reload on page change
            if (pagination.current) load()
          }}
        />
      </Spin>

      <div style={{ marginTop: 16, fontSize: 12, color: 'var(--muted)' }}>
        ※ 審計日誌由 analytics-api 統一記錄，記錄使用者、群組、工作區的新增/更新/刪除操作。匯出包含目前篩選條件下的所有記錄（最多 10,000 筆）。
      </div>
    </div>
  )
}