# NOTES.md — Decisões Rápidas e Contratos do Código Legado

> Guarda o PORQUÊ das mudanças e descobertas técnicas.
> Para restrições intocáveis e peculiaridades históricas, use preferencialmente o `.agent/INVARIANTS.md`.
> Este arquivo é para decisões tomadas durante as tarefas ativas e mapeamento de contratos vigentes.

---

## Como usar este arquivo (para o agente)

1. **Leia antes de planejar qualquer tarefa.**
2. **Registre uma nova entrada quando:**
   - Um comportamento estranho ou armadilha for resolvido/investigado.
   - Um novo contrato de dados (schema/payload) for mapeado ou expandido de forma retrocompatível.
   - Um débito técnico for assumido conscientemente durante um bugfix.
3. **Mantenha as entradas curtas e objetivas.**

---

## Decisões Técnicas Recentes

### 2026-10-14 [Task 03.1] Driver nativo do Codex CLI em dispatch e agent

- **Contexto:** Necessidade de registrar o OpenAI Codex CLI (`codex`) como runner oficial de primeira classe para agentes autônomos em `dispatch` e sessões de terminal em `agent`.
- **Decisões Técnicas e Contratos:**
  - **Detecção e Ambiente (`internal/agent`):**
    - `DetectAgentType` reconhece executáveis contendo `codex`.
    - `BuildAgentEnv` isola `CODEX_HOME` na raiz do perfil (`<profileDir>/.codex`) e injeta `OPENAI_BASE_URL` / `OPENAI_API_BASE` e `OPENAI_API_KEY` para o gateway multigravity.
  - **Montagem de Comandos (`internal/dispatch`):**
    - `ResolveAgentCommandWithCwd` monta `codex exec -C <cwd> --sandbox workspace-write "<prompt>"` para execuções headless autônomas.
    - Se o usuário fornecer flags explícitas (ex.: `--sandbox read-only`, `-C /custom`), elas são rigorosamente respeitadas e não sofrem duplicação.
    - Integração transparente com `--new-worktree`: o caminho do worktree efêmero é injetado como `-C <worktreePath>` garantindo contenção ao worktree.
  - **Sessões Interativas PTY (`internal/cmd/agent`):**
    - `multigravity agent run <profile> --agent codex` defaults o comando para `codex` e injeta `-C <cwd>` e `--sandbox workspace-write` de forma não intrusiva.
    - Schemas de ferramentas MCP `dispatch_task` e `dispatch_plan` atualizados com suporte a `'codex'`.

### 2026-10-14 [AI Archival & Project Mapping] Mecânica Interna de Arquivamento/Desarquivamento e Resolução de Project IDs

- **Contexto:** Ao desarquivar conversas ou sincronizar históricos entre perfis (`multigravity ai sync`), as conversas não apareciam na interface gráfica do ADE (Antigravity IDE), permanecendo ocultas ou não associadas ao projeto ativo.
- **Descobertas Técnicas e Contratos da IDE:**
  - **Tripla Camada de Estado de Conversas:**
    1. **SQLite (`conversations/<cid>.db`):** Tabela `trajectory_metadata_blob` armazena o protobuf com `projectId` (UUID v4 de 36 caracteres) e `workspaceUris`.
    2. **Anotações em Texto (`annotations/<cid>.pbtxt`):** Contém `title`, `archived: true/false`, `archival_status_timestamp: {seconds: ... nanos: ...}` e `last_user_view_time`.
    3. **Cache Binário de Sumários (`agyhub_summaries_proto.pb`):** Protobuf binário na raiz de `~/.gemini/antigravity/` lido pelo Language Server na inicialização (`--subclient_type hub`). Mantém o sumário unificado que alimenta o stream reativo `JetboxSubscribeToSummaries` (`exa.language_server_pb.LanguageServerService/JetboxSubscribeToSummaries`) consumido pelo frontend Electron.
  - **Armadilha do Arquivamento Estático:**
    - Modificar apenas `archived: false` no arquivo `.pbtxt` no disco é insuficiente quando a IDE/Language Server está em execução ou possui o cache binário carregado. O Language Server continua servindo os dados do cache interno (`agyhub_summaries_proto.pb`) onde `archived: true` ainda está gravado.
  - **API Canônica ConnectRPC do Language Server:**
    - O Language Server expõe RPCs na porta local da IDE (com autenticação via header `X-Codeium-Csrf-Token: <csrf_token>`, onde o token é lido do processo `--csrf_token <uuid>`).
    - **Endpoint canônico:** `POST /exa.language_server_pb.LanguageServerService/UpdateConversationAnnotations`.
    - **Payload JSON:** `{"cascadeId": "<cid>", "annotations": {"archived": false}}` (ou `true` para arquivar).
    - **Efeito:** Atualiza atomicamente a memória do Language Server, atualiza `agyhub_summaries_proto.pb`, grava o `.pbtxt` em disco e emite evento reativo no stream da UI, fazendo a conversa reaparecer imediatamente no ADE sem necessidade de reinicialização.
  - **Mapeamento de Project IDs na Sincronização entre Perfis:**
    - Cada perfil gera IDs próprios para seus projetos em `~/.gemini/config/projects/<project_id>.json`.
    - Ao sincronizar conversas de um perfil fonte (ex: `yegear`) para outro (ex: `luisfmb`), as conversas trazem o `projectId` antigo no blob SQLite e no `agyhub_summaries_proto.pb`. Sem o alinhamento desse ID para o ID do projeto no perfil de destino, as conversas ficam como "órfãs" e a IDE não as agrupa sob a pasta/projeto correto no sidebar.
    - Como ambos os Project IDs são UUIDs v4 de exatamente 36 caracteres ASCII (comprimento idêntico), a substituição de bytes preserva com 100% de integridade o wire format dos protobufs no SQLite e no `.pb`.
- **Diretrizes para Implementação de Comandos CLI:**
  - `multigravity ai archive <profile> <cid> [--project <proj>]`
  - `multigravity ai unarchive <profile> <cid> [--project <proj>]`
    - Quando o perfil estiver em execução: Descobrir porta e CSRF token do Language Server e chamar `UpdateConversationAnnotations` via ConnectRPC.
    - Quando o perfil estiver offline: Atualizar tanto o `annotations/<cid>.pbtxt` quanto os bytes de `annotations.archived` no `agyhub_summaries_proto.pb`.
  - `multigravity ai sync`:
    - Adicionar etapa opcional ou automática de mapeamento de Project IDs para o perfil destino, e merge/invalidação de `agyhub_summaries_proto.pb`.

### 2026-10-04 [Task 90.1] Registro único das rotas HTTP

- **Decisão:** `registerPair` grava o handler em `/api/v1{suffix}` e `/api{suffix}`. `HandleFunc` direto ficou só dentro de `register`.
- **Sem gêmeo `/api`:** `GET /api/v1/events` (canônico `GET /events`) e o gateway (`/v1/chat/completions`, `/v1/messages`, `/v1/models`, `/v1/router/*` mais o espelho `/api/v1`). Não foram criados `/api/events` nem `/api/chat/completions`.
- **Coberto agora:** `TestAPIRoutesHaveAliasTwin`.

### 2026-10-04 [Task 00.2] Lacunas de teste do gateway

- **Descoberto:** perfil sem credencial, com failover ligado, respondia 429 `rate_limit_exceeded` e não 401. O ramo que já distinguia falha de autenticação só rodava no fim do loop; `SelectProfile` esgotava o pool antes.
- **Contrato:** se toda tentativa falhou por credencial ausente, OpenAI devolve 401 `profile_unauthenticated` e Anthropic devolve 401 `authentication_error`. O upstream não é chamado. 429 permanece quando houve limite de fato.
- **Coberto agora:** `TestGatewayDoesNotForwardInboundAuthorization`, `TestGatewayUnauthenticatedProfile`, `TestGatewayMessagesUnauthenticatedProfile`, `TestGatewayEstimatesTokensWithoutUsageMetadata`, `TestGatewayUpstreamErrorDoesNotFailover`, `TestClientEndpointFailover`.

### 2026-10-04 [Task 02.1] Árvore de decisão dos runners

- **Contexto:** `prime`, `exec`, `headless run`, `dispatch` e `agent run` compartilham a ideia de "rodar um agente". A CLI de cada um permanece. A escolha fica na skill.
- **Decisão:** a primeira linha que casa vence. Diff para merge é `dispatch`. PTY ou `agent attach` é `agent run`. O mesmo prompt em dois ou mais perfis, com relatório, é `exec`. Só avançar janela de cota, com fração medida acima de 0,05 e reset elegível, é `prime`. O resto, um prompt e um perfil, é `headless run`. `exec` com um perfil continua válido na CLI; o agente escolhe `headless run` nesse caso.
- **Onde:** `skills/multigravity/SKILL.md`, ponteiro em `skills/multigravity-orchestrator/SKILL.md`, espelho curto em `README.md` e `README.pt-br.md`.

---

## Histórico Consolidado Anterior

Decisões técnicas de tarefas concluídas em releases anteriores (v1.4.0 a v2.2.0) foram consolidadas em [.agent/INVARIANTS.md](file:///home/yegear/github/ye-dev/multigravity-cli/.agent/INVARIANTS.md) e arquivadas em [.agent/ARCHIVE.md](file:///home/yegear/github/ye-dev/multigravity-cli/.agent/ARCHIVE.md) sob a seção `## Arquivo de Decisões Técnicas Consolidadas (v1.4.0 a v2.2.0)`.
