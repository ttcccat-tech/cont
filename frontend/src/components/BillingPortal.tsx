import { useEffect, useState } from 'react'
import { Card, Row, Col, Tag, Button, Space, Modal, Radio, message, Spin, Divider, Alert, Progress } from 'antd'
import { CreditCardOutlined, TeamOutlined, AppstoreOutlined, DollarOutlined } from '@ant-design/icons'
import { getPlans, getSubscription, createCheckoutSession, createPortalSession, getUsage, Plan, Subscription } from '../api/kong'

function formatPrice(cents: number): string {
  if (cents === 0) return '免費'
  return `$${(cents / 100).toFixed(2)}/月`
}

function parseFeatures(json: string): string[] {
  try { return JSON.parse(json) } catch { return [] }
}

function PlanCard({ plan, currentPlan, billingCycle, onSubscribe }: {
  plan: Plan
  currentPlan: string
  billingCycle: string
  onSubscribe: (planName: string, cycle: string) => void
}) {
  const isCurrent = plan.name === currentPlan
  const price = billingCycle === 'yearly' ? plan.price_yearly : plan.price_monthly
  const period = billingCycle === 'yearly' ? '年' : '月'
  const features = parseFeatures(plan.features)

  return (
    <Card
      style={{
        background: isCurrent ? 'var(--accent)' : 'var(--secondary)',
        border: isCurrent ? '2px solid var(--highlight)' : '1px solid var(--border)',
        borderRadius: 12,
        height: '100%',
      }}
      styles={{ body: { height: '100%', display: 'flex', flexDirection: 'column' } }}
    >
      <div style={{ flex: 1 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
          <span style={{ fontSize: 18, fontWeight: 700 }}>{plan.display_name}</span>
          {isCurrent && <Tag color="gold">目前方案</Tag>}
        </div>
        <div style={{ marginBottom: 16 }}>
          <span style={{ fontSize: 28, fontWeight: 800, color: 'var(--highlight)' }}>
            {price === 0 ? '免費' : `$${(price / 100).toFixed(0)}`}
          </span>
          {price > 0 && <span style={{ color: 'var(--muted)' }}>/{period}</span>}
        </div>
        <Divider style={{ margin: '12px 0' }} />
        <ul style={{ paddingLeft: 16, margin: 0 }}>
          {features.map((f, i) => (
            <li key={i} style={{ marginBottom: 6, fontSize: 13 }}>{f}</li>
          ))}
        </ul>
      </div>
      <Button
        type={isCurrent ? 'default' : 'primary'}
        disabled={isCurrent}
        onClick={() => onSubscribe(plan.name, billingCycle)}
        style={{ marginTop: 16, width: '100%' }}
      >
        {isCurrent ? '目前方案' : price === 0 ? '降級至免費' : '立即升級'}
      </Button>
    </Card>
  )
}

export default function BillingPortal() {
  const [plans, setPlans] = useState<Plan[]>([])
  const [subscription, setSubscription] = useState<Subscription | null>(null)
  const [loading, setLoading] = useState(true)
  const [billingCycle, setBillingCycle] = useState<'monthly' | 'yearly'>('monthly')
  const [checkoutLoading, setCheckoutLoading] = useState(false)
  const [portalLoading, setPortalLoading] = useState(false)
  const [usage, setUsage] = useState<{used: number; limit: number; percent_used: number; reset_at: string} | null>(null)

  const load = async () => {
    setLoading(true)
    try {
      const [p, s, u] = await Promise.all([getPlans(), getSubscription(), getUsage()])
      setPlans(p)
      setSubscription(s)
      setUsage(u)
    } catch (e: unknown) {
      const err = e as { response?: { data?: { error?: string } } }
      message.error(err?.response?.data?.error || '載入方案失敗')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [])

  // Handle return from Stripe checkout
  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    const sessionId = params.get('session_id')
    const billing = params.get('billing')
    if (sessionId && billing === 'success') {
      message.success('訂閱成功！')
      window.history.replaceState({}, '', '/settings')
      load()
    } else if (billing === 'cancelled') {
      message.info('已取消訂閱')
      window.history.replaceState({}, '', '/settings')
    }
  }, [])

  const handleSubscribe = async (planName: string, cycle: string) => {
    if (planName === 'free') {
      message.info('如需降級，請聯繫支援')
      return
    }
    setCheckoutLoading(true)
    try {
      const { url } = await createCheckoutSession(planName, cycle)
      window.location.href = url
    } catch (e: unknown) {
      const err = e as { response?: { data?: { error?: string } } }
      message.error(err?.response?.data?.error || '啟動結帳失敗')
      setCheckoutLoading(false)
    }
  }

  const handlePortal = async () => {
    setPortalLoading(true)
    try {
      const { url } = await createPortalSession()
      window.location.href = url
    } catch (e: unknown) {
      const err = e as { response?: { data?: { error?: string } } }
      message.error(err?.response?.data?.error || '開啟 billing portal 失敗')
      setPortalLoading(false)
    }
  }

  if (loading) return <div style={{ textAlign: 'center', padding: 48 }}><Spin size="large" /></div>

  const currentPlan = subscription?.plan_name || 'free'
  const subStatus = subscription?.status || 'active'

  const statusTag = {
    active: <Tag color="green">已訂閱</Tag>,
    canceled: <Tag color="red">已取消</Tag>,
    past_due: <Tag color="orange">逾期</Tag>,
    trialing: <Tag color="blue">試用中</Tag>,
    incomplete: <Tag color="orange">未完成</Tag>,
  }[subStatus] || <Tag>{subStatus}</Tag>

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
        <div>
          <h2 style={{ margin: 0 }}>方案與計費</h2>
          <span style={{ color: 'var(--muted)', fontSize: 13 }}>
            目前方案：<strong>{plans.find(p => p.name === currentPlan)?.display_name || currentPlan}</strong>
            {' '}({subscription?.billing_cycle || 'monthly'}) {statusTag}
          </span>
        </div>
        <Space>
          <Radio.Group value={billingCycle} onChange={e => setBillingCycle(e.target.value)}>
            <Radio.Button value="monthly">月付</Radio.Button>
            <Radio.Button value="yearly">年付（省 ~17%）</Radio.Button>
          </Radio.Group>
          {subscription?.stripe_customer_id && (
            <Button icon={<CreditCardOutlined />} onClick={handlePortal} loading={portalLoading}>
              管理計費
            </Button>
          )}
        </Space>
      </div>

      {subscription?.cancel_at_period_end && (
        <Alert
          message="您的訂閱將在本期結束後取消"
          type="warning"
          showIcon
          style={{ marginBottom: 16 }}
        />
      )}

      {usage && usage.limit > 0 && (
        <Card style={{ marginBottom: 24, background: 'var(--secondary)', border: 'none' }}>
          <div style={{ marginBottom: 8 }}>
            <Space style={{ float: 'right' }}>
              <span style={{ color: 'var(--muted)', fontSize: 12 }}>
                {new Date(usage.reset_at).toLocaleDateString('zh-TW')} 重置
              </span>
            </Space>
            <span style={{ fontWeight: 600 }}>本月 API 使用量</span>
          </div>
          <Progress
            percent={Math.min(usage.percent_used, 100)}
            format={() => `${usage.used.toLocaleString()} / ${usage.limit.toLocaleString()}`}
            strokeColor={usage.percent_used >= 90 ? '#ff4d4f' : usage.percent_used >= 75 ? '#faad14' : '#1677ff'}
          />
          {usage.percent_used >= 80 && (
            <Alert
              message={`您已使用本月配額的 ${usage.percent_used.toFixed(1)}%，建議升級以避免限流`}
              type="warning"
              showIcon
              style={{ marginTop: 8 }}
            />
          )}
        </Card>
      )}

      <Row gutter={[16, 16]}>
        {plans.map(plan => (
          <Col xs={24} md={8} key={plan.id}>
            <PlanCard
              plan={plan}
              currentPlan={currentPlan}
              billingCycle={billingCycle}
              onSubscribe={handleSubscribe}
            />
          </Col>
        ))}
      </Row>

      {subscription && (
        <Card style={{ marginTop: 24, background: 'var(--secondary)', border: 'none' }}>
          <Row gutter={[24, 16]}>
            <Col xs={24} md={8}>
              <Space direction="vertical" size={4}>
                <span style={{ color: 'var(--muted)', fontSize: 12 }}><AppstoreOutlined /> 工作區上限</span>
                <span style={{ fontSize: 16, fontWeight: 600 }}>
                  {subscription.plan_name === 'free'
                    ? `${plans.find(p => p.name === 'free')?.workspace_limit ?? 3} 個`
                    : '無限制'}
                </span>
              </Space>
            </Col>
            <Col xs={24} md={8}>
              <Space direction="vertical" size={4}>
                <span style={{ color: 'var(--muted)', fontSize: 12 }}><TeamOutlined /> 使用者上限</span>
                <span style={{ fontSize: 16, fontWeight: 600 }}>
                  {subscription.plan_name === 'free'
                    ? `${plans.find(p => p.name === 'free')?.user_limit ?? 5} 人`
                    : subscription.plan_name === 'pro'
                    ? `${plans.find(p => p.name === 'pro')?.user_limit ?? 25} 人`
                    : '無限制'}
                </span>
              </Space>
            </Col>
            <Col xs={24} md={8}>
              <Space direction="vertical" size={4}>
                <span style={{ color: 'var(--muted)', fontSize: 12 }}><DollarOutlined /> API 請求上限</span>
                <span style={{ fontSize: 16, fontWeight: 600 }}>
                  {subscription.plan_name === 'free'
                    ? `${(plans.find(p => p.name === 'free')?.request_limit ?? 1000).toLocaleString()} /月`
                    : '無限制'}
                </span>
              </Space>
            </Col>
          </Row>
        </Card>
      )}
    </div>
  )
}
