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

- [ ] **[07.1]** Gerenciamento Completo de Perfis via API (Criação, Exclusão, Renomeação e Launch de Instâncias)
- [ ] **[07.2]** Mutação de Compartilhamento Dinâmico via API (Toggle de MCP, Skills, Config, Git/GitHub)
- [ ] **[07.3]** Priming e Aquecimento de Cotas via API com Emissão de Progresso
- [ ] **[08.1]** Detecção e Mapeamento de Workspaces e Repositórios Ativos por Perfil
- [ ] **[08.2]** Invocação e Gestão de Agentes Headless em Background com Isolamento de Identidade
- [ ] **[08.3]** Sistema de Snapshots e Rollback Seguro de Perfis e Conversas
- [ ] **[09.1]** Histórico Temporal de Consumo de Cotas e Métricas de Tokens por Perfil
- [ ] **[09.2]** Streaming de Logs em Tempo Real (Live Log Stream do Language Server e Processos da IDE)
- [ ] **[09.3]** Sistema de Alertas Proativos e Notificações de Eventos (Cota Crítica, Quedas, Processos Órfãos)
- [ ] **[10.1]** Catálogo Centralizado e Diagnóstico de Saúde de Ferramentas (MCP Hub e Skills Registry)

---

## Backlog Futuro / Ideias (não priorizadas)

- [ ] **[99.1]** Preparar Release (Tag Git) e Sanitizar Contexto (Apenas executar com permissão explícita do usuário)

