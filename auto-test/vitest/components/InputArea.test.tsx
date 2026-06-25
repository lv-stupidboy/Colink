import React from 'react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { fireEvent, render, screen } from '@testing-library/react'
import { InputArea } from '../../../web/src/pages/thread/InputArea'

const agentOptions = [
  {
    id: 'agent-dev',
    role: 'agent' as const,
    name: 'DevAgent',
    label: 'DevAgent',
  },
  {
    id: 'agent-review',
    role: 'agent' as const,
    name: 'ReviewAgent',
    label: 'ReviewAgent',
  },
  {
    id: 'human-reviewer',
    role: 'human' as const,
    name: 'HumanReviewer',
    label: 'HumanReviewer',
  },
]

describe('UI-02: InputArea Agent interactions [P0]', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  // @feature F001 - Agent 对话核心
  // @priority P0
  // @id UI-02-01
  it('UI-02-01: filters mention list and spawns selected agent on send [F001]', () => {
    const onSend = vi.fn()
    const onSpawnAgent = vi.fn()

    render(
      <InputArea
        placeholder="输入任务"
        loadingContext={false}
        agentOptions={agentOptions}
        onSend={onSend}
        onSpawnAgent={onSpawnAgent}
      />,
    )

    const textarea = screen.getByPlaceholderText('输入任务')
    fireEvent.change(textarea, { target: { value: '@Dev' } })

    expect(screen.getByText('DevAgent')).toBeInTheDocument()
    expect(screen.queryByText('ReviewAgent')).not.toBeInTheDocument()

    fireEvent.click(screen.getByText('DevAgent'))
    expect(textarea).toHaveValue('@DevAgent ')

    fireEvent.change(textarea, { target: { value: '@DevAgent implement API tests' } })
    fireEvent.click(screen.getByRole('button', { name: /发送/ }))

    expect(onSend).toHaveBeenCalledWith('@DevAgent implement API tests', true)
    expect(onSpawnAgent).toHaveBeenCalledWith('agent', 'implement API tests', 'agent-dev')
  })

  // @feature F001 - Agent 对话核心
  // @priority P0
  // @id UI-02-02
  it('UI-02-02: sends normal text without spawning agent [F001]', () => {
    const onSend = vi.fn()
    const onSpawnAgent = vi.fn()

    render(
      <InputArea
        placeholder="输入任务"
        loadingContext={false}
        agentOptions={agentOptions}
        onSend={onSend}
        onSpawnAgent={onSpawnAgent}
      />,
    )

    const textarea = screen.getByPlaceholderText('输入任务')
    fireEvent.change(textarea, { target: { value: 'plain user message' } })
    fireEvent.keyDown(textarea, { key: 'Enter', code: 'Enter' })

    expect(onSend).toHaveBeenCalledWith('plain user message')
    expect(onSpawnAgent).not.toHaveBeenCalled()
    expect(textarea).toHaveValue('')
  })

  // @feature F001 - Agent 对话核心
  // @priority P1
  // @id UI-02-03
  it('UI-02-03: keeps unmatched mentions on the regular message path [F001]', () => {
    const onSend = vi.fn()
    const onSpawnAgent = vi.fn()

    render(
      <InputArea
        placeholder="输入任务"
        loadingContext={false}
        agentOptions={agentOptions}
        onSend={onSend}
        onSpawnAgent={onSpawnAgent}
      />,
    )

    const textarea = screen.getByPlaceholderText('输入任务')
    fireEvent.change(textarea, { target: { value: '@MissingAgent hello' } })
    fireEvent.click(screen.getByRole('button', { name: /发送/ }))

    expect(onSend).toHaveBeenCalledWith('@MissingAgent hello')
    expect(onSpawnAgent).not.toHaveBeenCalled()
  })
})
