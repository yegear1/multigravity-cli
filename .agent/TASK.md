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

*(Nenhuma tarefa pendente no ciclo ativo)*

---

## Backlog Futuro / Ideias (não priorizadas)

- [ ] **[99.1]** Preparar Release (Tag Git) e Sanitizar Contexto (Apenas executar com permissão explícita do usuário)

