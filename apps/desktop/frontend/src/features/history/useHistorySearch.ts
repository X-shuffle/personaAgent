import { useCallback, useEffect, useRef, useState, type KeyboardEvent as ReactKeyboardEvent } from 'react'
import { searchHistory } from './api'
import type { HistoryJumpTarget, HistorySearchItem } from './types'

type UseHistorySearchOptions = {
  enabled: boolean
  query: string
  isComposing: boolean
  onJump: (target: HistoryJumpTarget) => void
}

const searchDebounceMs = 150

// 管理历史搜索、键盘导航与命中跳转，服务输入框“边输边搜”体验。
export function useHistorySearch(options: UseHistorySearchOptions) {
  const { enabled, query, isComposing, onJump } = options
  const [results, setResults] = useState<HistorySearchItem[]>([])
  const [activeIndex, setActiveIndex] = useState(-1)
  const [isLoading, setIsLoading] = useState(false)
  const [errorText, setErrorText] = useState('')
  const [recentBrowseActive, setRecentBrowseActive] = useState(false)
  const [recentBrowseSeq, setRecentBrowseSeq] = useState(0)
  const requestSeqRef = useRef(0)

  useEffect(() => {
    // 用户重新输入关键词后，退出“最近浏览”模式，回到关键词搜索语义。
    if (query.trim() !== '') {
      setRecentBrowseActive(false)
    }
  }, [query])

  useEffect(() => {
    if (!enabled) {
      setResults([])
      setActiveIndex(-1)
      setIsLoading(false)
      setErrorText('')
      setRecentBrowseActive(false)
      return
    }

    const keyword = query.trim()
    // 空输入 + recentBrowseActive 表示用户在无关键词时主动浏览最近消息。
    const shouldLoadRecent = keyword === '' && recentBrowseActive
    if (!keyword && !shouldLoadRecent) {
      setResults([])
      setActiveIndex(-1)
      setIsLoading(false)
      setErrorText('')
      return
    }

    const delay = keyword === '' ? 0 : searchDebounceMs
    const timer = window.setTimeout(async () => {
      const requestSeq = requestSeqRef.current + 1
      requestSeqRef.current = requestSeq
      setIsLoading(true)
      setErrorText('')

      try {
        const items = await searchHistory(keyword)
        if (requestSeqRef.current !== requestSeq) {
          return
        }
        setResults(items)
        setActiveIndex((prev) => {
          if (items.length === 0) {
            return -1
          }
          if (prev < 0) {
            return 0
          }
          return Math.min(prev, items.length - 1)
        })
        if (shouldLoadRecent && items.length > 0) {
          const first = items[0]
          onJump({
            messageId: first.messageId,
          })
        }
      } catch {
        if (requestSeqRef.current !== requestSeq) {
          return
        }
        setResults([])
        setActiveIndex(-1)
        setErrorText('历史搜索失败，请稍后重试。')
      } finally {
        if (requestSeqRef.current === requestSeq) {
          setIsLoading(false)
        }
      }
    }, delay)

    return () => {
      window.clearTimeout(timer)
    }
  }, [enabled, query, recentBrowseActive, recentBrowseSeq])

  // 跳转到指定搜索结果：更新高亮并通知外层加载上下文。
  const jumpToIndex = useCallback(
    (index: number): boolean => {
      const item = results[index]
      if (!item) {
        return false
      }
      setActiveIndex(index)
      onJump({
        messageId: item.messageId,
      })
      return true
    },
    [onJump, results],
  )

  // 打开当前高亮结果；无高亮时默认打开第一条。
  const openActive = useCallback((): boolean => {
    if (results.length === 0) {
      return false
    }

    const index = activeIndex >= 0 ? activeIndex : 0
    return jumpToIndex(index)
  }, [activeIndex, jumpToIndex, results.length])

  // 处理历史搜索相关快捷键（↑/↓/Enter），并返回是否已消费事件。
  const onKeyDown = useCallback(
    (event: ReactKeyboardEvent<HTMLInputElement>): boolean => {
      if (!enabled) {
        return false
      }

      if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
        if (query.trim() === '') {
          setRecentBrowseActive(true)
          setRecentBrowseSeq((prev) => prev + 1)
          if (results.length === 0) {
            event.preventDefault()
            return true
          }
        }

        if (results.length === 0) {
          return false
        }

        event.preventDefault()
        const delta = event.key === 'ArrowDown' ? 1 : -1
        const baseIndex = activeIndex >= 0 ? activeIndex : 0
        const nextIndex = Math.max(0, Math.min(baseIndex + delta, results.length - 1))
        return jumpToIndex(nextIndex)
      }

      if (event.key === 'Enter') {
        const nativeEvent = event.nativeEvent as globalThis.KeyboardEvent
        if (isComposing || nativeEvent.isComposing) {
          return false
        }

        const handled = openActive()
        if (handled) {
          event.preventDefault()
        }
        return handled
      }

      return false
    },
    [activeIndex, enabled, isComposing, jumpToIndex, openActive, query, results.length],
  )

  return {
    results,
    activeIndex,
    isLoading,
    errorText,
    onKeyDown,
    jumpToIndex,
  }
}
