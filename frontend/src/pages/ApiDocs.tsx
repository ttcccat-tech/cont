import { useEffect, useState } from 'react'
import SwaggerUI from 'swagger-ui-react'
import { Card, Spin, Result, Button } from 'antd'
import { ReloadOutlined } from '@ant-design/icons'

// swagger-ui-dist CSS loaded via CDN in index.html

export default function ApiDocsPage() {
  const [specUrl, setSpecUrl] = useState('/api/docs.json')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const checkSpec = () => {
    setLoading(true)
    setError(null)
    fetch('/api/docs.json')
      .then(res => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`)
        return res.json()
      })
      .then(() => setLoading(false))
      .catch(err => {
        setError(`無法載入 OpenAPI 規格：${err.message}`)
        setLoading(false)
      })
  }

  useEffect(() => {
    checkSpec()
  }, [])

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
        <h1>API 文件</h1>
        <Button icon={<ReloadOutlined />} onClick={checkSpec}>
          重新整理
        </Button>
      </div>

      <Card
        style={{
          background: 'var(--secondary)',
          border: 'none',
          minHeight: 'calc(100vh - 160px)'
        }}
        styles={{ body: { padding: 0 } }}
      >
        {loading ? (
          <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: 400 }}>
            <Spin size="large" />
          </div>
        ) : error ? (
          <Result
            status="error"
            title="載入失敗"
            subTitle={error}
            extra={
              <Button type="primary" icon={<ReloadOutlined />} onClick={checkSpec}>
                重試
              </Button>
            }
          />
        ) : (
          <SwaggerUI
            url={specUrl}
            docExpansion="list"
            defaultModelsExpandDepth={-1}
            tryItOutEnabled={true}
            persistAuthorization={true}
            configuration={{
              docExpansion: 'list',
              filter: true,
              showExtensions: true,
              showCommonExtensions: true,
            }}
          />
        )}
      </Card>
    </div>
  )
}
