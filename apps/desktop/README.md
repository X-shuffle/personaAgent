# README

## About

This is the official Wails React-TS template.

You can configure the project by editing `wails.json`. More information about the project settings can be found
here: https://wails.io/docs/reference/project-config

## Live Development

To run in live development mode, run `wails dev` in the project directory. This will run a Vite development
server that will provide very fast hot reload of your frontend changes. If you want to develop in a browser
and have access to your Go methods, there is also a dev server that runs on http://localhost:34115. Connect
to this in your browser, and you can call your Go code from devtools.

## UI Concept Map (ASCII)

> 用于对齐 launcher 前端界面分区概念，便于后续讨论和提问。

```text
┌──────────────────────────────────────────────────────────────────────┐
│                              Launcher                               │
│                         (#app .launcher)                            │
│                                                                      │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │ [输入框 input]                               [发送按钮 btn]  │    │
│  │ placeholder: 输入问题（自动搜索历史），按 Enter 发送...      │    │
│  └──────────────────────────────────────────────────────────────┘    │
│                                                                      │
│  (条件显示) 历史浮层 showHistoryPanel                                │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │ HistorySearchPanel                                           │    │
│  │ - 搜索结果列表                                                │    │
│  │ - activeIndex 高亮                                            │    │
│  │ - loading / error 状态                                        │    │
│  └──────────────────────────────────────────────────────────────┘    │
│                                                                      │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │ 消息区 message-list (role=log)                               │    │
│  │                                                              │    │
│  │  A) 空态: message-empty                                      │    │
│  │     - 发送中: “正在发送中，请稍候...”                        │    │
│  │     - 否则: “还没有消息...”                                  │    │
│  │                                                              │    │
│  │  B) 列表态: 多个 message-item                                │    │
│  │     ┌────────────────────────────────────────────────────┐   │    │
│  │     │ meta: 角色(你/助手) + 可选“历史命中”标签           │   │    │
│  │     │ content:                                            │   │    │
│  │     │  - 用户: 纯文本                                     │   │    │
│  │     │  - 助手: ReactMarkdown + remarkGfm 渲染             │   │    │
│  │     └────────────────────────────────────────────────────┘   │    │
│  │                                                              │    │
│  │  C) 发送中提示: “正在连接后端并生成回复...”                 │    │
│  └──────────────────────────────────────────────────────────────┘    │
│                                                                      │
│  (条件显示) 错误区 error-box                                         │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │ error-text: 映射后的中文错误                                  │    │
│  │ [重试按钮]（用 lastMessage 再次 submit）                      │    │
│  └──────────────────────────────────────────────────────────────┘    │
│                                                                      │
│  底部提示 hint                                                       │
│  “↑/↓ 选择历史结果；Enter 发送；Esc：清空输入 → 隐藏窗口”          │
└──────────────────────────────────────────────────────────────────────┘
```
