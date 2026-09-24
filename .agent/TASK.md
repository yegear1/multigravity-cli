# TASK.md — Tarefas e Roadmap para Código Legado

> Define O QUE precisa ser feito. Reescrito/atualizado no início de cada nova tarefa.

---

## Tarefa Ativa

- [ ] **[90.4] [PRONTO PARA PLANEJAMENTO] Portar Lançamento de Perfis (`<name> [args...]`), Detecção de Executável (`antigravity`/`agy`) e Atalhos de Desktop (.desktop, .app, .lnk)**

---

## Log de Tarefas Concluídas

> Histórico anterior arquivado em `ARCHIVE.md` sob `[v1.5.0] - 2026-09-13`.

| Tarefa | Título | Commit(s) | Data |
|---|---|---|---|
| [90.3] | Implementação de Ciclo de Vida (`stop`, `restart`) e Limpeza de Caches (`clean`) em Go | feat(go) | 2026-09-24 |
| [90.2] | Implementação dos Comandos de Gestão de Perfis em Go (`new`, `delete`, `rename`) | feat(go) | 2026-09-24 |
| [90.1] | Setup da branch `feat/go-rewrite`, toolchain Go e inicialização do projeto Go com Cobra CLI | feat(go) | 2026-09-23 |
| [02.3] | Documentar Mapeamento Canônico de Diretórios do Antigravity em `INVARIANTS.md` | docs(invariants) | 2026-09-23 |
| [02.2] | Injetar comandos read-only padrão (git, posix, npm, pnpm, uv) em "Always allow" (`config.json`) | feat(config) | 2026-09-23 |
| [02.1] | Criar Skill Canônica do Multigravity para Agentes de IA (`skills/multigravity/SKILL.md`) | 375dd1c | 2026-09-15 |
| [01.3] | Fix do crash no menu interativo TUI (get_profile_color command not found) (fixes #1) | 55fb258 | 2026-09-14 |
| [00.0] | Release v1.5.0 (AI Quota Telemetry, Prime Automation, Permissions & GH Sync) | v1.5.0 | 2026-09-13 |
| [02.6] | Suporte a Auto-Priming de Limites de 5 Horas (gemini-5h e 3p-5h) e Checagem Pré-Prime | feat(prime) | 2026-09-13 |

---

## Backlog (Próximas, em ordem)

- [ ] **[90.5]** Portar Theming Visual (`color`) e Comandos de Compartilhamento/Isolamento Modular (`config`, `mcp`, `skills`, `gh`)
- [ ] **[90.6]** Portar Backup, Restauração e Templates (`clone`, `export`, `import`, `template`, `stats`)
- [ ] **[90.7]** Portar Telemetria de Cotas de IA (`quota`) e Automação de Priming (`prime`) via Language Server
- [ ] **[90.8]** Portar Menu Interativo TUI sem argumentos, Diagnóstico do Sistema (`doctor`) e Shell Completion
- [ ] **[90.9]** Validação de Paridade com Scripts Legados, Instalação e Troca do Ponto de Entrada Padrão

---

## Backlog Futuro / Ideias (não priorizadas)

- [ ] **[99.1]** Preparar Release (Tag Git) e Sanitizar Contexto (Apenas executar com permissão explícita do usuário)
