# TASK.md — Tarefas e Roadmap para Código Legado

> Define O QUE precisa ser feito. Reescrito/atualizado no início de cada nova tarefa.

---

## Tarefa Ativa

### 📌 Tarefa [99.1]: Revisão Final, Documentação e Validação de Release

- **Descrição:** Realizar auditoria completa de consistência entre Bash e PowerShell, verificar `git status`, testar `doctor` e garantir que o repositório esteja pronto para publicação ou commit pelo usuário.
- **Sistema(s) Envolvido(s):** `multigravity`, `multigravity.ps1`, `README.md`, templates `.agent/`
- **Tipo de Ação:**
  - [x] Somente leitura / Documentação
  - [ ] Escrita de código-fonte
- **Status:** CONCLUÍDO / PRONTO PARA DECISÃO DO USUÁRIO

---

## Log de Tarefas Concluídas

| Tarefa | Título | Commit(s) | Data |
|---|---|---|---|
| [00.0] | Injeção do template brownfield no repositório legado | Untracked | 2026-09-12 |
| [00.1] | Auditoria, calibração da stack e mapeamento de invariantes | Pending | 2026-09-12 |
| [01.1] | Preservação de dotfiles de dev (.gitconfig e .ssh) no isolamento de perfil | Pending | 2026-09-12 |
| [01.2] | Suporte nativo ao executável e alias agy (Antigravity 2.0 / CLI) | Pending | 2026-09-12 |
| [02.1] | Theming visual por perfil via `--color` e comando `multigravity color` | Pending | 2026-09-12 |
| [02.2] | Comandos de ciclo de vida (`stop`, `restart`) e trava contra concorrência | Pending | 2026-09-12 |
| [02.3] | Otimização de backup (`export` sem caches) e comando `clean` | Pending | 2026-09-12 |
| [03.1] | Menu TUI / Seletor interativo ao invocar `multigravity` sem argumentos | Pending | 2026-09-12 |
| [03.2] | Migração e Exportação/Importação granular de chats de IA sem dados de autenticação | Pending | 2026-09-12 |

---

## Backlog (Próximas, em ordem)

- [x] Todos os itens priorizados do roadmap foram concluídos com sucesso!

---

## Backlog Futuro / Ideias (não priorizadas)

- [ ] **[99.1]** Preparar Release (Tag Git) e Sanitizar Contexto (Apenas executar com permissão explícita do usuário)
- [ ] Suporte a compartilhamento granular de MCP Servers (`--shared-mcp`)
