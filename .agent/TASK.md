# TASK.md — Tarefas e Roadmap para Código Legado

> Define O QUE precisa ser feito. Reescrito/atualizado no início de cada nova tarefa.

---

## Tarefa Ativa

Nenhuma tarefa em `EM EXECUÇÃO`. A próxima é **[03.2]**.

---

## Fila deste ciclo

Ordem fixa. Uma tarefa por vez. A `[00.1]` saiu da fila em 2026-10-04, sem implementação. A `[00.2]` saiu em 2026-10-04. A `[00.3]` saiu em 2026-10-14.

| ID | Estado | Título |
|---|---|---|
| [03.1] | `CONCLUÍDO` | Driver nativo do Codex CLI em dispatch e agent (`--agent codex`, sandbox e `-C`) |
| [03.2] | `PRONTO PARA PLANEJAMENTO` | Comando `multigravity dispatch apply <task-id>` para mesclar worktrees aprovados |
| [03.3] | `BACKLOG` | Sandboxing de processos de agentes via wrapper `agent-guard wrap --repo` |
| [03.4] | `BACKLOG` | Comando CLI de recomendação de rota de cotas agnóstico (`multigravity route`) |
| [03.5] | `BACKLOG` | Hook de verificação e auditoria no Dispatch (`--verify <cmd>`) |
| [03.6] | `BACKLOG` | Injeção de contexto agnóstica de governança em `dispatch run` (`AGENTS.md` / `TASK.md`) |


### [03.1] Driver nativo do Codex CLI em dispatch e agent

- **Objetivo:** registrar `--agent codex` como runner oficial de agentes em `internal/dispatch/` e `internal/agent/`.
- **Comportamento:**
  - Monta invocação automática de `codex` utilizando `-C <target_repo>` para isolamento de diretório.
  - Injeta política de sandbox nativa do Codex (`-s workspace-write` ou `--sandbox workspace-write`).
  - Permite execução headless autônoma via `codex exec` com prompt compilado da tarefa cápsula.
  - Integração transparente com `--new-worktree` para criar e vincular branch efêmera gerenciada pelo `multigravity worktree`.
- **Testes:** testes unitários em `internal/dispatch/` validando montagem correta de argumentos e flags do comando `codex`.
- **DoD:** `multigravity dispatch run <profile> --agent codex --cwd <path> --prompt "..."` executa e monitora o Codex em sessão isolada.

### [03.2] Comando `multigravity dispatch apply <task-id>` para mesclagem de worktrees

- **Objetivo:** fornecer paridade com `codex apply`, permitindo que um diff inspecionado e aprovado em um worktree efêmero (`multigravity dispatch diff <id>`) seja aplicado limpo e com segurança no repositório principal.
- **Comportamento:**
  - Valida que a tarefa está com status `completed` e o diff foi auditado.
  - Executa aplicação controlada do patch (`git apply` ou cherry-pick/squash) na working tree do repositório base.
  - Remove o worktree efêmero após a aplicação bem-sucedida (`multigravity worktree prune`).
- **Testes:** teste de integração simulando criação de diff em worktree e aplicação no repositório base.
- **DoD:** comando `multigravity dispatch apply <task-id>` aplica o diff e limpa o worktree.

### [03.3] Sandboxing de processos de agentes via wrapper `agent-guard wrap --repo`

- **Objetivo:** garantir contenção de escopo para agentes e custom runners que não possuem sandbox nativo no executável.
- **Comportamento:**
  - Quando `--repo <path>` ou `--sandbox` for especificado para agentes customizados ou Claude/Aider, injeta `agent-guard wrap --role developer --repo <path> -- <cmd>`.
  - Bloqueia mutações em arquivos fora da árvore do repositório target no nível de subshell.
- **Testes:** teste de montagem de comando com wrapper e flags de ambiente.
- **DoD:** execução interceptada bloqueia comandos destrutivos fora do CWD.

### [03.4] Comando CLI de recomendação de rota de cotas agnóstico (`multigravity route`)

- **Objetivo:** fornecer recomendação determinística do melhor perfil e modelo para execução com base no consumo atual de cotas e janelas de reset.
- **Comportamento:**
  - Novo comando `multigravity route [--weight light|medium|heavy] [--json]`.
  - Avalia os buckets dos perfis ativos (Gemini semanal/5h, 3P), selecionando o perfil ideal sem exigir que clientes externos (ex.: `taskctl`, scripts, extensões) parseiem dados brutos de quota.
  - Saída JSON estruturada com `profile`, `model`, `remaining_fraction`, `reset_time` e `reason`.
- **Testes:** testes unitários em `internal/quota/` ou `cmd/` validando a ordenação e seleção de perfis sob diferentes cenários de depleção.
- **DoD:** `multigravity route --json` retorna alocação ótima com status code 0.

### [03.5] Hook de verificação e auditoria no Dispatch (`--verify <cmd>`)

- **Objetivo:** permitir que o `multigravity dispatch run` execute um comando de validação de qualidade (ex.: `taskctl audit`, linter ou testes) dentro da worktree antes de marcar a tarefa como pronta.
- **Comportamento:**
  - Nova flag `--verify <command>` em `multigravity dispatch run`.
  - Executa `<command>` dentro do diretório de trabalho/worktree após o término do agente.
  - Se a verificação falhar (exit code != 0), marca a tarefa com status `needs_review` ou `audit_failed` e anexa o output da verificação nos logs/detalhes da tarefa.
  - Suporte a configuração default em `~/.multigravity/config.json` para auto-verificação caso configurado.
- **Testes:** teste de ciclo de dispatch verificando que `--verify` executa no diretório correto e atualiza o status de acordo com o exit code.
- **DoD:** `multigravity dispatch run ... --verify "taskctl audit"` valida o diff na worktree efêmera.

### [03.6] Injeção de contexto agnóstica de governança em `dispatch run`

- **Objetivo:** garantir que agentes autônomos executados via `dispatch run` respeitem contratos locais de governança (`AGENTS.md`, `.agent/TASK.md`, `CLAUDE.md`) do repositório target.
- **Comportamento:**
  - Quando `--repo <path>` ou CWD contiver arquivos canônicos de governança de agentes, injeta automaticamente uma diretriz de preâmbulo no prompt ou nas instruções de sistema do runner:
    `"Notice: Target repository enforces governance contracts in AGENTS.md / .agent/TASK.md. Comply with Definition of Done and scope limits."`
  - Totalmente agnóstico: detecta a presença de arquivos padrão sem acoplamento rígido de código com nenhum orquestrador específico.
  - Pode ser desativado via flag `--no-governance-context`.
- **Testes:** teste de montagem de prompt em `internal/dispatch/` verificando a inclusão do preâmbulo quando o arquivo existir no repositório.
- **DoD:** prompts montados para runners incluem o contexto de governança de forma não-intrusiva.


### [00.2] Lacunas de teste do gateway

- **Objetivo:** listar contratos do gateway sem teste e cobrir só esses buracos. `thinkingConfig`, failover 429/403 e cota desconhecida já têm testes em `gateway_test.go`, `router_test.go` e `anthropic_test.go`.
- **Impacto legado:** testes novos em `internal/gateway`. Produção intacta, salvo um defeito que o teste novo revele — aí o fix entra nesta mesma tarefa, com o teste como rede.
- **Testes:** `go test ./internal/gateway/...` verde antes e depois. Nenhum teste existente reescrito para “caber” no novo.
- **DoD:** uma nota curta em `NOTES.md` com a lista do que estava descoberto e o nome do teste que passou a cobrir. Buraco inexistente não ganha teste duplicado.
- **Fora de escopo:** refatorar `router.go` ou `anthropic.go`.

### [90.1] Registro único das rotas HTTP

- **Pré-condição:** `[00.2]` concluída. Caracterização já feita: `setupRoutes` em `internal/server/routes.go` registra cada handler em `/api/v1/...` e de novo em `/api/...` (100 `HandleFunc`). Os dois prefixos são contrato (`NOTES.md`, skill, clientes).
- **Objetivo:** um helper registra o par de prefixos. O arquivo pode ser partido por domínio (perfil, cota, dispatch, gateway) com os mesmos handlers.
- **Impacto legado:** paths, métodos, status e corpos JSON permanecem. Alias `/api` permanece. SSE, gateway `/v1/chat/completions` e `/v1/messages` permanecem.
- **Testes:** `go test ./internal/server/...` sem teste novo de comportamento, mais um teste de tabela que afirma que cada path `/api/v1` tem o gêmeo `/api` no mux. `go test ./...` antes de encerrar.
- **DoD:** nenhum `HandleFunc` duplicado à mão para o par de prefixos; suíte do server verde.
- **Fora de escopo:** apagar o alias `/api`, mudar payloads, autenticar o daemon.

### [00.3] Arquivar NOTES já consolidadas

- **Pré-condição:** `[90.1]` concluída, para o relatório dessa tarefa ainda ter onde ser escrito.
- **Objetivo:** tirar de `NOTES.md` as entradas cujo contrato já está em `INVARIANTS.md`. O destino é uma seção datada em `ARCHIVE.md`.
- **Impacto legado:** agentes passam a ler um `NOTES.md` curto. Invariante que existir só em NOTES migra para `INVARIANTS.md` antes do corte.
- **Testes:** diff revisado entrada a entrada. Nenhuma invariante fica só no arquivo arquivado.
- **DoD:** `NOTES.md` guarda decisões ainda sem contrato estável. `ARCHIVE.md` recebe o lote. `INVARIANTS.md` segue a fonte do que não pode mudar.
- **Fora de escopo:** reescrever invariantes, apagar `NOTES.md` por inteiro.

---

## Log de Tarefas Concluídas

> Histórico anterior arquivado em `ARCHIVE.md` sob `[v2.2.1] - 2026-09-29`.

| Tarefa | Título | Commit(s) | Data |
|---|---|---|---|
| [03.1] | Driver nativo do Codex CLI em dispatch e agent | e8e0547 | 2026-10-14 |
| [00.3] | Arquivar NOTES já consolidadas | working tree | 2026-10-14 |
| [90.1] | Registro único das rotas HTTP | working tree | 2026-10-04 |
| [00.2] | Lacunas de teste do gateway | working tree | 2026-10-04 |
| [02.1] | Árvore de decisão dos runners | working tree | 2026-10-04 |

---

## Backlog Futuro / Ideias (não priorizadas)

- [ ] **[99.1]** Preparar Release (Tag Git) e Sanitizar Contexto (Apenas executar com permissão explícita do usuário)

A extensão companion (`[12.1]`, branch `feat/in-editor-companion`) saiu do backlog em 2026-10-04. `main` segue sem `extensions/companion`.
