import { useCallback, useEffect, useMemo, useRef, useState, type KeyboardEvent } from 'react'
import ReactMarkdown from 'react-markdown'
import './App.css'
import { EventsOn } from '../wailsjs/runtime/runtime'
import { HideLauncher, SendChat } from '../wailsjs/go/main/App'

const focusInputEventName = 'launcher:focus-input'

type ChatError = {
  status_code?: number
  code?: string
  message?: string
}

type ChatResult = {
  response?: string
  error?: ChatError
}

function mapErrorToMessage(err?: ChatError): string {
  if (!err) {
    return '请求失败，请稍后重试。'
  }

  if (err.code === 'config_error') {
    return '未配置后端地址，请设置 DESKTOP_CHAT_BASE_URL。'
  }

  if (err.code === 'network_error') {
    return '无法连接后端服务，请确认服务已启动且地址可访问。'
  }

  switch (err.status_code) {
    case 400:
      return '请求格式异常，请重试。'
    case 422:
      return '输入不合法，请修改后重试。'
    case 502:
      return '模型服务暂不可用，请稍后重试。'
    case 500:
      return '服务内部异常，请稍后重试。'
    default:
      return err.message || '请求失败，请稍后重试。'
  }
}

function App() {
  const inputRef = useRef<HTMLInputElement>(null)
  const [query, setQuery] = useState('')
  const [isSearchMode, setIsSearchMode] = useState(false)
  const [isLoading, setIsLoading] = useState(false)
  const [isComposing, setIsComposing] = useState(false)
  const [answer, setAnswer] = useState('')
  const [errorText, setErrorText] = useState('')
  const [lastMessage, setLastMessage] = useState('')

  const focusInput = useCallback(() => {
    requestAnimationFrame(() => {
      inputRef.current?.focus()
    })
  }, [])

  useEffect(() => {
    const off = EventsOn(focusInputEventName, () => {
      focusInput()
    })

    focusInput()
    return () => {
      off()
    }
  }, [focusInput])

  const submit = useCallback(
    async (rawMessage?: string) => {
      if (isLoading) {
        return
      }

      const message = (rawMessage ?? query).trim()
      if (!message) {
        return
      }

      setIsLoading(true)
      setErrorText('')
      setAnswer('')
      setLastMessage(message)

      try {
        const result = (await SendChat(message)) as ChatResult
        if (result?.error) {
          setErrorText(mapErrorToMessage(result.error))
          return
        }
        setAnswer(result?.response || '')
      } catch {
        setErrorText('请求失败，请稍后重试。')
      } finally {
        setIsLoading(false)
      }
    },
    [isLoading, query],
  )

  const onEsc = useCallback(async () => {
    if (query.length > 0) {
      setQuery('')
      return
    }

    if (isSearchMode) {
      setIsSearchMode(false)
      return
    }

    await HideLauncher()
  }, [isSearchMode, query])

  const onInputKeyDown = useCallback(
    async (event: KeyboardEvent<HTMLInputElement>) => {
      if (event.key === 'Escape') {
        event.preventDefault()
        await onEsc()
        return
      }

      if (event.key === 'Enter') {
        const nativeEvent = event.nativeEvent as globalThis.KeyboardEvent
        if (isComposing || nativeEvent.isComposing || nativeEvent.keyCode === 229) {
          return
        }
        event.preventDefault()
        await submit()
      }
    },
    [isComposing, onEsc, submit],
  )

  const placeholder = useMemo(() => {
    if (isSearchMode) {
      return '搜索历史（占位）...'
    }
    return '输入问题并按 Enter 发送...'
  }, [isSearchMode])

  return (
    <div id="app" className="launcher">
      <div className="launcher-header">
        <span className="mode">{isSearchMode ? '搜索态' : '输入态'}</span>
        <button className="btn" onClick={() => setIsSearchMode((prev) => !prev)} disabled={isLoading}>
          {isSearchMode ? '退出搜索' : '进入搜索'}
        </button>
      </div>

      <div className="input-box">
        <input
          ref={inputRef}
          id="query"
          className="input"
          autoComplete="off"
          name="query"
          type="text"
          value={query}
          placeholder={placeholder}
          onChange={(e) => setQuery(e.target.value)}
          onCompositionStart={() => setIsComposing(true)}
          onCompositionEnd={() => setIsComposing(false)}
          onKeyDown={onInputKeyDown}
          disabled={isLoading}
        />
        <button className="btn" onClick={() => submit()} disabled={isLoading || !query.trim()}>
          {isLoading ? '发送中' : '发送'}
        </button>
      </div>

      {answer && (
        <div className="response">
          <ReactMarkdown>{answer}</ReactMarkdown>
        </div>
      )}

      {errorText && (
        <div className="error-box">
          <div className="error-text">{errorText}</div>
          <button className="btn" onClick={() => submit(lastMessage)} disabled={isLoading || !lastMessage}>
            重试
          </button>
        </div>
      )}

      <div className="hint">Enter 发送；Esc：清空输入 → 退出搜索 → 隐藏窗口</div>
    </div>
  )
}

export default App
