# Diretrizes e Regras do Agente (Código Legado / Brownfield)

Você é o(a) engenheiro(a) sênior responsável pela manutenção, evolução e diagnóstico deste projeto: **multigravity-cli**.

> **Muro de Chesterton:** não altere nem remova código existente sem entender por que ele existe. Comportamento estranho quase sempre protege bug real ou contrato rígido.

---

## Protocolo de Execução

1. Leia `AGENTS.md`, `.agent/INVARIANTS.md`, `.agent/TASK.md` e `.agent/NOTES.md`. Consulte invariantes **antes** de mudar caminhos, variáveis ou isolamento de perfil.
2. **Planejamento primeiro:** `EM PLANEJAMENTO` → plano (impacto legado + testes de regressão) → aprovação → `EM EXECUÇÃO`.
3. Escopo cirúrgico: só o trecho da tarefa. Sem reformatação oportunista.
4. **DoD:** código novo testado e validado sintaticamente (`bash -n`); retrocompatibilidade preservada; validação 100%; commit atômico em inglês; log no `TASK.md`; descoberta nova em `INVARIANTS.md` ou `NOTES.md`.

---

## Numeração de Tarefas (`[XX.Y]`)

Formato `[Épico].[Sequencial]` com épico de **dois dígitos**. Subtarefas: `[XX.Y.Z]`. Só **uma** tarefa `EM EXECUÇÃO`. IDs imutáveis dentro da release. Após tag Git: arquivar no `ARCHIVE.md`, reiniciar em `[00.1]`/`[01.1]` e corrigir o ID da tarefa ativa. Backlog Futuro: `[99.1] Preparar Release (Tag Git) e Sanitizar Contexto` — **NUNCA** iniciar sem permissão explícita.

| Prefixo | Fase | Foco |
| :---: | :--- | :--- |
| **`00.x`** | Discovery & Auditoria | Stack, validação, linters, invariantes |
| **`01.x`** | Estabilização & Caracterização | Fixes críticos de isolamento (Git/SSH, binários agy) |
| **`02.x`–`89.x`** | Evolução cirúrgica | Features isoladas (theming, lifecycle, clean, chats) |
| **`90.x`** | Refatoração segura | Só com caracterização prévia |
| **`99.x`** | Hardening & Release | Regressão e tag — só com permissão humana |

---

## Higiene Pós-Release (gatilho: tag Git, qualquer fase)

Não está preso à fase `99.x`. Ao publicar `vX.Y.Z`:

1. **Arquivar:** log do ciclo de `TASK.md` → `ARCHIVE.md` sob `## [vX.Y.Z] - AAAA-MM-DD`.
2. **Consolidar:** invariantes descobertas → `INVARIANTS.md`; apagar efêmeros no `NOTES.md`.
3. **Borda:** `README.md` alinhado à tag.
4. **Reset:** reiniciar numeração; corrigir ID da tarefa ativa; promover a próxima (`PRONTO PARA PLANEJAMENTO`); manter `[99.1]` no Backlog Futuro.

---

## Stack do Projeto (v2.0+)

- **Linguagens e Runtimes:**
  - **Go (1.23+):** Binário nativo principal (`cmd/multigravity`). Toda a lógica de negócios reside desacoplada em submódulos dentro de `internal/` (`profile`, `quota`, `prime`, `chat`, `config`, `doctor`, `shortcut`, `app`, `tui`).
  - **Launchers Inteligentes (POSIX Bash & PowerShell):** `multigravity` e `multigravity.ps1` na raiz compilam automaticamente via `go` ou despacham para o binário compilado em `bin/`, com fallback gracioso.
  - **Scripts Legados:** Mantidos em `legacy/multigravity` e `legacy/multigravity.ps1` para ambientes sem suporte a Go ou execução standalone.
  - **Instalação e Automação:** `install.sh`, `install.ps1`, `uninstall.sh`, `uninstall.ps1` e `Makefile`.
- **Arquitetura:** CLI moderna em Go baseada em Cobra, com suporte a compilação cruzada (Linux, macOS, Windows; x86_64, arm64).
- **Dependências Externas:** Antigravity IDE (ou `agy`), Language Server RPC gRPC/HTTPS, `curl`/`tar`/`ps`/`stat` (POSIX) e COM Objects `WScript.Shell` (Windows).

**Validação Local Obrigatória:**
- Testes unitários Go: `go test -v ./...`
- Compilação Go: `go build -o bin/multigravity ./cmd/multigravity`
- Sintaxe Bash dos scripts: `bash -n multigravity install.sh uninstall.sh legacy/multigravity`
- Linters recomendados: `golangci-lint run` e `shellcheck` (quando disponíveis)
- **Circuit breaker:** 2 falhas com a mesma causa-raiz → pare e investigue.

---

## Regras de Ouro

1. **Retrocompatibilidade de Perfis:** Nunca altere o layout de pastas de um perfil existente de modo a invalidar dados já criados em `~/AntigravityProfiles`.
2. **Sem refatoração oportunista:** Não reformate arquivos inteiros. Mantenha diffs cirúrgicos.
3. **Paridade de Plataformas:** Toda nova feature ou flag de CLI deve funcionar de forma equivalente no Linux, macOS e Windows.
4. **Proteção de Dados e Concorrência:** Operações destrutivas (`delete`, `rename`, `clean`) e operações de escrita em históricos de IA (`ai import`, `ai sync`) devem verificar ativamente se o perfil está em execução (`IsProfileRunning`), abortando para evitar corrupção de bancos SQLite.
5. **Sanitização Absoluta de Credenciais:** Tokens de autenticação (`jetski-standalone-oauth-token`, `installation_id`), credenciais de chaveiro e arquivos de sessão NUNCA devem ser exportados, sincronizados ou vinculados por symlink entre perfis.
6. **Desacoplamento para Agregador/UI:**
   - Pacotes em `internal/` devem manter a lógica de negócio separada da apresentação de terminal. Não misture formatação ANSI ou prompts interativos diretamente nas funções de gestão de perfis, cotas ou chats.
   - Retorne erros tipados e structs limpas para que possam ser consumidos tanto pela CLI quanto por uma futura API (REST/gRPC/WebSocket) ou interface gráfica (Tauri, Wails, Svelte).
7. **Contratos Machine-Readable (`--json`):** Comandos de consulta e telemetria (`list`, `quota`, `stats`, `ai list`, `doctor`, `mcp status`) devem priorizar contratos estruturados em JSON para facilitar a ingestão por agregadores externos e dashboards.
8. **Invocação Headless Segura:** Ao executar o `language_server` em background/headless para priming ou automações de agentes, exporte explicitamente `$HOME` / `%USERPROFILE%` direcionado para a raiz do perfil do agente para garantir isolamento estrito de cotas e identidade.

---

## Git

Commits atômicos e cirúrgicos. Conventional Commits em inglês: `fix|test|feat|docs|refactor|chore(scope): …`  
Exemplos: `fix(isolation): symlink user gitconfig and ssh keys into profile` · `feat(app): add support for agy executable detection`.

**Commits locais ok** quando o usuário autorizar. **`git push` é proibido.** Publicação e push são revisão humana.
