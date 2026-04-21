import { useCallback, useEffect, useRef, useState, type KeyboardEvent } from 'react'
import ReactMarkdown from 'react-markdown'
import './App.css'
import { EventsOn } from '../wailsjs/runtime/runtime'
import { HideLauncher, SendChat } from '../wailsjs/go/main/App'
import { loadMessageContext } from './features/history/api'
import HistorySearchPanel from './features/history/HistorySearchPanel'
import { useHistorySearch } from './features/history/useHistorySearch'
import type { HistoryJumpTarget } from './features/history/types'

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

type RenderedMessage = {
  localId: string
  messageId?: number
  sessionId?: string
  role: 'user' | 'assistant'
  content: string
  source: 'chat' | 'history'
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
  const idCounterRef = useRef(0)
  const highlightTimerRef = useRef<number | null>(null)
  const jumpRequestSeqRef = useRef(0)

  const [query, setQuery] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const [isComposing, setIsComposing] = useState(false)
  const [errorText, setErrorText] = useState('')
  const [lastMessage, setLastMessage] = useState('')
  const [messages, setMessages] = useState<RenderedMessage[]>([])
  const [pendingScrollMessageId, setPendingScrollMessageId] = useState<number | null>(null)
  const [highlightedMessageId, setHighlightedMessageId] = useState<number | null>(null)
  const [jumpInfoText, setJumpInfoText] = useState('')

  const nextLocalId = useCallback(() => {
    idCounterRef.current += 1
    return `${Date.now()}-${idCounterRef.current}`
  }, [])

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

  useEffect(() => {
    return () => {
      if (highlightTimerRef.current != null) {
        window.clearTimeout(highlightTimerRef.current)
      }
    }
  }, [])

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
      setJumpInfoText('')
      setLastMessage(message)
      setMessages((prev) => [
        ...prev,
        {
          localId: nextLocalId(),
          role: 'user',
          content: message,
          source: 'chat',
        },
      ])

      try {
        const result = (await SendChat(message)) as ChatResult
        if (result?.error) {
          setErrorText(mapErrorToMessage(result.error))
          return
        }

        const response = result?.response || ''
        if (response) {
          setMessages((prev) => [
            ...prev,
            {
              localId: nextLocalId(),
              role: 'assistant',
              content: response,
              source: 'chat',
            },
          ])
        }
      } catch {
        setErrorText('请求失败，请稍后重试。')
      } finally {
        setIsLoading(false)
      }
    },
    [isLoading, nextLocalId, query],
  )

  const onEsc = useCallback(async () => {
    if (query.length > 0) {
      setQuery('')
      return
    }

    await HideLauncher()
  }, [query])

  const onHistoryJump = useCallback(async (target: HistoryJumpTarget) => {
    const requestSeq = jumpRequestSeqRef.current + 1
    jumpRequestSeqRef.current = requestSeq

    try {
      const contextItems = await loadMessageContext(target.messageId)
      if (jumpRequestSeqRef.current !== requestSeq) {
        return
      }

      if (contextItems.length === 0) {
        setJumpInfoText('目标消息未加载。')
        return
      }

      const focusedMessages: RenderedMessage[] = contextItems.map((item) => {
        const role: RenderedMessage['role'] = item.role === 'assistant' ? 'assistant' : 'user'
        return {
          localId: `history-${item.messageId}`,
          messageId: item.messageId,
          sessionId: item.sessionId,
          role,
          content: item.content,
          source: 'history',
        }
      })

      setMessages(focusedMessages)
      setPendingScrollMessageId(target.messageId)
      setJumpInfoText('已定位到历史命中。')
    } catch {
      if (jumpRequestSeqRef.current === requestSeq) {
        setJumpInfoText('加载目标上下文失败。')
      }
    }
  }, [])

  const {
    results: historyResults,
    activeIndex: historyActiveIndex,
    isLoading: isHistoryLoading,
    errorText: historyErrorText,
    onKeyDown: onHistoryKeyDown,
    jumpToIndex,
  } = useHistorySearch({
    enabled: true,
    query,
    isComposing,
    onJump: onHistoryJump,
  })

  useEffect(() => {
    if (pendingScrollMessageId == null) {
      return
    }

    const element = document.getElementById(`msg-${pendingScrollMessageId}`)
    if (!element) {
      return
    }

    element.scrollIntoView({ block: 'center', behavior: 'smooth' })
    setHighlightedMessageId(pendingScrollMessageId)
    setPendingScrollMessageId(null)

    if (highlightTimerRef.current != null) {
      window.clearTimeout(highlightTimerRef.current)
    }
    highlightTimerRef.current = window.setTimeout(() => {
      setHighlightedMessageId((prev) => (prev === pendingScrollMessageId ? null : prev))
    }, 1800)
  }, [messages, pendingScrollMessageId])

  const onInputKeyDown = useCallback(
    async (event: KeyboardEvent<HTMLInputElement>) => {
      if (event.key === 'Escape') {
        event.preventDefault()
        await onEsc()
        return
      }

      if (event.key === 'ArrowUp' || event.key === 'ArrowDown') {
        const handled = onHistoryKeyDown(event)
        if (handled) {
          return
        }
      }

      if (event.key === 'Enter') {
        const nativeEvent = event.nativeEvent as globalThis.KeyboardEvent
        if (isComposing || nativeEvent.isComposing) {
          return
        }
        event.preventDefault()
        await submit()
      }
    },
    [isComposing, onEsc, onHistoryKeyDown, submit],
  )

  const placeholder = '输入问题（自动搜索历史），按 Enter 发送...'

  const hintText = '↑/↓ 选择历史结果；Enter 发送；Esc：清空输入 → 隐藏窗口'

  return (
    <div id="app" className="launcher">
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

      <HistorySearchPanel
        query={query}
        results={historyResults}
        activeIndex={historyActiveIndex}
        isLoading={isHistoryLoading}
        errorText={historyErrorText}
        onSelect={(index) => {
          jumpToIndex(index)
        }}
      />

      {jumpInfoText && <div className="jump-info">{jumpInfoText}</div>}

      <div className="message-list" role="log" aria-live="polite">
        {messages.length === 0 ? (
          <div className="message-empty">暂无消息</div>
        ) : (
          messages.map((item) => {
            const domId = item.messageId != null ? `msg-${item.messageId}` : `msg-local-${item.localId}`
            const isHighlighted = item.messageId != null && item.messageId === highlightedMessageId
            const className = isHighlighted
              ? `message-item message-${item.role} message-highlighted`
              : `message-item message-${item.role}`
            return (
              <div id={domId} key={domId} className={className} data-message-id={item.messageId ?? ''}>
                <div className="message-meta">
                  <span className="message-role">{item.role === 'assistant' ? '助手' : '你'}</span>
                  {item.source === 'history' && <span className="message-source">历史命中</span>}
                </div>
                <div className="message-content">
                  {item.role === 'assistant' ? <ReactMarkdown>{item.content}</ReactMarkdown> : item.content}
                </div>
              </div>
            )
          })
        )}
      </div>

      {errorText && (
        <div className="error-box">
          <div className="error-text">{errorText}</div>
          <button className="btn" onClick={() => submit(lastMessage)} disabled={isLoading || !lastMessage}>
            重试
          </button>
        </div>
      )}

      <div className="hint">{hintText}</div>
    </div>
  )
}

export default App
