# TASK.md — Tarefas e Roadmap para Código Legado

> Define O QUE precisa ser feito. Reescrito/atualizado no início de cada nova tarefa.

---

## Tarefa Ativa

*(Nenhuma tarefa ativa no momento)*

---

## Log de Tarefas Concluídas

> Histórico anterior arquivado em `ARCHIVE.md` sob `[v2.1.0] - 2026-09-26`.

| Tarefa | Título | Commit(s) | Data |
|---|---|---|---|
| [06.1] | Despachar plano com --repo sem trocar a workspace quando o alvo não tem origin | a4576df | 2026-09-27 |
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

---

## Backlog Futuro / Ideias (não priorizadas)

- [ ] **[12.1]** Extensão Companion In-Editor para Antigravity IDE: Monitor de Cotas na StatusBar e Painel Visual via Daemon Local (`multigravity serve`)
  - A implementação está em `feat/in-editor-companion` e ficou fora da tag v2.1.0. `main` permanece sem `extensions/companion` até o merge.
- [ ] **[99.1]** Preparar Release (Tag Git) e Sanitizar Contexto (Apenas executar com permissão explícita do usuário)
