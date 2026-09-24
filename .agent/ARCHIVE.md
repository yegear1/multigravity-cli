# ARCHIVE.md — Arquivo Histórico de Tarefas Concluídas

> Lotes arquivados após tag Git (ou quando o log do `TASK.md` passar de ~15 linhas).
> Cabeçalho canônico: `## [vX.Y.Z] - AAAA-MM-DD`. Detalhe: `git log`.

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
