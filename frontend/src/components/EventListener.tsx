import { useEffect, useRef, useState } from 'react'
import { message } from 'antd'
import { getToken } from '../api/kong'

interface SSEEvent {
  type: string
  data: unknown
}

export function EventListener() {
  const esRef = useRef<EventSource | null>(null)
  const [connected, setConnected] = useState(false)

  useEffect(() => {
    const token = getToken()
    if (!token) return

    // Connect to SSE stream via nginx proxy
    const es = new EventSource(`/api/auth/events`)

    es.onopen = () => setConnected(true)

    es.onerror = () => {
      setConnected(false)
      // Reconnect after 5s
      setTimeout(() => {
        es.close()
        esRef.current = null
      }, 5000)
    }

    // Handle api_key_approved events
    es.addEventListener('api_key_approved', (e: MessageEvent) => {
      try {
        const data = JSON.parse(e.data)
        message.success({
          content: `🔑 API Key 核准成功：${data.key_name || 'API Key'} — 鑰匙已生成`,
          duration: 6,
        })
      } catch {}
    })

    // Handle api_key_rejected events
    es.addEventListener('api_key_rejected', (e: MessageEvent) => {
      try {
        const data = JSON.parse(e.data)
        message.warning({
          content: `❌ API Key 已拒絕：${data.key_name || 'API Key'}`,
          duration: 5,
        })
      } catch {}
    })

    // Handle subscription_updated events
    es.addEventListener('subscription_updated', (e: MessageEvent) => {
      try {
        const data = JSON.parse(e.data)
        message.info({
          content: `📋 訂閱更新：${data.plan_name || 'plan'}`,
          duration: 5,
        })
      } catch {}
    })

    // Handle user_invited events
    es.addEventListener('user_invited', (e: MessageEvent) => {
      try {
        const data = JSON.parse(e.data)
        message.info({
          content: `👤 您已被邀請加入工作區`,
          duration: 5,
        })
      } catch {}
    })

    // Handle heartbeat (ignore)
    es.addEventListener('heartbeat', () => {})

    // Handle connected confirmation
    es.addEventListener('connected', () => setConnected(true))

    esRef.current = es

    return () => {
      es.close()
      esRef.current = null
    }
  }, [])

  return null // invisible component
}
