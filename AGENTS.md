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

## Stack do Projeto

- **Linguagens e Runtimes:**
  - **Bash (Linux / macOS):** `multigravity`, `install.sh`, `uninstall.sh`. Exige compatibilidade com Bash 4+ e POSIX sh nos scripts de launcher (`~/.local/share/multigravity/launchers/*.sh`).
  - **PowerShell (Windows):** `multigravity.ps1`, `install.ps1`, `uninstall.ps1`. Compatível com Windows PowerShell 5.1 e PowerShell 7+ (`pwsh`).
- **Arquitetura:** CLI sem build step ou empacotador binário pré-compilado. Execução direta via interpretador do sistema.
- **Sistemas Operacionais Suportados:** Linux, macOS (Darwin) e Windows.
- **Dependências Externas:** Antigravity IDE (ou `agy`), `curl`, `tar`, `ps`, `stat` (POSIX) e COM Objects `WScript.Shell` (Windows).

**Validação Local:**
- Sintaxe Bash: `bash -n multigravity install.sh uninstall.sh`
- Linters recomendados: `shellcheck` (quando disponível)
- **Circuit breaker:** 2 falhas com a mesma causa-raiz → pare e investigue.

---

## Regras de Ouro

1. **Retrocompatibilidade de Perfis:** Nunca altere o layout de pastas de um perfil existente de modo a invalidar dados já criados em `~/AntigravityProfiles`.
2. **Sem refatoração oportunista:** Não reformate arquivos inteiros. Mantenha diffs cirúrgicos.
3. **Paridade de Plataformas:** Toda nova feature ou flag de CLI no script Bash (`multigravity`) deve ter, sempre que cabível, paridade correspondente no script PowerShell (`multigravity.ps1`).
4. **Proteção de Dados:** Operações destrutivas (`delete`, `rename`, `clean`) devem exigir confirmação interativa ou flags explícitas e verificar se o processo está em execução.

---

## Git

Commits atômicos e cirúrgicos. Conventional Commits em inglês: `fix|test|feat|docs|refactor|chore(scope): …`  
Exemplos: `fix(isolation): symlink user gitconfig and ssh keys into profile` · `feat(app): add support for agy executable detection`.

**Commits locais ok** quando o usuário autorizar. **`git push` é proibido.** Publicação e push são revisão humana.
