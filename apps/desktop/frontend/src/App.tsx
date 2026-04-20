import { useCallback, useEffect, useMemo, useRef, useState, type KeyboardEvent } from 'react'
import './App.css'
import { EventsOn } from '../wailsjs/runtime/runtime'
import { HideLauncher } from '../wailsjs/go/main/App'

const focusInputEventName = 'launcher:focus-input'

function App() {
  const inputRef = useRef<HTMLInputElement>(null)
  const [query, setQuery] = useState('')
  const [isSearchMode, setIsSearchMode] = useState(false)

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
      if (event.key !== 'Escape') {
        return
      }
      event.preventDefault()
      await onEsc()
    },
    [onEsc],
  )

  const placeholder = useMemo(() => {
    if (isSearchMode) {
      return '搜索历史（占位）...'
    }
    return '输入问题（Phase B 占位）...'
  }, [isSearchMode])

  return (
    <div id="app" className="launcher">
      <div className="launcher-header">
        <span className="mode">{isSearchMode ? '搜索态' : '输入态'}</span>
        <button className="btn" onClick={() => setIsSearchMode((prev) => !prev)}>
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
          onKeyDown={onInputKeyDown}
        />
      </div>

      <div className="hint">Esc：清空输入 → 退出搜索 → 隐藏窗口</div>
    </div>
  )
}

export default App
