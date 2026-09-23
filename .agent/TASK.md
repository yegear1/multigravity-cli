# TASK.md — Tarefas e Roadmap para Código Legado

> Define O QUE precisa ser feito. Reescrito/atualizado no início de cada nova tarefa.

---

## Tarefa Ativa
 
- [ ] Nenhuma tarefa em execução. Pronto para próximo ciclo ou release.


 
---
 
## Log de Tarefas Concluídas
 
> Histórico anterior arquivado em `ARCHIVE.md` sob `[v1.5.0] - 2026-09-13`.
 
| Tarefa | Título | Commit(s) | Data |
|---|---|---|---|
| [90.1] | Setup da branch `feat/go-rewrite`, toolchain Go e inicialização do projeto Go com Cobra CLI | feat(go) | 2026-09-23 |
| [02.3] | Documentar Mapeamento Canônico de Diretórios do Antigravity em `INVARIANTS.md` | docs(invariants) | 2026-09-23 |
| [02.2] | Injetar comandos read-only padrão (git, posix, npm, pnpm, uv) em "Always allow" (`config.json`) | feat(config) | 2026-09-23 |
| [02.1] | Criar Skill Canônica do Multigravity para Agentes de IA (`skills/multigravity/SKILL.md`) | 375dd1c | 2026-09-15 |
| [01.3] | Fix do crash no menu interativo TUI (get_profile_color command not found) (fixes #1) | 55fb258 | 2026-09-14 |
| [00.0] | Release v1.5.0 (AI Quota Telemetry, Prime Automation, Permissions & GH Sync) | v1.5.0 | 2026-09-13 |
| [02.6] | Suporte a Auto-Priming de Limites de 5 Horas (gemini-5h e 3p-5h) e Checagem Pré-Prime | feat(prime) | 2026-09-13 |

---

## Backlog (Próximas, em ordem)

- [ ] Promover próxima feature do backlog

---

## Backlog Futuro / Ideias (não priorizadas)

- [ ] **[99.1]** Preparar Release (Tag Git) e Sanitizar Contexto (Apenas executar com permissão explícita do usuário)
