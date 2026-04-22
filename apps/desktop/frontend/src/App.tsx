import { useCallback, useEffect, useRef, useState, type KeyboardEvent } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import './App.css'
import { ClipboardSetText, EventsOn } from '../wailsjs/runtime/runtime'
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
  role: 'user' | 'assistant'
  content: string
  source: 'chat' | 'history'
}

// 优先透传后端错误内容，便于快速定位真实故障。
function mapErrorToMessage(err?: ChatError): string {
  if (!err) {
    return '请求失败，请稍后重试。'
  }

  const message = (err.message || '').trim()
  if (message) {
    return message
  }

  if (err.code) {
    return `请求失败（${err.code}）。`
  }

  if (err.status_code) {
    return `请求失败（HTTP ${err.status_code}）。`
  }

  return '请求失败，请稍后重试。'
}

// App 是启动器主界面：输入、发送、历史搜索与历史回跳都在这里编排。
function App() {
  const inputRef = useRef<HTMLInputElement>(null)
  const messageListRef = useRef<HTMLDivElement>(null)
  const idCounterRef = useRef(0)
  const jumpRequestSeqRef = useRef(0)

  const [query, setQuery] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const [isComposing, setIsComposing] = useState(false)
  const [errorText, setErrorText] = useState('')
  const [lastMessage, setLastMessage] = useState('')
  const [messages, setMessages] = useState<RenderedMessage[]>([])
  const [copiedMessageLocalId, setCopiedMessageLocalId] = useState('')

  const copyAssistantMessage = useCallback(async (item: RenderedMessage) => {
    if (item.role !== 'assistant') {
      return
    }

    const copied = await ClipboardSetText(item.content)
    if (!copied) {
      return
    }

    setCopiedMessageLocalId(item.localId)
    window.setTimeout(() => {
      setCopiedMessageLocalId((prev) => (prev === item.localId ? '' : prev))
    }, 1200)
  }, [])

  // 生成仅前端使用的本地消息 ID，避免和后端 messageId 混淆。
  const nextLocalId = useCallback(() => {
    idCounterRef.current += 1
    return `${Date.now()}-${idCounterRef.current}`
  }, [])

  // 在下一帧聚焦输入框，避免窗口刚显示时焦点竞争。
  const focusInput = useCallback(() => {
    requestAnimationFrame(() => {
      inputRef.current?.focus()
    })
  }, [])

  // 监听后端发来的聚焦事件：窗口弹出时自动把光标放回输入框。
  useEffect(() => {
    const off = EventsOn(focusInputEventName, () => {
      focusInput()
    })

    focusInput()
    return () => {
      off()
    }
  }, [focusInput])

  // 跨窗口点击时当前 WebView 通常收不到 pointerdown，改用失焦作为“点击外侧”兜底。
  useEffect(() => {
    const onWindowBlur = () => {
      requestAnimationFrame(() => {
        if (!document.hasFocus()) {
          void HideLauncher()
        }
      })
    }

    window.addEventListener('blur', onWindowBlur)
    return () => {
      window.removeEventListener('blur', onWindowBlur)
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

      // 先乐观渲染用户消息，再异步补齐助手回复，保持输入反馈即时。
      setIsLoading(true)
      setErrorText('')
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
          console.error('[desktop] SendChat returned error', {
            code: result.error.code,
            statusCode: result.error.status_code,
            message: result.error.message,
            requestMessage: message,
          })
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
      } catch (error) {
        console.error('[desktop] SendChat threw exception', {
          requestMessage: message,
          error,
        })
        setErrorText('网络或服务异常，请检查连接后重试。')
      } finally {
        setIsLoading(false)
      }
    },
    [isLoading, nextLocalId, query],
  )

  // 统一处理 Esc：有输入先清空，无输入再隐藏窗口。
  const onEsc = useCallback(async () => {
    if (query.length > 0) {
      setQuery('')
      return
    }

    await HideLauncher()
  }, [query])

  const onHistoryJump = useCallback(async (target: HistoryJumpTarget) => {
    // 用递增序号丢弃过期请求，避免快速切换历史时发生回填乱序。
    const requestSeq = jumpRequestSeqRef.current + 1
    jumpRequestSeqRef.current = requestSeq

    try {
      const contextItems = await loadMessageContext(target.messageId)
      if (jumpRequestSeqRef.current !== requestSeq) {
        return
      }

      if (contextItems.length === 0) {
        return
      }

      // 仅展示命中消息及其邻近上下文，聚焦“从历史跳回当前会话”场景。
      const focusedMessages: RenderedMessage[] = contextItems.map((item) => {
        const role: RenderedMessage['role'] = item.role === 'assistant' ? 'assistant' : 'user'
        return {
          localId: `history-${item.messageId}`,
          messageId: item.messageId,
          role,
          content: item.content,
          source: 'history',
        }
      })

      setMessages(focusedMessages)
    } catch {
      return
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

  // 输入框键盘分发：先处理显隐/历史导航，再处理发送。
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

  const showHistoryPanel = isHistoryLoading || historyErrorText !== '' || historyResults.length > 0
  const hintText = '↑/↓ 选择历史结果；Enter 发送；Esc：清空输入 → 隐藏窗口'

  // 新消息进入后自动滚动到底部，保证最新问答始终可见。
  useEffect(() => {
    const container = messageListRef.current
    if (!container) {
      return
    }
    container.scrollTop = container.scrollHeight
  }, [messages, isLoading])

  const emptyText = isLoading ? '正在发送中，请稍候...' : '还没有消息，输入问题后按 Enter 开始对话。'
  const isCompactMessageArea = messages.length === 0 && !isLoading


  return (
    <div id="app">
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

      {showHistoryPanel && (
        <div className="history-floating">
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
        </div>
      )}

      <div
        ref={messageListRef}
        className={isCompactMessageArea ? 'message-list message-list-compact' : 'message-list'}
        role="log"
        aria-live="polite"
      >
        {messages.length === 0 ? (
          <div className="message-empty">{emptyText}</div>
        ) : (
          messages.map((item) => {
            const domId = item.messageId != null ? `msg-${item.messageId}` : `msg-local-${item.localId}`
            const className = `message-item message-${item.role}`
            return (
              <div id={domId} key={domId} className={className} data-message-id={item.messageId ?? ''}>
                <div className="message-meta">
                  <span className="message-role">{item.role === 'assistant' ? '助手' : '你'}</span>
                  {item.source === 'history' && <span className="message-source">历史命中</span>}
                  {item.role === 'assistant' && (
                    <button
                      className="message-copy-btn"
                      type="button"
                      onClick={() => {
                        void copyAssistantMessage(item)
                      }}
                      title={copiedMessageLocalId === item.localId ? '已复制' : '复制消息'}
                      aria-label={copiedMessageLocalId === item.localId ? '已复制' : '复制消息'}
                    >
                      {copiedMessageLocalId === item.localId ? '✓' : '⧉'}
                    </button>
                  )}
                </div>
                <div className="message-content">
                  {item.role === 'assistant' ? (
                    <ReactMarkdown remarkPlugins={[remarkGfm]}>{item.content}</ReactMarkdown>
                  ) : (
                    item.content
                  )}
                </div>
              </div>
            )
          })
        )}
        {isLoading && <div className="message-loading">正在连接后端并生成回复...</div>}
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
