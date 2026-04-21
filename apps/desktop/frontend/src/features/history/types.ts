export type HistorySearchItem = {
  messageId: number
  sessionId: string
  sessionTitle: string
  role: string
  content: string
  status: string
  errorCode: string
  createdAt: number
}

export type HistoryJumpTarget = {
  messageId: number
  sessionId: string
}
