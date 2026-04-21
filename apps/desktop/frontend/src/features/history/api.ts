import { LoadMessageContext, SearchHistory } from '../../../wailsjs/go/main/App'
import type { main } from '../../../wailsjs/go/models'
import type { HistorySearchItem } from './types'

const defaultLimit = 20

function mapSearchItem(item: main.HistorySearchItem): HistorySearchItem {
  return {
    messageId: item.message_id,
    sessionId: item.session_id,
    sessionTitle: item.session_title,
    role: item.role,
    content: item.content,
    status: item.status,
    errorCode: item.error_code,
    createdAt: item.created_at,
  }
}

export async function searchHistory(keyword: string, limit: number = defaultLimit): Promise<HistorySearchItem[]> {
  const hits = await SearchHistory(keyword, limit, 0)
  return hits.map((item) => mapSearchItem(item))
}

export async function loadMessageContext(messageId: number): Promise<HistorySearchItem[]> {
  const items = await LoadMessageContext(messageId)
  return items.map((item) => mapSearchItem(item))
}
