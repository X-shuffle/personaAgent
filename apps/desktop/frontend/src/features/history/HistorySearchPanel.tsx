import type { HistorySearchItem } from './types'

type HistorySearchPanelProps = {
  query: string
  results: HistorySearchItem[]
  activeIndex: number
  isLoading: boolean
  errorText: string
  onSelect: (index: number) => void
}

export default function HistorySearchPanel(props: HistorySearchPanelProps) {
  const { query, results, activeIndex, isLoading, errorText, onSelect } = props

  return (
    <div className="history-panel" aria-live="polite">
      {isLoading && <div className="history-state">搜索中...</div>}
      {!isLoading && errorText && <div className="history-state history-state-error">{errorText}</div>}
      {!isLoading && !errorText && query.trim() === '' && <div className="history-state">输入关键词搜索历史</div>}
      {!isLoading && !errorText && query.trim() !== '' && results.length === 0 && (
        <div className="history-state">未命中历史消息</div>
      )}

      {results.length > 0 && (
        <ul className="history-list" role="listbox" aria-label="历史搜索结果">
          {results.map((item, index) => {
            const isActive = index === activeIndex
            const rowClassName = isActive ? 'history-item history-item-active' : 'history-item'
            return (
              <li
                key={item.messageId}
                className={rowClassName}
                role="option"
                aria-selected={isActive}
                onMouseDown={(event) => {
                  event.preventDefault()
                }}
                onClick={() => onSelect(index)}
              >
                <div className="history-item-meta">
                  <span>{item.role === 'assistant' ? '助手' : '你'}</span>
                  {item.sessionTitle && <span>{item.sessionTitle}</span>}
                </div>
                <div className="history-item-content">{item.content}</div>
              </li>
            )
          })}
        </ul>
      )}
    </div>
  )
}
