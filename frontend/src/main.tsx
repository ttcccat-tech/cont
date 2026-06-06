import React from 'react'
import ReactDOM from 'react-dom/client'
import { ConfigProvider, theme } from 'antd'
import zhTW from 'antd/locale/zh_TW'
import App from './App'
import './index.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ConfigProvider
      locale={zhTW}
      theme={{
        algorithm: theme.darkAlgorithm,
        token: {
          colorPrimary: '#e94560',
          colorBgBase: '#1a1a2e',
          colorTextBase: '#eaeaea',
          borderRadius: 6,
          fontFamily: "'Noto Sans TC', 'PingFang TC', 'Microsoft JhengHei', -apple-system, BlinkMacSystemFont, sans-serif",
        },
        components: {
          Table: { colorBgContainer: '#16213e' },
          Modal: { colorBgElevated: '#16213e' },
          Select: { colorBgElevated: '#16213e' },
          Dropdown: { colorBgElevated: '#16213e' },
          Popover: { colorBgElevated: '#16213e' },
        },
      }}
    >
      <App />
    </ConfigProvider>
  </React.StrictMode>
)