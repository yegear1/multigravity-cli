# TASK.md — Tarefas e Roadmap para Código Legado

> Define O QUE precisa ser feito. Reescrito/atualizado no início de cada nova tarefa.

---

## Tarefa Ativa

*(Nenhuma tarefa ativa no momento)*

---

## Log de Tarefas Concluídas

> Histórico anterior arquivado em `ARCHIVE.md` sob `[v2.0.0] - 2026-09-24`.

| Tarefa | Título | Commit(s) | Data |
|---|---|---|---|
| [13.2] | Despacho Concorrente de Tarefas e Subagentes (`multigravity exec [profile\|--all] "<prompt>"`) com pooling paralelo de cotas | feat(headless): fan out one prompt across isolated profiles | 2026-09-26 |
| [13.1] | Autenticação Direta Headless via CLI (`multigravity login` com Google OAuth2 PKCE e callback HTTP efêmero) | feat(auth): add headless OAuth2 PKCE login into the profile vault | 2026-09-26 |
| [11.6] | Alinhamento Estratégico de Posicionamento, Documentação (README, README.pt-br, CHANGELOG) e Catálogo de Recursos da Plataforma | docs(platform): align branding, readme, and changelog with agentic development and ai gateway architecture | 2026-09-26 |
| [08.2] | Invocação e Gestão de Agentes Headless em Background com Isolamento de Identidade | feat(headless): add background headless instance manager, agent prompt runner, and rest endpoints | 2026-09-26 |
| [08.1] | Detecção e Mapeamento de Workspaces e Repositórios Ativos por Perfil | feat(workspace): add workspace mapping, active repository detection, and rest endpoints | 2026-09-26 |
| [15.4] | Visualizador e API de Diffs / Status de Execução de Tarefas no `multigravity serve` para futura GUI Desktop (Tauri/Wails) | feat(dispatch): add structured diff parser, execution dashboard, and web visualizer for desktop guis | 2026-09-25 |
| [15.3] | Motor de Despacho de Tarefas (`multigravity dispatch`) com Associação de Perfil, Worktree e Captura de Logs | feat(dispatch): add task dispatcher with profile isolation, worktrees, and log capture | 2026-09-25 |
| [15.2] | Multiplexador de Terminais PTY e Execução Headless de Agentes CLI (Claude Code, Aider, OpenCode) com Isolamento de Identidade | feat(agent): add pty multiplexer and headless agent execution with profile isolation | 2026-09-25 |
| [15.1] | Gerenciador de Git Worktrees Efêmeros por Agente/Tarefa (`internal/worktree`) | feat(worktree): add ephemeral git worktree manager with cli and rest endpoints | 2026-09-25 |
| [14.4] | Gateway Anthropic-Compatible (`/v1/messages`) e Mapeamento de Modelos (Claude Sonnet/Opus ↔ Gemini 3.5/3.6) | feat(gateway): add anthropic messages gateway with sse and multi-account failover | 2026-09-25 |
| [14.3] | Roteador Multi-Contas com Algoritmos de Distribuição (Smart Priority, Round Robin) e Auto-Failover em HTTP 429/403 entre Perfis | feat(gateway): implement multi-account router with smart load balancing and auto-failover | 2026-09-25 |
| [14.2] | Gateway de Completions OpenAI-Compatible (`/v1/chat/completions`) no `multigravity serve` com SSE | feat(gateway): add openai completions gateway with sse streaming | 2026-09-25 |
| [14.1] | Heurística de Janela de Cotas (5h vs Semanal) e Ping de Aquecimento Proativo | feat(quota): add window heuristic and proactive 5h warm-up prime | 2026-09-25 |
| [07.3] | Priming e Aquecimento de Cotas via API com Emissão de Progresso | feat(prime): add priming API endpoints, SSE progress, and CLI --json | 2026-09-25 |
| [07.2] | Mutação de Compartilhamento Dinâmico via API (Toggle de MCP, Skills, Config, Git/GitHub) | feat(server): add sharing mutation endpoints and dynamic resource toggling | 2026-09-25 |
| [07.1] | Gerenciamento Completo de Perfis via API (Criação com `--auth-only`, Exclusão, Renomeação e Launch/Stop/Restart de Instâncias) | feat(server): implement mutation endpoints for profile creation, deletion, and lifecycle | 2026-09-25 |
| [11.5] | Landing Page Estática e Onboarding Visual Interativo para Iniciantes (GitHub Pages / Showcase) | feat(docs): create interactive landing page and onboarding showcase for github pages | 2026-09-25 |
| [11.4] | Matriz Comparativa Full vs Auth-Only e Atualização de Documentação no README.md | docs(readme): add full vs auth-only comparison matrix and decision guide | 2026-09-25 |
| [11.3] | Ícone Embutido (`//go:embed`) e Associação Automática em Atalhos Desktop | feat(shortcut): embed default icons via go:embed and associate across desktop shortcuts | 2026-09-25 |
| [11.2] | Contrato Machine-Readable (`--json`) no Comando multigravity status | feat(status): add --json flag with byte-level sizing and single profile query | 2026-09-25 |
| [11.1] | Suporte a Perfis Auth-Only Reais (`--auth-only` / `--shared`) com Symlink de Extensões e Configurações Host | feat(profile): add --auth-only flag with host extensions and editor settings symlinks | 2026-09-25 |
| [90.1] | Preservação de Scripts Legados em Branch Separada e Remoção do Diretório legacy/ | refactor(build): isolate legacy scripts to branch and remove legacy directory | 2026-09-24 |
| [06.1] | Detecção de Chats de IA em Uso/Abertos no SO e Suporte a Sync Granular Seguro | feat(ai): detect in-use chats via OS handles with safe granular sync | 2026-09-24 |
| [05.3] | Detecção e Exibição de Tamanho de Conversas (bytes/human-readable) em 'ai list' | feat(ai): add size and size_bytes tracking for AI conversations | 2026-09-24 |
| [05.2] | Detecção de Estado Arquivado/Ativo e Extração de Tópicos em 'ai list' | feat(ai): detect archived and active chats with transcript topic extraction | 2026-09-24 |
| [05.1] | Validação Operacional no Ambiente Real e Correção de Detecção de Executável (FindApp) | fix(app): enforce error on invalid app override and fix install home resolution | 2026-09-24 |
| [04.1] | Suporte a Streaming em Tempo Real via Server-Sent Events (SSE) no Servidor HTTP | feat(server): add Server-Sent Events (SSE) streaming endpoint | 2026-09-24 |
| [03.1] | Expansão da skill canônica do Multigravity e endpoints RPC/HTTP locais para Agregador | feat(server): add local HTTP REST server and expand canonical skill | 2026-09-24 |
| [02.1] | Padronização e Suporte a Contratos Machine-Readable (--json) nos Comandos de Consulta | feat(cli): add --json output flag to list, stats, doctor, quota, ai list, and mcp status | 2026-09-24 |
| [00.1] | Diagnóstico e Auditoria de Integridade Pós-Release v2.0.0 | test(audit): validate Go tests, bash syntax, and doctor diagnostics post v2.0.0 | 2026-09-24 |
| [99.1] | Preparar Release v2.0.0 (Tag Git, Changelog e Binários) e Sanitizar Contexto | chore: release v2.0.0 | 2026-09-24 |
| [00.2] | Atualizar Diretrizes de Agente (AGENTS.md) com Stack Go v2.0 e Fundações para Agregador/UI | docs(agents): update stack to Go v2.0 and add aggregator guidelines | 2026-09-24 |

---

## Backlog (Próximas, em ordem)

### Fase 1: Gateway de IA Multi-Contas & Refinamento de Cotas (Épico 14)
- [x] **[14.3]** Roteador Multi-Contas com Algoritmos de Distribuição (Smart Priority, Round Robin) e Auto-Failover em HTTP 429/403 entre Perfis
- [x] **[14.4]** Gateway Anthropic-Compatible (`/v1/messages`) e Mapeamento de Modelos (Claude Sonnet/Opus ↔ Gemini 3.5/3.6)

### Fase 2: Orquestrador de Agentes — Worktrees, PTYs & Task Dispatcher (Épico 15)
- [x] **[15.1]** Gerenciador de Git Worktrees Efêmeros por Agente/Tarefa (`internal/worktree`)
- [x] **[15.2]** Multiplexador de Terminais PTY e Execução Headless de Agentes CLI (Claude Code, Aider, OpenCode) com Isolamento de Identidade
- [x] **[15.3]** Motor de Despacho de Tarefas (`multigravity dispatch`) com Associação de Perfil, Worktree e Captura de Logs
- [x] **[15.4]** Visualizador e API de Diffs / Status de Execução de Tarefas no `multigravity serve` para futura GUI Desktop (Tauri/Wails)

### Backlog Geral de Evolução do Core
- [x] **[08.1]** Detecção e Mapeamento de Workspaces e Repositórios Ativos por Perfil
- [x] **[08.2]** Invocação e Gestão de Agentes Headless em Background com Isolamento de Identidade
- [ ] **[08.3]** Sistema de Snapshots e Rollback Seguro de Perfis e Conversas
- [ ] **[09.1]** Histórico Temporal de Consumo de Cotas e Métricas de Tokens por Perfil
- [ ] **[09.2]** Streaming de Logs em Tempo Real (Live Log Stream do Language Server e Processos da IDE)
- [ ] **[09.3]** Sistema de Alertas Proativos e Notificações de Eventos (Cota Crítica, Quedas, Processos Órfãos)
- [ ] **[10.1]** Catálogo Centralizado e Diagnóstico de Saúde de Ferramentas (MCP Hub e Skills Registry)

---

## Backlog Futuro / Ideias (não priorizadas)

- [ ] **[12.1]** Extensão Companion In-Editor para Antigravity IDE: Monitor de Cotas na StatusBar e Painel Visual via Daemon Local (`multigravity serve`)
  - *Diretriz de Execução:* **Desenvolver e testar obrigatoriamente em branch dedicada** (ex: `feat/in-editor-companion` ou `experiment/in-editor-companion`), mantendo a branch principal (`main`) livre de dependências de tooling TypeScript/VSIX até validação funcional completa.
- [x] **[13.1]** Autenticação Direta Headless via CLI (`multigravity login <profile>` com Google OAuth2 PKCE e callback HTTP efêmero)
- [x] **[13.2]** Despacho Concorrente de Tarefas e Subagentes (`multigravity exec [profile|--all] "<prompt>"`) com pooling paralelo de cotas
- [ ] **[13.3]** Importador e Migração de Ferramentas Comunitárias Legadas (`cockpit-tools` em `~/.antigravity_cockpit/accounts/`)
- [ ] **[99.1]** Preparar Release (Tag Git) e Sanitizar Contexto (Apenas executar com permissão explícita do usuário)




