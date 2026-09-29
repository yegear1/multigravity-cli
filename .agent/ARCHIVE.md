# ARCHIVE.md — Arquivo Histórico de Tarefas Concluídas

> Lotes arquivados após tag Git (ou quando o log do `TASK.md` passar de ~15 linhas).
> Cabeçalho canônico: `## [vX.Y.Z] - AAAA-MM-DD`. Detalhe: `git log`.

## [v2.2.1] - 2026-09-29

| Tarefa | Título | Commit(s) | Data |
|---|---|---|---|
| [99.1] | Preparar Release v2.2.1 (Tag Git, Changelog e Binários) e Sanitizar Contexto | 345fedf | 2026-09-29 |
| [01.1] | Resolução de atritos de DX em dispatch, agent run, exec e telemetria de cota | 51e56fd | 2026-09-29 |

## [v2.2.0] - 2026-09-28

| Tarefa | Título | Commit(s) | Data |
|---|---|---|---|
| [99.1] | Preparar Release v2.2.0 (Tag Git, Changelog e Binários) e Sanitizar Contexto | chore: release v2.2.0 | 2026-09-28 |
| [08.1] | Correção de colisão de nomes em detecção de processos, git no doctor, timeout em exec e alias --force | d238eba | 2026-09-28 |
| [07.1] | Suporte a flag --model (-m) em multigravity exec, headless run e REST API | 423b8ba | 2026-09-28 |
| [06.1] | Despachar plano com --repo sem trocar a workspace quando o alvo não tem origin | a4576df | 2026-09-27 |
| [05.3] | dispatch_task aceita args e prompt no schema MCP e handler de despacho | b023952 | 2026-09-27 |
| [05.2] | Diff de tarefa inclui arquivos untracked via git ls-files e git diff --no-index | a932a2f | 2026-09-27 |
| [05.1] | Omitir CSRF token da serialização JSON em cotas (ActiveServer), MCP e REST API | 5625845 | 2026-09-27 |
| [04.4] | Expor ferramenta dispatch_plan no servidor MCP, registrar multigravity no mcp_config.json e instalar agy CLI | fabedb6 | 2026-09-27 |
| [04.3] | Endpoint de Subtask Aggregator na API REST (/api/v1/dispatch/plans) | cbbbd40 | 2026-09-27 |
| [04.2] | Skill de Orquestração Multi-Agente (skills/multigravity-orchestrator) | fd8117c | 2026-09-27 |
| [04.1] | Servidor MCP nativo stdio/HTTP e catálogo de ferramentas | c4de524 | 2026-09-27 |
| [03.4] | Skill e READMEs registram o contrato de modelo do gateway | 750b514 | 2026-09-27 |
| [03.3] | Gateway envia thinkingLevel no Flash 3.7 e 3.8 tiered | 398121f | 2026-09-27 |
| [03.2] | Catálogo do gateway usa os ids ao vivo do menu do agente | 2fee716 | 2026-09-27 |
| [03.1] | Gateway usa o cofre OAuth do perfil e cota medida no roteador | f1e859e, ccc105a | 2026-09-27 |
| [00.1] | Alinhar `AGENTS.md` à stack Go 1.27.1, aos pacotes da v2.1 e à identidade do repositório | 3ee2852 | 2026-09-26 |
| [02.1] | Expandir comandos read-only canônicos (Linux, uv, pnpm, ruff, pyright, eslint, docker local) e semear perfis | 3ee2852 | 2026-09-26 |
| [00.2] | Alinhar documentações (READMEs, SKILL.md, landing page e launchers) à v2.1 | 15a6c00 | 2026-09-26 |

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
