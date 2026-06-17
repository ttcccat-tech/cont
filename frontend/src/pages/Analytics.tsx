import { useEffect, useState, useRef } from 'react'
import { Card, Row, Col, Statistic, Spin, Tag, Select, DatePicker, Progress } from 'antd'
import ReactECharts from 'echarts-for-react'
import { getStatus, getMetrics, getAnalyticsUsage, AnalyticsUsageResponse } from '../api/kong'

const { RangePicker } = DatePicker
const { Option } = Select

interface MetricsPoint {
  time: number // epoch ms
  requests: number
  active: number
  accepted: number
}

// ── localStorage persistence (module scope — avoids ref/init order bug) ──────
const STORAGE_KEY = 'kong_analytics_history'

function loadHistory(): MetricsPoint[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw) as MetricsPoint[]
    const cutoff = Date.now() - 7 * 24 * 60 * 60 * 1000
    return parsed.filter(p => p.time > cutoff)
  } catch { return [] }
}

function saveHistory(points: MetricsPoint[]) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(points))
  } catch { /* quota exceeded or private browsing */ }
}
// ────────────────────────────────────────────────────────────────────────────

export default function Analytics() {
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)
  const [timeRange, setTimeRange] = useState<'1h' | '6h' | '24h' | '7d'>('1h')

  // Current snapshot
  const [metrics, setMetrics] = useState<Record<string, number>>({})
  const [status, setStatus] = useState<any>(null)

  // Cont usage analytics
  const [contUsage, setContUsage] = useState<AnalyticsUsageResponse | null>(null)

  // Historical data — loaded once at init, kept in sync via saveHistory on each poll
  const historyRef = useRef<MetricsPoint[]>(loadHistory())
  const [history, setHistory] = useState<MetricsPoint[]>(historyRef.current)
  const lastReqRef = useRef(0)
  const lastFetchRef = useRef(0)

  const fetchData = async () => {
    try {
      const now = Date.now()
      // Throttle to one fetch per 60s
      if (now - lastFetchRef.current < 58000) {
        setLoading(false)
        return
      }
      lastFetchRef.current = now

      const [st, mi, cu] = await Promise.all([getStatus(), getMetrics(), getAnalyticsUsage()])
      setStatus(st)
      setMetrics(mi)
      setContUsage(cu)
      setError(false)

      const timeStr = Date.now()

      const reqTotal = mi['kong_nginx_requests_total'] || 0
      const reqDelta = lastReqRef.current === 0 ? 0 : reqTotal - lastReqRef.current
      lastReqRef.current = reqTotal

      const active = mi['kong_nginx_connections_total{state="active"}'] ?? st?.server?.connections_active ?? 0
      const accepted = mi['kong_nginx_connections_total{state="accepted"}'] ?? st?.server?.connections_accepted ?? 0

      // Worker memory: only available in /status JSON (workers_lua_vms), not in Prometheus metrics
      const workerMem = st?.memory?.workers_lua_vms
        ? [1260,1261,1262,1263].map(pid => {
            const w = st.memory.workers_lua_vms.find((w: any) => w.pid === pid)
            if (!w) return 0
            const match = String(w.http_allocated_gc).match(/^([\d.]+)\s*(\w+)?/)
            if (!match) return 0
            const val = parseFloat(match[1])
            const unit = match[2] || 'MiB'
            return unit === 'GiB' ? val * 1024 : unit === 'KiB' ? val / 1024 : val
          })
        : [0,0,0,0]

      const dictAlloc = mi['kong_memory_lua_shared_dict_bytes{shared_dict="kong"}'] ?? 0
      const dictCap = mi['kong_memory_lua_shared_dict_total_bytes{shared_dict="kong"}'] ?? 1

      historyRef.current = [
        ...historyRef.current.slice(-59), // keep last 60 points (1h at 60s intervals)
        { time: timeStr, requests: reqDelta, active, accepted }
      ]
      setHistory([...historyRef.current])
      saveHistory(historyRef.current)

    } catch {
      setError(true)
    } finally {
      setLoading(false)
    }
  }

  // Filter history by selected time range (timestamp-based)
  const getFilteredHistory = () => {
    const now = Date.now()
    const ranges: Record<string, number> = {
      '1h': 60 * 60 * 1000,
      '6h': 6 * 60 * 60 * 1000,
      '24h': 24 * 60 * 60 * 1000,
      '7d': 7 * 24 * 60 * 60 * 1000,
    }
    const cutoff = now - (ranges[timeRange] ?? ranges['1h'])
    return historyRef.current.filter(p => p.time >= cutoff)
  }

  useEffect(() => {
    fetchData()
    const interval = setInterval(fetchData, 60000)
    return () => clearInterval(interval)
  }, [])

  if (loading) return <Spin size="large" style={{ display:'flex', justifyContent:'center', marginTop:80 }} />

  if (error) {
    return (
      <div style={{ textAlign:'center', marginTop:80 }}>
        <Tag color="red" style={{fontSize:16}}>無法連線至 Cont Metrics</Tag>
        <p style={{color:'var(--muted)', marginTop:8}}>請確認 Cont Prometheus 插件已啟用</p>
      </div>
    )
  }

  const reqTotal = metrics['kong_nginx_requests_total'] ?? status?.server?.total_requests ?? 0
  const activeConns = metrics['kong_nginx_connections_total{state="active"}'] ?? status?.server?.connections_active ?? 0
  const acceptedConns = metrics['kong_nginx_connections_total{state="accepted"}'] ?? 0

  // Build request trend from history
  const requestTrendOption = {
    backgroundColor: 'transparent',
    title: { text: `流量趨勢（${timeRange}）`, textStyle: { color: '#eaeaea', fontSize: 14 } },
    tooltip: { trigger: 'axis' },
    legend: {
      textStyle: { color: '#8892a0' },
      top: 0,
      data: ['每秒請求差值', '活躍連線']
    },
    grid: { left: '3%', right: '4%', bottom: '3%', containLabel: true },
    xAxis: {
      type: 'category' as const,
      data: getFilteredHistory().map(h => {
        const d = new Date(h.time)
        return `${d.getHours().toString().padStart(2,'0')}:${d.getMinutes().toString().padStart(2,'0')}`
      }),
      axisLine: { lineStyle: { color: '#333' } },
      axisLabel: { color: '#8892a0', fontSize: 10 }
    },
    yAxis: [
      {
        type: 'value' as const,
        name: '請求/s',
        axisLine: { lineStyle: { color: '#333' } },
        axisLabel: { color: '#8892a0' },
        splitLine: { lineStyle: { color: '#222' } }
      },
      {
        type: 'value' as const,
        name: '活躍連線',
        axisLine: { lineStyle: { color: '#333' } },
        axisLabel: { color: '#8892a0' },
        splitLine: { show: false }
      }
    ],
    series: [
      {
        name: '每秒請求差值',
        type: 'line' as const,
        data: getFilteredHistory().map(h => h.requests),
        smooth: true,
        lineStyle: { color: '#4ade80' },
        areaStyle: { color: 'rgba(74,222,128,0.1)' }
      },
      {
        name: '活躍連線',
        type: 'line' as const,
        yAxisIndex: 1,
        data: getFilteredHistory().map(h => h.active),
        smooth: true,
        lineStyle: { color: '#60a5fa' }
      }
    ]
  }

  const workerMemChart = (st: any) => {
    const workerMbs = [1260,1261,1262,1263].map(pid => {
      if (!st?.memory?.workers_lua_vms) return 0
      const w = st.memory.workers_lua_vms.find((w: any) => w.pid === pid)
      if (!w) return 0
      const match = String(w.http_allocated_gc).match(/^([\d.]+)\s*(\w+)?/)
      if (!match) return 0
      const val = parseFloat(match[1])
      const unit = match[2] || 'MiB'
      return unit === 'GiB' ? val * 1024 : unit === 'KiB' ? val / 1024 : val
    })
    return {
      backgroundColor: 'transparent',
      title: { text: 'Worker 記憶體使用（MB）', textStyle: { color: '#eaeaea', fontSize: 14 } },
      tooltip: { trigger: 'axis' as const },
      grid: { left: '3%', right: '4%', bottom: '3%', containLabel: true },
      xAxis: {
        type: 'category' as const,
        data: ['Worker-1\nPID:1260','Worker-2\nPID:1261','Worker-3\nPID:1262','Worker-4\nPID:1263'],
        axisLine: { lineStyle: { color: '#333' } },
        axisLabel: { color: '#8892a0', fontSize: 11 }
      },
      yAxis: {
        type: 'value' as const,
        axisLine: { lineStyle: { color: '#333' } },
        axisLabel: { color: '#8892a0', formatter: '{value} MB' },
        splitLine: { lineStyle: { color: '#222' } }
      },
      series: [{
        type: 'bar' as const,
        data: workerMbs.map((v, i) => ({ value: v.toFixed(1), itemStyle: { color: ['#4ade80','#60a5fa','#facc15','#e879f9'][i] } })),
        barRadius: [4,4,0,0],
        label: { show: true, position: 'top', formatter: '{c} MB', color: '#eaeaea', fontSize: 11 }
      }]
    }
  }

  const connsPieOption = {
    backgroundColor: 'transparent',
    title: { text: '連線狀態分佈', textStyle: { color: '#eaeaea', fontSize: 14 } },
    tooltip: { trigger: 'item' as const },
    legend: { textStyle: { color: '#8892a0' }, bottom: 0 },
    series: [{
      type: 'pie' as const,
      radius: ['40%','70%'],
      label: { color: '#eaeaea' },
      data: [
        { value: metrics['kong_nginx_connections_total{state="active"}'] ?? activeConns, name: '活躍', itemStyle: { color: '#4ade80' } },
        { value: metrics['kong_nginx_connections_total{state="writing"}'] ?? 0, name: '寫入中', itemStyle: { color: '#60a5fa' } },
        { value: metrics['kong_nginx_connections_total{state="waiting"}'] ?? 0, name: '等待中', itemStyle: { color: '#facc15' } },
        { value: metrics['kong_nginx_connections_total{state="reading"}'] ?? 0, name: '讀取中', itemStyle: { color: '#e879f9' } },
      ]
    }]
  }

  const dictChartOption = {
    backgroundColor: 'transparent',
    title: { text: '共用記憶體（Shared Dict）', textStyle: { color: '#eaeaea', fontSize: 14 } },
    tooltip: { trigger: 'axis' as const, formatter: (p: any) => `${p.name}: ${p.value} MB` },
    grid: { left: '3%', right: '4%', bottom: '3%', containLabel: true },
    xAxis: {
      type: 'category' as const,
      data: ['DB Cache', 'Core (kong)'],
      axisLine: { lineStyle: { color: '#333' } },
      axisLabel: { color: '#8892a0', fontSize: 10 }
    },
    yAxis: {
      type: 'value' as const,
      axisLine: { lineStyle: { color: '#333' } },
      axisLabel: { color: '#8892a0', formatter: (v: number) => `${(v/1024/1024).toFixed(0)} MB` },
      splitLine: { lineStyle: { color: '#222' } }
    },
    series: [{
      type: 'bar' as const,
      data: [
        { value: ((metrics['kong_memory_lua_shared_dict_bytes{shared_dict="kong_db_cache"}'] ?? 0)/1024/1024).toFixed(1), itemStyle: { color: '#4ade80' } },
        { value: ((metrics['kong_memory_lua_shared_dict_bytes{shared_dict="kong"}'] ?? 0)/1024/1024).toFixed(1), itemStyle: { color: '#60a5fa' } },
      ],
      barRadius: [4,4,0,0],
      label: { show: true, position: 'top', formatter: '{c} MB', color: '#eaeaea', fontSize: 10 }
    }]
  }

  return (
    <div>
      <div style={{ display:'flex', justifyContent:'space-between', alignItems:'center', marginBottom:24 }}>
        <h1 style={{ color:'var(--text)', fontSize:22 }}>統計報告</h1>
        <Select value={timeRange} onChange={(v) => setTimeRange(v)} style={{ width:140 }}
          dropdownStyle={{ background:'var(--secondary)' }}>
          <Option value="1h">近 1 小時</Option>
          <Option value="6h">近 6 小時</Option>
          <Option value="24h">近 24 小時</Option>
          <Option value="7d">近 7 天</Option>
        </Select>
      </div>

      {/* Current Stats Row */}
      <Row gutter={[16,16]} style={{ marginBottom: 16 }}>
        <Col xs={12} sm={6}>
          <Card style={{ background:'var(--secondary)', border:'none' }}>
            <Statistic title={<span style={{color:'var(--muted)'}}>累計請求</span>} value={reqTotal}
              valueStyle={{color:'#4ade80', fontSize:20}} />
          </Card>
        </Col>
        <Col xs={12} sm={6}>
          <Card style={{ background:'var(--secondary)', border:'none' }}>
            <Statistic title={<span style={{color:'var(--muted)'}}>活躍連線</span>} value={activeConns}
              valueStyle={{color:'#60a5fa', fontSize:20}} />
          </Card>
        </Col>
        <Col xs={12} sm={6}>
          <Card style={{ background:'var(--secondary)', border:'none' }}>
            <Statistic title={<span style={{color:'var(--muted)'}}>每秒請求差值</span>}
              value={getFilteredHistory().at(-1)?.requests ?? 0}
              valueStyle={{color:'#facc15', fontSize:20}} />
          </Card>
        </Col>
        <Col xs={12} sm={6}>
          <Card style={{ background:'var(--secondary)', border:'none' }}>
            <Statistic title={<span style={{color:'var(--muted)'}}>Workers</span>} value={4}
              valueStyle={{color:'#e879f9', fontSize:20}} />
          </Card>
        </Col>
      </Row>

      {/* Cont Usage Analytics Panel */}
      {contUsage && (
        <>
          <Row gutter={[16,16]} style={{ marginBottom: 16 }}>
            <Col xs={12} sm={6}>
              <Card style={{ background:'var(--secondary)', border:'none' }}>
                <Statistic title={<span style={{color:'var(--muted)'}}>本月用量</span>}
                  value={contUsage.monthly_total.toLocaleString()}
                  valueStyle={{color:'#4ade80', fontSize:20}} />
              </Card>
            </Col>
            <Col xs={12} sm={6}>
              <Card style={{ background:'var(--secondary)', border:'none' }}>
                <Statistic title={<span style={{color:'var(--muted)'}}>配額上限</span>}
                  value={contUsage.quota_limit.toLocaleString()}
                  valueStyle={{color:'#60a5fa', fontSize:20}} />
              </Card>
            </Col>
            <Col xs={12} sm={6}>
              <Card style={{ background:'var(--secondary)', border:'none' }}>
                <Statistic title={<span style={{color:'var(--muted)'}}>用量百分比</span>}
                  value={`${contUsage.usage_percent.toFixed(1)}%`}
                  valueStyle={{color: contUsage.usage_percent > 90 ? '#f87171' : contUsage.usage_percent > 70 ? '#facc15' : '#4ade80', fontSize:20}} />
              </Card>
            </Col>
            <Col xs={12} sm={6}>
              <Card style={{ background:'var(--secondary)', border:'none' }}>
                <Statistic title={<span style={{color:'var(--muted)'}}>方案</span>}
                  value={contUsage.plan}
                  valueStyle={{color:'#e879f9', fontSize:20}} />
              </Card>
            </Col>
          </Row>
          <Row gutter={[16,16]} style={{ marginBottom: 16 }}>
            <Col xs={24}>
              <Card style={{ background:'var(--secondary)', border:'none' }}>
                <Progress
                  percent={Math.min(contUsage.usage_percent, 100)}
                  strokeColor={contUsage.usage_percent > 90 ? '#f87171' : contUsage.usage_percent > 70 ? '#facc15' : '#4ade80'}
                  trailColor="rgba(255,255,255,0.1)"
                  format={(p) => `${contUsage.monthly_total.toLocaleString()} / ${contUsage.quota_limit.toLocaleString()}`}
                />
              </Card>
            </Col>
          </Row>
          <Row gutter={[16,16]} style={{ marginBottom: 16 }}>
            <Col xs={24}>
              <Card style={{ background:'var(--secondary)', border:'none' }} title={<span style={{color:'var(--text)'}}>24小時用量趨勢</span>}>
                <ReactECharts option={{
                  backgroundColor: 'transparent',
                  tooltip: { trigger: 'axis' },
                  grid: { left: '3%', right: '4%', bottom: '3%', containLabel: true },
                  xAxis: {
                    type: 'category' as const,
                    data: contUsage.hourly_trend.map(h => h.hour),
                    axisLine: { lineStyle: { color: '#333' } },
                    axisLabel: { color: '#8892a0', fontSize: 10, rotate: 45 }
                  },
                  yAxis: {
                    type: 'value' as const,
                    name: '請求次數',
                    axisLine: { lineStyle: { color: '#333' } },
                    axisLabel: { color: '#8892a0' },
                    splitLine: { lineStyle: { color: '#222' } }
                  },
                  series: [{
                    type: 'bar' as const,
                    data: contUsage.hourly_trend.map(h => h.count),
                    itemStyle: { color: '#4ade80' },
                    barRadius: [4, 4, 0, 0],
                    label: { show: false }
                  }]
                }} style={{ height: 200 }} />
              </Card>
            </Col>
          </Row>
          <Row gutter={[16,16]} style={{ marginBottom: 16 }}>
            <Col xs={24} lg={12}>
              <Card style={{ background:'var(--secondary)', border:'none' }} title={<span style={{color:'var(--text)'}}>Top Routes</span>}>
                {contUsage.top_routes.length > 0 ? (
                  contUsage.top_routes.map((r, i) => (
                    <div key={r.route_id} style={{ display:'flex', justifyContent:'space-between', color:'var(--text)', padding:'4px 0', borderBottom:'1px solid rgba(255,255,255,0.05)' }}>
                      <span style={{color:'var(--muted)'}}>#{i+1} {r.route_id}</span>
                      <span style={{color:'#4ade80'}}>{r.count.toLocaleString()}</span>
                    </div>
                  ))
                ) : <span style={{color:'var(--muted)'}}>暫無資料</span>}
              </Card>
            </Col>
            <Col xs={24} lg={12}>
              <Card style={{ background:'var(--secondary)', border:'none' }} title={<span style={{color:'var(--text)'}}>Top Consumers</span>}>
                {contUsage.top_consumers.length > 0 ? (
                  contUsage.top_consumers.map((c, i) => (
                    <div key={c.consumer_id} style={{ display:'flex', justifyContent:'space-between', color:'var(--text)', padding:'4px 0', borderBottom:'1px solid rgba(255,255,255,0.05)' }}>
                      <span style={{color:'var(--muted)'}}>#{i+1} {c.consumer_id}</span>
                      <span style={{color:'#60a5fa'}}>{c.count.toLocaleString()}</span>
                    </div>
                  ))
                ) : <span style={{color:'var(--muted)'}}>暫無資料</span>}
              </Card>
            </Col>
          </Row>
        </>
      )}

      {/* Charts Row 1 */}
      <Row gutter={[16,16]}>
        <Col xs={24} lg={16}>
          <Card style={{ background:'var(--secondary)', border:'none' }}>
            <ReactECharts option={requestTrendOption} style={{ height:300 }} />
          </Card>
        </Col>
        <Col xs={24} lg={8}>
          <Card style={{ background:'var(--secondary)', border:'none' }}>
            <ReactECharts option={connsPieOption} style={{ height:300 }} />
          </Card>
        </Col>
      </Row>

      {/* Charts Row 2 */}
      <Row gutter={[16,16]} style={{ marginTop:16 }}>
        <Col xs={24} lg={12}>
          <Card style={{ background:'var(--secondary)', border:'none' }}>
            <ReactECharts option={workerMemChart(status)} style={{ height:250 }} />
          </Card>
        </Col>
        <Col xs={24} lg={12}>
          <Card style={{ background:'var(--secondary)', border:'none' }}>
            <ReactECharts option={dictChartOption} style={{ height:250 }} />
          </Card>
        </Col>
      </Row>
    </div>
  )
}