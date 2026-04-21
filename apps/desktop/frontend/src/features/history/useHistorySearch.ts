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

export function useHistorySearch(options: UseHistorySearchOptions) {
  const { enabled, query, isComposing, onJump } = options
  const [results, setResults] = useState<HistorySearchItem[]>([])
  const [activeIndex, setActiveIndex] = useState(-1)
  const [isLoading, setIsLoading] = useState(false)
  const [errorText, setErrorText] = useState('')
  const requestSeqRef = useRef(0)

  useEffect(() => {
    if (!enabled) {
      setResults([])
      setActiveIndex(-1)
      setIsLoading(false)
      setErrorText('')
      return
    }

    const keyword = query.trim()
    if (!keyword) {
      setResults([])
      setActiveIndex(-1)
      setIsLoading(false)
      setErrorText('')
      return
    }

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
        setActiveIndex(items.length > 0 ? 0 : -1)
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
    }, searchDebounceMs)

    return () => {
      window.clearTimeout(timer)
    }
  }, [enabled, query])

  const jumpToIndex = useCallback(
    (index: number): boolean => {
      const item = results[index]
      if (!item) {
        return false
      }
      setActiveIndex(index)
      onJump({
        messageId: item.messageId,
        sessionId: item.sessionId,
      })
      return true
    },
    [onJump, results],
  )

  const openActive = useCallback((): boolean => {
    if (results.length === 0) {
      return false
    }

    const index = activeIndex >= 0 ? activeIndex : 0
    return jumpToIndex(index)
  }, [activeIndex, jumpToIndex, results.length])

  const onKeyDown = useCallback(
    (event: ReactKeyboardEvent<HTMLInputElement>): boolean => {
      if (!enabled) {
        return false
      }

      if (event.key === 'ArrowDown') {
        if (results.length === 0) {
          return false
        }
        event.preventDefault()
        setActiveIndex((prev) => Math.min(prev + 1, results.length - 1))
        return true
      }

      if (event.key === 'ArrowUp') {
        if (results.length === 0) {
          return false
        }
        event.preventDefault()
        setActiveIndex((prev) => Math.max(prev - 1, 0))
        return true
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
    [enabled, isComposing, openActive, results.length],
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
