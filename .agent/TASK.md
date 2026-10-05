# TASK.md — Tarefas e Roadmap para Código Legado

> Define O QUE precisa ser feito. Reescrito/atualizado no início de cada nova tarefa.

---

## Tarefa Ativa

Nenhuma tarefa em `EM EXECUÇÃO`. A próxima é **[00.3]**.

---

## Fila deste ciclo

Ordem fixa. Uma tarefa por vez. A `[00.1]` saiu da fila em 2026-10-04, sem implementação. A `[00.2]` saiu em 2026-10-04.

| ID | Estado | Título |
|---|---|---|
| [00.3] | `PRONTO PARA PLANEJAMENTO` | Arquivar NOTES já consolidadas |

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
| [90.1] | Registro único das rotas HTTP | working tree | 2026-10-04 |
| [00.2] | Lacunas de teste do gateway | working tree | 2026-10-04 |
| [02.1] | Árvore de decisão dos runners | working tree | 2026-10-04 |

---

## Backlog Futuro / Ideias (não priorizadas)

- [ ] **[99.1]** Preparar Release (Tag Git) e Sanitizar Contexto (Apenas executar com permissão explícita do usuário)

A extensão companion (`[12.1]`, branch `feat/in-editor-companion`) saiu do backlog em 2026-10-04. `main` segue sem `extensions/companion`.
