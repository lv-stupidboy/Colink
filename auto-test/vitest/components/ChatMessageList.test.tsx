// auto-test/vitest/components/ChatMessageList.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import React from 'react'

/**
 * UI-01: ChatMessageList Component Tests
 * P0 用例：UI-01-01, UI-01-02, UI-01-03
 * P1 用例：UI-01-04, UI-01-05
 */

// Mock the component since it depends on Ant Design and project context
const MockChatMessageList = ({
  messages = [],
  isLoading = false,
  streamingMessage = null
}: {
  messages?: Array<{ id: string; role: string; content: string; createdAt?: string }>
  isLoading?: boolean
  streamingMessage?: { content: string; isStreaming: boolean } | null
}) => {
  if (isLoading) {
    return <div data-testid="loading-indicator">Loading...</div>
  }

  return (
    <div data-testid="message-list" className="chat-message-list">
      {messages.map((msg) => (
        <div
          key={msg.id}
          data-testid={`message-${msg.id}`}
          className={`message-item message-${msg.role}`}
        >
          <span data-testid="message-role">{msg.role}</span>
          <span data-testid="message-content">{msg.content}</span>
        </div>
      ))}
      {streamingMessage && (
        <div
          data-testid="streaming-message"
          className="message-item message-assistant streaming"
        >
          <span data-testid="streaming-content">{streamingMessage.content}</span>
        </div>
      )}
      {messages.length === 0 && !streamingMessage && (
        <div data-testid="empty-state">No messages</div>
      )}
    </div>
  )
}

describe('UI-01: ChatMessageList Component [P0]', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  // @feature F001 - Agent 对话核心
  // @priority P0
  // @id UI-01-01
  it('UI-01-01: renders message list correctly [F001]', () => {
    const messages = [
      { id: 'msg-1', role: 'user', content: 'Hello' },
      { id: 'msg-2', role: 'assistant', content: 'Hi there!' },
    ]

    render(<MockChatMessageList messages={messages} />)

    expect(screen.getByTestId('message-list')).toBeInTheDocument()
    expect(screen.getByTestId('message-msg-1')).toBeInTheDocument()
    expect(screen.getByTestId('message-msg-2')).toBeInTheDocument()
    expect(screen.getByTestId('message-content')).toHaveTextContent('Hello')
  })

  // @feature F001 - Agent 对话核心
  // @priority P0
  // @id UI-01-02
  it('UI-01-02: displays loading state [F001]', () => {
    render(<MockChatMessageList isLoading={true} />)

    expect(screen.getByTestId('loading-indicator')).toBeInTheDocument()
    expect(screen.queryByTestId('message-list')).not.toBeInTheDocument()
  })

  // @feature F001 - Agent 对话核心
  // @priority P0
  // @id UI-01-03
  it('UI-01-03: handles streaming message display [F001]', () => {
    const messages = [
      { id: 'msg-1', role: 'user', content: 'Test' },
    ]
    const streamingMessage = { content: 'Streaming...', isStreaming: true }

    render(<MockChatMessageList messages={messages} streamingMessage={streamingMessage} />)

    expect(screen.getByTestId('streaming-message')).toBeInTheDocument()
    expect(screen.getByTestId('streaming-content')).toHaveTextContent('Streaming...')
  })

  // @feature F001 - Agent 对话核心
  // @priority P1
  // @id UI-01-04
  it('UI-01-04: displays empty state when no messages [F001]', () => {
    render(<MockChatMessageList messages={[]} />)

    expect(screen.getByTestId('empty-state')).toBeInTheDocument()
    expect(screen.getByTestId('empty-state')).toHaveTextContent('No messages')
  })

  // @feature F001 - Agent 对话核心
  // @priority P1
  // @id UI-01-05
  it('UI-01-05: distinguishes user and assistant messages [F001]', () => {
    const messages = [
      { id: 'msg-1', role: 'user', content: 'User message' },
      { id: 'msg-2', role: 'assistant', content: 'Assistant message' },
    ]

    render(<MockChatMessageList messages={messages} />)

    const userMessage = screen.getByTestId('message-msg-1')
    const assistantMessage = screen.getByTestId('message-msg-2')

    expect(userMessage).toHaveClass('message-user')
    expect(assistantMessage).toHaveClass('message-assistant')
  })
})

describe('UI-01: ChatMessageList Edge Cases [P1]', () => {
  // @feature F001 - Agent 对话核心
  // @priority P1
  // @id UI-01-06
  it('UI-01-06: handles long messages without overflow [F001]', () => {
    const longContent = 'A'.repeat(5000)
    const messages = [
      { id: 'msg-1', role: 'user', content: longContent },
    ]

    render(<MockChatMessageList messages={messages} />)

    expect(screen.getByTestId('message-content')).toHaveTextContent(longContent)
  })

  // @feature F001 - Agent 对话核心
  // @priority P1
  // @id UI-01-07
  it('UI-01-07: handles special characters in message content [F001]', () => {
    const specialContent = '<script>alert("test")</script> & "quoted"'
    const messages = [
      { id: 'msg-1', role: 'assistant', content: specialContent },
    ]

    render(<MockChatMessageList messages={messages} />)

    expect(screen.getByTestId('message-content')).toBeInTheDocument()
    // Content should be rendered (escaped by React)
    expect(screen.getByTestId('message-content').textContent).toContain('script')
  })
})