# ISDP Frontend

React 18 + TypeScript + Vite + Ant Design 5 + Zustand 4. Dev server port 3000, proxies `/api/*` to backend 8080.

## Structure

```
src/
├── api/client.ts          # Axios client. snake→camelCase transform. Namespaced methods.
├── store/index.ts          # Zustand + subscribeWithSelector. 70+ fields, 30+ actions.
├── types/index.ts          # All domain types + enums + display labels (845 lines)
├── pages/                  # Route-level components (ThreadView, AgentRoleList, SkillLibrary, etc.)
├── components/
│   ├── thread/             # 30 files — message rendering, chat input, panels
│   │   ├── StatusPanel/    # 10 files — agent status, token usage, session chains
│   │   └── CodePanel/      # 5 files — file diff viewer
│   └── ...                 # ArtifactCard, FileTree, ThemeSwitcher, Logo, etc.
├── hooks/useWebSocket.ts   # WebSocket with auto-reconnect
├── layouts/MainLayout.tsx  # Sidebar + content layout
├── themes/                 # 4 theme files (dark/light + variants)
├── config/                 # Version config (generated at build time)
└── utils/                  # Formatting helpers
```

## Patterns

**State management**: Zustand with `subscribeWithSelector`. Use selectors to subscribe to specific slices:
```tsx
// Low-frequency state
const currentThread = useAppStore((s) => s.currentThread);
// High-frequency state (streaming handled by separate component)
const messages = useAppStore((s) => s.messages);
// Actions via getState() — no subscription
const loadThread = useAppStore((s) => s.loadThread);
```

**API client**: `api/client.ts` — Axios with namespaced methods:
```tsx
api.skills.list({ pageSize: 100 })
api.threads.get(threadId)
api.agents.bindSkills(agentId, skillIds)
```
Response interceptor transforms snake_case → camelCase per-endpoint.

**Page component pattern** (list pages):
```
useState for local data + loading + pagination
Form.useForm() for modal forms
useEffect for initial data load
Modal for CRUD dialogs
Table/Card for list display
```

**WebSocket**: `useWebSocket` hook with auto-reconnect. ThreadView maintains separate refs for team mode vs debug mode.

## Key Pages

| Page | Lines | Role |
|------|-------|------|
| `ThreadView.tsx` | 1678 | Main workbench. Dual-mode (team/debug). WebSocket streaming. |
| `SkillLibrary/index.tsx` | 989 | Skill CRUD with file upload + tag management |
| `AgentRoleList.tsx` | 914 | Agent role config with preview |
| `SubagentList.tsx` | 715 | Subagent CRUD with markdown parsing |
| `CommandList.tsx` | 654 | Command management with skill association |
| `store/index.ts` | 640 | Global state: threads, messages, agents, streaming |
| `AssetPackage/index.tsx` | 635 | Asset package import/export |
| `ProjectDetail/index.tsx` | 616 | Project detail with thread list |

## Anti-Patterns

- `as any`, `@ts-ignore`, `@ts-expect-error` — forbidden
- Empty catch blocks — always handle/display error
- Direct state mutation — always use Zustand actions
- Subscribing to entire store — use specific selectors

## Testing

- **E2E**: Playwright. 12 spec files in `tests/e2e/`. Run `npm run test:e2e`.
- **Config**: `playwright.config.ts` — Chromium, baseURL `localhost:3001`, HTML+JSON reporters.
- **Fixtures**: `tests/fixtures/test-fixtures.ts` — custom result aggregation.
- **Test IDs**: FT-01 through FT-12 mapped to test plan.

## Theme System

Zustand `themeStore` persisted to localStorage key `isdp-theme-storage`. Ant Design `ConfigProvider` generates tokens dynamically. CSS variables (`--color-primary`, `--bg-base`, etc.) for non-Ant elements.
