# ARCHIVE.md — Arquivo Histórico de Tarefas Concluídas

> Lotes arquivados após tag Git (ou quando o log do `TASK.md` passar de ~15 linhas).
> Cabeçalho canônico: `## [vX.Y.Z] - AAAA-MM-DD`. Detalhe: `git log`.

## [v2.1.0] - 2026-09-26

| Tarefa | Título | Commit(s) | Data |
|---|---|---|---|
| [99.1] | Preparar Release v2.1.0 (Tag Git, Changelog e Binários) e Sanitizar Contexto | chore: release v2.1.0 | 2026-09-26 |
| [10.1] | Catálogo e diagnóstico de saúde de MCP e skills (`catalog`, `GET /catalog`) | feat(catalog): inventory MCP servers and skills health | 2026-09-26 |
| [09.2] | Stream de log da IDE gráfica (`logs`, `GET /profiles/{name}/ide/logs`) | feat(logs): stream redacted IDE main.log | 2026-09-26 |
| [08.3] | Sistema de Snapshots e Rollback Seguro de Perfis e Conversas | feat(profile): add snapshot and rollback for profiles and conversations | 2026-09-26 |
| [09.3] | Sistema de Alertas Proativos e Notificações de Eventos (Cota Crítica, Quedas, Processos Órfãos) | feat(alert): report critical quota, drops, and reaped headless processes | 2026-09-26 |
| [09.1] | Histórico Temporal de Consumo de Cotas e Métricas de Tokens por Perfil | feat(quota): record per-profile quota and token history | 2026-09-26 |
| [13.2] | Despacho Concorrente de Tarefas e Subagentes (`multigravity exec`) | feat(headless): fan out one prompt across isolated profiles | 2026-09-26 |
| [13.1] | Autenticação Direta Headless via CLI (`multigravity login` com Google OAuth2 PKCE) | feat(auth): add headless OAuth2 PKCE login into the profile vault | 2026-09-26 |
| [11.6] | Alinhamento de posicionamento, README e CHANGELOG | docs(platform): align branding, readme, and changelog | 2026-09-26 |
| [08.2] | Invocação e Gestão de Agentes Headless em Background | feat(headless): add background headless instance manager | 2026-09-26 |
| [08.1] | Detecção e Mapeamento de Workspaces e Repositórios Ativos por Perfil | feat(workspace): add workspace mapping and active repository detection | 2026-09-26 |
| [15.4] | Visualizador e API de Diffs / Status de Execução de Tarefas | feat(dispatch): add structured diff parser and execution dashboard | 2026-09-25 |
| [15.3] | Motor de Despacho de Tarefas (`multigravity dispatch`) | feat(dispatch): add task dispatcher with profile isolation and worktrees | 2026-09-25 |
| [15.2] | Multiplexador de Terminais PTY e Execução Headless de Agentes CLI | feat(agent): add pty multiplexer and headless agent execution | 2026-09-25 |
| [15.1] | Gerenciador de Git Worktrees Efêmeros (`internal/worktree`) | feat(worktree): add ephemeral git worktree manager | 2026-09-25 |
| [14.4] | Gateway Anthropic-Compatible (`/v1/messages`) | feat(gateway): add anthropic messages gateway with sse | 2026-09-25 |
| [14.3] | Roteador Multi-Contas com Auto-Failover | feat(gateway): implement multi-account router with auto-failover | 2026-09-25 |
| [14.2] | Gateway de Completions OpenAI-Compatible (`/v1/chat/completions`) | feat(gateway): add openai completions gateway with sse streaming | 2026-09-25 |
| [14.1] | Heurística de Janela de Cotas (5h vs Semanal) e Ping de Aquecimento | feat(quota): add window heuristic and proactive 5h warm-up prime | 2026-09-25 |
| [07.3] | Priming e Aquecimento de Cotas via API com Emissão de Progresso | feat(prime): add priming API endpoints, SSE progress, and CLI --json | 2026-09-25 |
| [07.2] | Mutação de Compartilhamento Dinâmico via API | feat(server): add sharing mutation endpoints and dynamic resource toggling | 2026-09-25 |
| [07.1] | Gerenciamento Completo de Perfis via API | feat(server): implement mutation endpoints for profile creation, deletion, and lifecycle | 2026-09-25 |
| [11.5] | Landing Page Estática e Onboarding Visual | feat(docs): create interactive landing page and onboarding showcase | 2026-09-25 |
| [11.4] | Matriz Comparativa Full vs Auth-Only | docs(readme): add full vs auth-only comparison matrix | 2026-09-25 |
| [11.3] | Ícone Embutido e Associação em Atalhos Desktop | feat(shortcut): embed default icons via go:embed | 2026-09-25 |
| [11.2] | Contrato Machine-Readable (`--json`) em `multigravity status` | feat(status): add --json flag with byte-level sizing | 2026-09-25 |
| [11.1] | Perfis Auth-Only (`--auth-only` / `--shared`) | feat(profile): add --auth-only flag with host extensions symlinks | 2026-09-25 |
| [90.1] | Remoção do diretório `legacy/` após isolar scripts na branch | refactor(build): isolate legacy scripts to branch and remove legacy directory | 2026-09-24 |
| [06.1] | Detecção de Chats de IA em Uso e Sync Granular Seguro | feat(ai): detect in-use chats via OS handles with safe granular sync | 2026-09-24 |
| [05.3] | Tamanho de Conversas em `ai list` | feat(ai): add size and size_bytes tracking for AI conversations | 2026-09-24 |
| [05.2] | Estado Arquivado/Ativo e Tópicos em `ai list` | feat(ai): detect archived and active chats with transcript topic extraction | 2026-09-24 |
| [05.1] | Correção de Detecção de Executável (FindApp) | fix(app): enforce error on invalid app override and fix install home resolution | 2026-09-24 |
| [04.1] | Streaming SSE no Servidor HTTP | feat(server): add Server-Sent Events (SSE) streaming endpoint | 2026-09-24 |
| [03.1] | Servidor HTTP local e skill canônica para o agregador | feat(server): add local HTTP REST server and expand canonical skill | 2026-09-24 |
| [02.1] | Contratos `--json` nos comandos de consulta | feat(cli): add --json output flag to list, stats, doctor, quota, ai list, and mcp status | 2026-09-24 |
| [00.1] | Diagnóstico e Auditoria de Integridade Pós-Release v2.0.0 | test(audit): validate Go tests, bash syntax, and doctor diagnostics post v2.0.0 | 2026-09-24 |
| [00.2] | Diretrizes de Agente com stack Go v2.0 | docs(agents): update stack to Go v2.0 and add aggregator guidelines | 2026-09-24 |

## [v2.0.0] - 2026-09-24

| Tarefa | Título | Commit(s) | Data |
|---|---|---|---|
| [99.1] | Preparar Release v2.0.0 (Tag Git, Changelog e Binários) e Sanitizar Contexto | chore(release) | 2026-09-24 |
| [90.9] | Validação de Paridade com Scripts Legados, Instalação e Troca do Ponto de Entrada Padrão | f72b027 | 2026-09-24 |
| [90.8] | Portar Menu Interativo TUI sem argumentos, Diagnóstico do Sistema (`doctor`) e Shell Completion | 8ac1bad | 2026-09-24 |
| [90.7] | Portar Telemetria de Cotas de IA (`quota`), Automação de Priming (`prime`) e Gestão de Chats (`ai`) via Language Server | 7a62868 | 2026-09-24 |
| [90.6] | Portar Backup, Restauração e Templates (`clone`, `export`, `import`, `template`, `stats`) | feat(go) | 2026-09-24 |
| [90.5] | Portar Theming Visual (`color`) e Comandos de Compartilhamento/Isolamento Modular (`config`, `mcp`, `skills`, `gh`) | 37895ae | 2026-09-24 |
| [90.4] | Implementação de Lançamento de Perfis, Detecção de Executável (antigravity/agy) e Atalhos de Desktop em Go | feat(go) | 2026-09-24 |
| [90.3] | Implementação de Ciclo de Vida (`stop`, `restart`) e Limpeza de Caches (`clean`) em Go | feat(go) | 2026-09-24 |
| [90.2] | Implementação dos Comandos de Gestão de Perfis em Go (`new`, `delete`, `rename`) | feat(go) | 2026-09-24 |
| [90.1] | Setup da branch `feat/go-rewrite`, toolchain Go e inicialização do projeto Go com Cobra CLI | feat(go) | 2026-09-23 |
| [02.3] | Documentar Mapeamento Canônico de Diretórios do Antigravity em `INVARIANTS.md` | docs(invariants) | 2026-09-23 |
| [02.2] | Injetar comandos read-only padrão (git, posix, npm, pnpm, uv) em "Always allow" (`config.json`) | feat(config) | 2026-09-23 |
| [02.1] | Criar Skill Canônica do Multigravity para Agentes de IA (`skills/multigravity/SKILL.md`) | 375dd1c | 2026-09-15 |
| [01.3] | Fix do crash no menu interativo TUI (get_profile_color command not found) (fixes #1) | 55fb258 | 2026-09-14 |
| [02.6] | Suporte a Auto-Priming de Limites de 5 Horas (gemini-5h e 3p-5h) e Checagem Pré-Prime | feat(prime) | 2026-09-13 |

---

## [v1.5.0] - 2026-09-13

| Tarefa | Título | Commit(s) | Data |
|---|---|---|---|
| [02.1] | Compartilhamento e Sincronização Automática de config.json e Permissões | 93da4c1 | 2026-09-13 |
| [02.2] | Telemetria e Monitoramento de Cotas e Tokens (multigravity quota) | eb6a208 | 2026-09-13 |
| [02.3] | Automação de Reset de Cota Semanal (multigravity prime & watchdog) | a7efd2b | 2026-09-13 |
| [02.4] | Prime Dual-Bucket (Gemini + Claude/GPT), prompts.json Externo e Jitters Independentes | a2224d0 | 2026-09-13 |
| [02.5] | Sincronização de credenciais GitHub CLI (gh), git-credentials e herança de PATH de usuário | c722286 | 2026-09-13 |

---

## [v1.4.1] - 2026-09-13

| Tarefa | Título | Commit(s) | Data |
|---|---|---|---|
| [02.1] | Compartilhamento de MCP Servers e Sincronização Direta de Conversas de IA | 7ff3436 | 2026-09-13 |
| [02.2] | Compartilhamento de Skills e Plugins Globais | a97fc26 | 2026-09-13 |

---

## [v1.4.0] - 2026-09-12

| Tarefa | Título | Commit(s) | Data |
|---|---|---|---|
| [00.0] | Injeção do template brownfield no repositório legado | 56aecbb | 2026-09-12 |
| [00.1] | Auditoria, calibração da stack e mapeamento de invariantes | 3769548 | 2026-09-12 |
| [01.1] | Preservação de dotfiles de dev (.gitconfig e .ssh) no isolamento de perfil | 3769548 | 2026-09-12 |
| [01.2] | Suporte nativo ao executável e alias agy (Antigravity 2.0 / CLI) | 3769548 | 2026-09-12 |
| [02.1] | Theming visual por perfil via `--color` e comando `multigravity color` | 3769548 | 2026-09-12 |
| [02.2] | Comandos de ciclo de vida (`stop`, `restart`) e trava contra concorrência | 3769548 | 2026-09-12 |
| [02.3] | Otimização de backup (`export` sem caches) e comando `clean` | 3769548 | 2026-09-12 |
| [03.1] | Menu TUI / Seletor interativo ao invocar `multigravity` sem argumentos | 3769548 | 2026-09-12 |
| [03.2] | Migração e Exportação/Importação granular de chats de IA sem dados de autenticação | 3769548 | 2026-09-12 |
| [03.3] | Documentação completa do fork (README.md), URLs e adição de licença MIT | 6eaad1f | 2026-09-12 |
| [03.4] | Correção de URLs de update, expansão de caminhos do Antigravity, versão e .gitignore | 3eca151 | 2026-09-12 |
| [03.5] | Esclarecimento formal do escopo da licença MIT e direitos do código base | 35f6267 | 2026-09-12 |
