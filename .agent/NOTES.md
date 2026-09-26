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

### 2026-09-26 [Task 09.3] Alertas de cota crítica, queda e processo órfão

- **Contexto:** O dashboard e o `serve` precisam avisar cota no limiar, queda recente e headless que morreu deixando estado. O limiar de 5% já existe no warm-up, a série está em `[09.1]` e o reap de PID morto já vive no manager headless.
- **Decisões Técnicas:**
  - `quota.MinRemainingFraction` (0.05) é a mesma guarda do prime de 5h e dos alertas. Cota crítica: fração restante do último snapshot `source=quota` `<=` esse valor. Queda: diferença absoluta entre os dois últimos snapshots do mesmo bucket `>=` esse valor.
  - O alerta não consulta o language server. Sem dois pontos de histórico não há queda.
  - Órfão chama `headless.Manager.ReapStale`, o mesmo `reapDeadState` de `GetStatus`. Processo vivo não é morto. O alerta sai uma vez, porque o arquivo de estado sai junto.
  - `multigravity alerts [perfil] [--json]` sai com código zero quando a avaliação funciona, com ou sem alertas. `GET /api/v1/alerts` devolve o mesmo relatório. O broker SSE emite `alerts` só quando o conjunto muda e há clientes conectados.

### 2026-09-26 [Task 09.1] Histórico temporal de cota e tokens por perfil

- **Contexto:** `multigravity quota` e `GET /api/v1/quota` devolvem só a leitura instantânea do language server. O gateway e o dashboard precisam da série de fração restante e de tokens.
- **Decisões Técnicas:**
  - A série fica em `<perfil>/.multigravity/quota-history.jsonl` (modo `0600`). Não grava prompt, resposta nem token de autenticação.
  - Fontes: `quota` (snapshot ao vivo), `gateway` (completions OpenAI/Anthropic) e `headless` (`RunAgentPrompt`, inclusive `exec`).
  - Snapshot de cota idêntico dentro de 1 minuto não é regravado. Amostras com mais de 30 dias, ou além das 2000 mais recentes, saem na compactação.
  - Contagem do gateway usa `usageMetadata` do CloudCode quando o stream traz o campo. Sem isso, a amostra é uma estimativa de 4 runes por token (`estimated: true`).
  - `GET /api/v1/quota` permanece o ponto instantâneo. A série é `GET /api/v1/quota/history` e `GET /api/v1/quota/{profile}/history` (`since`, `until`, `source`, `limit`).

### 2026-09-26 [Task 13.2] Despacho concorrente via fan-out do headless run

- **Contexto:** `multigravity exec [perfil|--all] "<prompt>"` precisa queimar cotas isoladas em paralelo. O isolamento de `HOME` e do cofre por perfil já permite concorrência real, ao contrário de ferramentas que alternam o keyring global.
- **Decisões Técnicas:**
  - O fan-out vive em `headless.Manager.Exec` e só chama `RunAgentPrompt`. Não há pacote novo, worktree, PTY nem roteamento de gateway.
  - Pool de workers (default: um em voo por perfil selecionado; `--workers` limita). O relatório JSON (`results`, `succeeded`, `failed`, `total_tokens`) preserva a ordem dos perfis.
  - Falha de um perfil entra no agregado; o CLI imprime o relatório e sai com código diferente de zero. `POST /api/v1/exec` devolve HTTP 200 com o mesmo relatório.
  - Duplicatas na lista de perfis são removidas. Prompt vazio, perfil inexistente ou `--all` sem perfis abortam antes de disparar o runner.

### 2026-09-26 [Task 13.1] Autenticação Direta Headless via CLI com OAuth2 PKCE

- **Contexto:** Perfis headless precisavam de login Google sem abrir a IDE e sem gravar o refresh token no chaveiro global do sistema (invariante de isolamento: o keyring do SO é compartilhado e não pode ser alternado entre contas).
- **Decisões Técnicas:**
  - Pacote `internal/auth`: PKCE S256, listener efêmero só em `127.0.0.1:0` no path `/callback`, troca do authorization code e consulta de userinfo. O `state` inválido não encerra o fluxo; `error` do provedor encerra.
  - Cofre do perfil, modo `0600`: o mesmo JSON de credencial (`token` + `auth_method: consumer`) vai para `.gemini/antigravity-cli/antigravity-oauth-token` e `.gemini/jetski-standalone-oauth-token`. E-mail e nome ficam em `account.json`, sem tokens.
  - CLI `multigravity login` (`status`, `logout`), `--json`, `--no-browser`, `--timeout`, `--force`. A saída JSON não inclui access token nem refresh token. Login e logout abortam se o perfil estiver em execução, salvo `--force`.
  - `GEMINI_FORCE_FILE_STORAGE=true` só é exportado no ambiente isolado quando o cofre já existe, para o `agy` ler o arquivo do perfil e não o chaveiro do host. Perfis autenticados pela IDE via keyring continuam sem essa variável.
  - O comando não escreve no Credential Manager, Keychain nem Secret Service do host.

### 2026-09-26 [Task 11.6] Alinhamento Estratégico de Posicionamento, Documentação e Catálogo da Plataforma

- **Contexto:** Com a evolução rápida da arquitetura Go v2.0 nos Épicos 07, 08, 14 e 15, o `multigravity-cli` deixou de ser um simples launcher/gerenciador de perfis da IDE e tornou-se uma Plataforma de Desenvolvimento Agêntico (ADE) e Gateway de IA Multi-Contas. A documentação voltada a humanos (`README.md`, `README.pt-br.md`, `CHANGELOG.md`) estava defasada, omitindo recursos críticos de gateway, orquestração de tarefas, worktrees efêmeros e visualizador de diffs web.
- **Decisões Técnicas e de Posicionamento:**
  - **Identidade e Preservação de Marca:** A marca canônica e o nome do binário `multigravity` foram preservados integralmente para honrar os invariantes de compatibilidade (atalhos do SO, links simbólicos, scripts de instalação).
  - **Tagline e Posicionamento:** Atualizados para *"The Agentic Development Platform & Multi-Account AI Gateway for Google Antigravity (and `agy`)"*, destacando o tripé: Isolamento de Perfis, Gateway de IA Multi-Contas com Failover e Orquestração de Agentes Autônomos com Worktrees.
  - **Documentação de Recursos Críticos:**
    - Catálogo completo de comandos adicionado nas tabelas do README: `dispatch`, `agent`, `worktree`, `workspace`.
    - Seções práticas detalhadas com exemplos de consumo via `curl`, Cursor, Aider e Claude Code para os gateways `/v1/chat/completions` e `/v1/messages`.
    - Documentação dos Git Worktrees efêmeros (`.multigravity/worktrees/`) e do visualizador gráfico de diffs embutido (`/ui/tasks`).
    - Paridade 100% de conteúdo e exemplos entre `README.md` (EN) e `README.pt-br.md` (PT-BR).
  - **Atualização do Changelog:** Seção `[Unreleased]` do `CHANGELOG.md` preenchida com todas as entregas concluídas pós-v2.0.0.

### 2026-09-26 [Task 08.2] Invocação e Gestão de Agentes Headless em Background com Isolamento de Identidade

- **Contexto:** Agentes headless, pipelines automatizados, orquestradores externos (Cursor, Claude Code, Aider, agregadores REST) e rotinas de cota exigem acesso persistente ou sob demanda ao `language_server` do Antigravity e à CLI `agy` sem abrir a pesada interface gráfica Electron da IDE. Anteriormente, invocações efêmeras recriavam o processo a cada chamada sofrendo cold-start de 1.5–3.0s. Além disso, a execução de prompts de agentes headless necessitava de isolamento estrito de identidade e contratos estruturados em JSON.
- **Decisões Técnicas:**
  - **Módulo Desacoplado `internal/headless`:**
    - `types.go`: contratos estruturados `InstanceInfo` (`profile`, `pid`, `port`, `csrf_token`, `status`, `started_at`, `log_file`, `health_error`, `executable_path`), `StartOptions`, `AgentRunOptions`, `AgentRunResult` (`profile`, `prompt`, `response`, `total_tokens`, `duration_seconds`, `exit_code`, `error`).
    - `process_unix.go` e `process_windows.go`: abstração multiplataforma para execução desanexada (`Setpgid: true` no POSIX, `CREATE_NEW_PROCESS_GROUP` no Windows), verificação não-invasiva de liveness de PID e encerramento gracioso (SIGTERM com fallback para kill).
    - `manager.go`:
      - Persistência e auto-reaping: grava `<profileDir>/.multigravity/headless.json` e detecta PIDs mortos de forma resiliente, limpando estado órfão automaticamente.
      - Invariante #8: exporta `$HOME` / `%USERPROFILE%` direcionado para a raiz do perfil e preserva o `PATH` do host com caminhos binários do usuário.
      - Captura de porta HTTPS dinâmica e token CSRF a partir do stderr com verificação imediata de health check TLS.
      - Métodos: `Start`, `Stop`, `Restart`, `GetStatus`, `List`, `GetLogs`.
    - `runner.go`:
      - Motor duplo para execução de prompts: executa via `agy` (`-p <prompt> --print-timeout <dur> --output-format json --dangerously-skip-permissions`) ou fallback via Language Server Cascade RPC (`StartCascade` / `SendUserCascadeMessage`).
      - Parse estruturado de saída JSON e contagem de tokens consumidos.
    - Test hooks herméticos (`SetTestHooks`, `SetRunnerTestHooks`) garantindo 100% de cobertura sem processos reais em runtime de teste.
  - **Fast-Path em `internal/quota/headless.go`:**
    - `StartHeadlessServer` verifica se um servidor headless gerenciado já está ativo e saudável para o perfil antes de disparar um processo novo, eliminando o cold-start de ~2s em `multigravity quota` e `multigravity prime` (resposta em ~10ms) com `IsManaged: true` evitando que `Close()` finalize o processo em background.
  - **CLI `multigravity headless` (aliases `hl`) (`internal/cmd/headless.go`):**
    - Subcomandos: `list` (alias `ls`), `status <profile>`, `start <profile>`, `stop <profile>`, `restart <profile>`, `logs <profile>` (`--tail`, `--follow`), `run <profile> <prompt>` (`--timeout`, `--dangerously-skip-permissions`, `--json`).
    - Contrato `--json` em todos os subcomandos e autocompletion de perfis.
  - **API REST & SSE (`internal/server`):**
    - Rotas duplas `/api/v1/headless/...` e `/api/headless/...`:
      - `GET /headless`: listagem de instâncias ativas.
      - `GET /headless/{profile}`: status e health check.
      - `POST /headless/{profile}/start`: inicialização em background.
      - `POST /headless/{profile}/stop`: encerramento gracioso.
      - `POST /headless/{profile}/restart`: reinicialização.
      - `POST /headless/{profile}/run`: execução de prompt headless.
      - `GET /headless/{profile}/logs`: logs com suporte a `?tail=N`.
    - Eventos SSE emitidos em tempo real (`action: "headless_start"`, `"headless_stop"`, `"headless_restart"`).

### 2026-09-26 [Task 08.1] Detecção e Mapeamento de Workspaces e Repositórios Ativos por Perfil

- **Contexto:** Perfis do Antigravity possuem workspaces e projetos associados em `.gemini/config/projects/*.json` com metadados de branches, políticas de execução de agentes (sandboxMode, autoExecutionPolicy) e links de sistema de arquivos (`file://...`). Para orquestradores de agentes, GUIs desktop (Tauri/Wails) e desenvolvedores em terminal, era fundamental mapear quais repositórios pertencem a cada perfil, correlacionar o diretório atual (`multigravity ws current`) e rastrear qual workspace está ativamente aberto em instâncias em execução.
- **Decisões Técnicas:**
  - **Módulo Desacoplado `internal/workspace`:**
    - `types.go`: contratos estruturados `Workspace`, `GitRepoInfo` (`branch`, `remote_url`, `commit_hash`, `commit_message`, `is_clean`, `modified_files`, `untracked_files`), `WorkspaceSettings`, `ProfileWorkspacesSummary`, e schemas de parsing de projetos Antigravity.
    - `git.go`: inspeção de telemetria Git sem mutações (`DetectGitRepoInfo`), com hook testável `SetGitRunnerFn` permitindo testes unitários 100% herméticos.
    - `detector.go`:
      - `FileURIToPath`: decodificação de `file://` URIs com paridade estrita Linux/macOS/Windows (tratando caminhos absolutos e letras de drive).
      - `GetLastSelectedProject`: extração determinística do projeto selecionado a partir de `<userDataDir>/app_storage.json` (`new-convo-last-selected-project`, `lastCreatedProjectId`).
      - `GetProfileWorkspaces`: mapeia projetos do perfil e marca `IsActive = true` quando a instância está em execução (`profile.IsProfileRunning`) e o projeto corresponde ao `app_storage.json` ou argumentos de processo.
      - `GetAllWorkspaces`: agrega e ordena (ativos primeiro, depois por nome).
      - `GetActiveWorkspaces`: filtra exclusivamente workspaces ativos.
      - `GetWorkspaceByPath`: mapeia qual perfil e workspace contêm um determinado caminho de diretório local.
      - `GetProfileSummary`: visão consolidada do perfil e workspace ativo.
  - **CLI `multigravity workspace` (aliases `ws`, `workspaces`) (`internal/cmd/workspace.go`):**
    - Subcomandos: `list` (alias `ls`, flags `--active`, `--json`), `active` (`--json`), `current` (aliases `here`, `pwd`, `--json`), `show <profile> <workspace>` (alias `get`, `info`, `--json`).
    - Autocompletion dinâmico via shell completion para nomes de perfis e workspaces.
  - **Endpoints REST HTTP (`internal/server`):**
    - `GET /api/v1/workspaces` (e `/api/workspaces`): listagem com filtros `?profile=`, `?active=true`, `?path=`.
    - `GET /api/v1/workspaces/active`: listagem de workspaces ativos.
    - `GET /api/v1/profiles/{name}/workspaces`: resumo e lista de workspaces do perfil.
    - `GET /api/v1/profiles/{name}/workspaces/active`: workspace ativo do perfil.
  - **Testes e Validação:**
    - 100% dos testes unitários e de integração passando em `internal/workspace`, `internal/cmd` e `internal/server`.


### 2026-09-25 [Task 15.4] Visualizador e API de Diffs / Status de Execução de Tarefas no multigravity serve para futura GUI Desktop (Tauri/Wails)

- **Contexto:** Com o motor de despacho (`internal/dispatch`), terminais virtuais PTY (`internal/agent`) e Git Worktrees (`internal/worktree`) ativos, interfaces gráficas desktop (Tauri v2 / Wails v2 / web companions) e desenvolvedores em terminal necessitam de contratos de dados estruturados de diff (arquivos modificados, hunks, contadores `+`/`-`, detecção binária), endpoint consolidado de telemetria/dashboard de tarefas e visualizador web embutido diretamente no binário (`//go:embed`).
- **Decisões Técnicas:**
  - **Parser de Unified Git Diff (`internal/dispatch/diff_parser.go`):**
    - `ParseUnifiedDiff(raw string) *StructuredDiff`: parser puro em Go capaz de processar diffs unificados gerados pelo Git, extraindo cabeçalhos `diff --git`, caminhos de arquivos antigos/novos, status (`modified`, `added`, `deleted`, `renamed`), detecção de binários (`Binary files ... differ`), e hunks completos (`@@ -old,lines +new,lines @@`) com linhas tipadas (`context`, `addition`, `deletion`, `header`) e contadores numéricos de linha preservados.
    - Contratos estruturados em `types.go`: `StructuredDiff`, `DiffSummary` (`files_changed`, `additions`, `deletions`), `DiffFile`, `DiffHunk`, `DiffLine`, e `TaskDashboardSummary`.
  - **Métodos Desacoplados no `TaskManager` (`internal/dispatch/manager.go`):**
    - `GetTaskStructuredDiff(repoPath, id string)`: converte diff do worktree da tarefa em `StructuredDiff` (retornando estrutura vazia em tarefas sem worktree em vez de erro abrupto).
    - `GetTaskFiles(repoPath, id string)`: retorna a lista resumida de `DiffFile` para árvores laterais rápidas na UI.
    - `GetDashboardSummary(repoPath string)`: consolida contadores de tarefas (`total`, `running`, `completed`, `failed`, `cancelled`), tarefas ativas e até 15 tarefas mais recentes.
  - **Endpoints REST & UI no Servidor HTTP (`internal/server`):**
    - `GET /api/v1/dispatch/tasks/{id}/diff`: suporta `?format=structured` ou `?structured=true`, retornando o objeto aditivo `structured` no payload JSON mantendo 100% de retrocompatibilidade com clientes existentes.
    - `GET /api/v1/dispatch/tasks/{id}/files`: listagem simplificada de arquivos e contadores.
    - `GET /api/v1/dispatch/dashboard`: dados consolidados do dashboard.
    - `GET /ui/tasks`: painel web moderno com estatísticas executivas, tabela de tarefas, badges em tempo real e atualização automática via eventos SSE (`/events`).
    - `GET /ui/tasks/{id}/diff`: visualizador interativo embutido de diff com barra de estatísticas (`files changed`, `+N additions`, `-N deletions`), árvore de navegação de arquivos lateral e realce de código/linhas unificado.
    - Empacotamento hermético via `//go:embed` em `internal/server/ui/assets/` (`diff.html` e `tasks.html`), garantindo zero dependências de Node.js ou runtime externo.
  - **CLI `multigravity dispatch` (`internal/cmd/dispatch.go`):**
    - `multigravity dispatch diff <task-id>`:
      - `--structured`: exibe resumo tabular de arquivos alterados e contadores de linha no terminal.
      - `--json`: inclui o schema completo `structured` no payload de saída.
      - `-w, --web`: imprime a URL local pronta para visualização no navegador ou webview.
    - `multigravity dispatch dashboard` (alias `dash`): visualizador de terminal e contrato `--json` com métricas consolidadas de tarefas.
  - **Correção de Concorrência em PTY (`internal/agent/session.go`):**
    - Identificada condição de corrida onde `cmd.Wait()` retornava no encerramento do processo e `s.ptyDev.Close()` era executado imediatamente pelo `waitLoop`, antes de o `readLoop` drenar os últimos bytes presentes no buffer de kernel do PTY. Adicionado canal de sincronização `readDone` permitindo esvaziamento completo da saída antes do teardown do descritor de terminal.

### 2026-09-25 [Task 15.3] Motor de Despacho de Tarefas (multigravity dispatch) com Associação de Perfil, Worktree e Captura de Logs

- **Contexto:** Orquestração de agentes autônomos de desenvolvimento e tarefas isoladas requer um motor central (`internal/dispatch`) capaz de coordenar o provisionamento de Git Worktrees efêmeros (`internal/worktree`), isolamento de perfil e identidade com variáveis de ambiente dedicadas (`internal/profile`), e execução em terminais interativos virtuais PTY (`internal/agent`), persistindo logs de execução (`run.log`) e metadados (`task.json`).
- **Decisões Técnicas:**
  - **Módulo `internal/dispatch`:**
    - `types.go`: contratos estruturados `Task`, `TaskStatus` (`pending`, `running`, `completed`, `failed`, `cancelled`), `DispatchOptions`, `TaskFilter`, `TaskManifest` e `TaskDiff`.
    - `manager.go`: ciclo de vida completo (`Dispatch`, `GetTask`, `ListTasks`, `CancelTask`, `DeleteTask`, `PruneTasks`, `GetTaskLogs`, `GetTaskDiff`).
    - **Gravação Síncrona e Stream de Logs (`streamLogsToFile`):** O arquivo de log `run.log` e o manifesto `task.json` são criados de forma síncrona na chamada `Dispatch()`. Na inicialização do streaming, o chunk inicial já emitido pela sessão PTY (`inst.GetOutput(0)`) é gravado imediatamente no disco antes de aguardar o canal pub/sub, prevenindo perda de logs para comandos rápidos (< 10ms).
    - **Fallback de Logs em Memória:** `GetTaskLogs()` lê o arquivo `run.log` do disco; caso o arquivo esteja vazio (ex: comando em buffer antes do flush de saída), consulta dinamicamente o buffer circular da sessão PTY (`inst.GetSessionOutput()`), garantindo que consultas à API ou CLI nunca retornem saída vazia indevidamente.
    - **Gerenciamento Seguro de Concorrência e Cleanup:** Cancelamento de tarefas invoca `inst.StopSession(sessionID, 3*time.Second)` e atualiza o estado para `cancelled`. Exclusão (`DeleteTask`) e poda (`PruneTasks`) verificam ativamente se a sessão está em execução (abortando sem `--force` ou matando o processo com `force: true`) e limpam opcionalmente a worktree correspondente via `wtMgr.RemoveWorktree(..., force)`.
  - **CLI `multigravity dispatch` (aliases `dp`, `task`):**
    - Subcomandos: `run <cmd>`, `list` (alias `ls`), `status <task-id>`, `logs <task-id>`, `diff <task-id>`, `cancel <task-id>`, `delete <task-id>` (alias `rm`), `prune`.
    - Suporte a `--profile`, `--repo`, `--branch`, `--worktree` (criação automática ou reuso), `--env`, `--background` (modo desacoplado vs streaming em tempo real), `--tail` e `--follow` (`-f`) para logs.
    - Contratos estruturados em JSON via `--json` em todos os subcomandos de consulta e ciclo de vida.
    - Autocompletion dinâmico para perfis e task IDs.
  - **API REST & SSE (`internal/server`):**
    - Endpoints registrados sob prefixos duplos `/api/v1/dispatch/...` e `/api/dispatch/...`:
      - `POST /tasks`: cria e despacha nova tarefa.
      - `GET /tasks`: lista tarefas com suporte a filtros de query (`status`, `profile`, `repo`).
      - `GET /tasks/{id}`: consulta detalhes da tarefa.
      - `GET /tasks/{id}/logs`: obtém logs com suporte a `?tail=N` e `?follow=true` via SSE chunk streaming.
      - `GET /tasks/{id}/diff`: obtém diff de Git e estatísticas de arquivos alterados no worktree.
      - `POST /tasks/{id}/cancel`: cancela execução.
      - `DELETE /tasks/{id}`: remove tarefa e metadados.
      - `POST /tasks/prune`: limpa tarefas concluídas/falhas com `retention_hours`.
    - Eventos SSE emitidos em tempo real no feed global (`event: "action"`, `action: "task_dispatch"`).
  - **Fix de Concorrência em `internal/agent/session.go`:**
    - Corrigido race condition onde `waitLoop` fechava os canais de assinantes fora do mutex `s.mu`, enquanto `readLoop` chamava `broadcast()` concorrentemente, causando pânico de `send on closed channel`. A finalização de canais foi encapsulada com guarda atômica `s.closed` sob o mesmo mutex de escrita.

### 2026-09-25 [Task 15.2] Multiplexador de Terminais PTY e Execução Headless de Agentes CLI (Claude Code, Aider, OpenCode) com Isolamento de Identidade

- **Contexto:** Ferramentas modernas de agentes autônomos de desenvolvimento (Claude Code CLI, Aider, OpenCode, Agy) requerem um terminal interativo real (PTY / TTY) para exibir interfaces ricas em ANSI, processar prompts interativos de confirmação (`[y/n]`, concessão de ferramentas) e capturar fielmente logs de execução. O sistema necessitava de um multiplexador concorrente com isolamento estrito de perfil (`$HOME` / `%USERPROFILE%`) e roteamento transparente para os gateways OpenAI e Anthropic locais.
- **Decisões Técnicas:**
  - **Módulo `internal/agent`:**
    - `types.go`: contratos estruturados `Session`, `SessionStatus`, `CreateSessionOptions`, `SessionFilter`, `OutputChunk`, `ResizeOptions` e `SendInputRequest`.
    - `env.go`: construção de ambiente isolado (`BuildAgentEnv`), redirecionando `HOME` / `USERPROFILE` para `$MULTIGRAVITY_HOME/<profile>`, preservando caminhos binários de host do usuário e injetando variáveis de Gateway (`ANTHROPIC_BASE_URL` para Claude Code; `OPENAI_BASE_URL` e `OPENAI_API_BASE` para Aider e OpenCode).
    - `pty_unix.go`: abstração PTY para POSIX (Linux/macOS) via `github.com/creack/pty` com suporte a redimensionamento dinâmico (`pty.Setsize`).
    - `pty_windows.go`: fallback gracioso para Windows com pipes assíncronos (`io.Pipe`) assegurando paridade multiplataforma (Regra de Ouro #3 do `AGENTS.md`).
    - `session.go`: controlador de instância com buffer circular/deslizante (`maxBufferSize = 512 KB`) e pub/sub não-bloqueante para múltiplos assinantes simultâneos (CLI e SSE).
    - `manager.go`: gerenciador central de ciclo de vida (`StartSession`, `GetSession`, `ListSessions`, `StopSession`, `KillSession`, `WriteSessionInput`, `ResizeSession`, `GetSessionOutput`, `SubscribeSession`, `PruneSessions`).
  - **CLI `multigravity agent` (alias `ag`):**
    - Subcomandos: `run <profile> [--] <cmd>`, `list` (alias `ls`), `status <id>`, `logs <id>`, `stop <id>`, `kill <id>`, `attach <id>`.
    - Suporte a modo interativo em raw mode (`golang.org/x/term`) com captura de `SIGWINCH` e modo desacoplado (`--detach` / `-d`).
    - Contratos estruturados em JSON via `--json` em todos os comandos de consulta e ciclo de vida.
  - **API REST & SSE (`internal/server`):**
    - Endpoints registrados: `GET|POST /api/v1/agent/sessions`, `GET|DELETE /api/v1/agent/sessions/{id}`, `GET /api/v1/agent/sessions/{id}/output`, `GET /api/v1/agent/sessions/{id}/stream` (SSE chunk feed), `POST /api/v1/agent/sessions/{id}/input`, `POST /api/v1/agent/sessions/{id}/resize`, `POST /api/v1/agent/sessions/prune`.
    - Eventos SSE emitidos em tempo real (`action: "agent_session"`) no barramento global de telemetria.

### 2026-09-25 [Task 15.1] Gerenciador de Git Worktrees Efêmeros por Agente/Tarefa (`internal/worktree`)

- **Contexto:** Para suportar orquestração de múltiplos agentes de IA (Claude Code, Aider, OpenCode) e execuções paralelas sobre o mesmo repositório, o sistema necessita de isolamento em nível de sistema de arquivos através de Git Worktrees efêmeros, sem poluir o `git status` do repositório hospedeiro nem sobrescrever branches principais.
- **Decisões Técnicas:**
  - **Módulo `internal/worktree`:**
    - `types.go`: contratos estruturados `Worktree`, `WorktreeStatus`, `CreateOptions`, `RemoveOptions`, `DiffOptions` e `WorktreeManifest`.
    - `git.go`: invocações isoladas via `exec.Command` para comandos Git com resolução de `--git-common-dir`.
    - `manager.go`: ciclo de vida completo (`CreateWorktree`, `ListWorktrees`, `GetWorktree`, `RemoveWorktree`, `PruneWorktrees`, `GetWorktreeStatus`, `GetWorktreeDiff`).
  - **Convenção de Localização e Isolamento de Git (`EnsureGitExclude`):**
    - Worktrees por padrão residem em `<repo>/.multigravity/worktrees/<task-id>`.
    - Adiciona automaticamente `.multigravity/` em `<gitCommonDir>/info/exclude` de forma idempotente, mantendo o diretório invisível para `git status` e `git diff` sem alterar o `.gitignore` versionado do projeto.
  - **Armadilha Evitada no Parser de `git status --porcelain`:**
    - A função `runGit` deve remover apenas `\r\n` trailing (`strings.TrimRight(out, "\r\n")`) e NUNCA `strings.TrimSpace` na saída completa, pois modificações unstaged possuem formato ` M <file>` com espaço leading no índice 0. O parser extrai o caminho com `strings.TrimSpace(l[2:])`.
  - **Persistência de Metadados e Reconciliação:**
    - Metadados gravados em `<gitCommonDir>/multigravity-worktrees.json` sob mutex de sincronização, reconciliados dinamicamente com a saída de `git worktree list --porcelain`.
  - **CLI `multigravity worktree` (alias `wt`):**
    - Subcomandos: `list`, `create`, `status`, `diff`, `remove`, `prune` com contratos JSON estruturados via `--json` e autocompletion dinâmico.
  - **API REST & SSE (`internal/server`):**
    - Rotas `/api/v1/worktrees` e `/api/worktrees` registradas para todas as operações, transmitindo eventos de ação em tempo real no feed SSE (`action: "worktree"`).

### 2026-09-25 [Task 14.4] Gateway Anthropic-Compatible (/v1/messages) e Mapeamento de Modelos (Claude Sonnet/Opus ↔ Gemini 3.5/3.6)

- **Contexto:** Ferramentas, bibliotecas e agentes projetados para o ecossistema Anthropic (como Claude Code CLI, Cursor, Aider, Cline, Roo Code, e SDKs `@anthropic-ai/sdk` / Python `anthropic`) demandam conformidade estrita com o protocolo da Anthropic Messages API (`POST /v1/messages`), incluindo sua sequência específica de eventos SSE (`message_start`, `content_block_start`, `content_block_delta`, `content_block_stop`, `message_delta`, `message_stop`), blocos de `system` (string ou array de blocos) e content blocks multimodais (imagens base64).
- **Decisões Técnicas:**
  - **Módulo de Tipos e Contratos (`internal/gateway/anthropic_types.go`):**
    - `AnthropicMessageRequest`, `AnthropicMessage`, `AnthropicContentBlock` (com `AnthropicImageSource`), `AnthropicMessageResponse`, `AnthropicUsage`.
    - Eventos SSE estruturados: `AnthropicMessageStartEvent`, `AnthropicContentBlockStartEvent`, `AnthropicContentBlockDeltaEvent` (`AnthropicTextDelta`), `AnthropicContentBlockStopEvent`, `AnthropicMessageDeltaEvent`, `AnthropicMessageStopEvent`.
    - `AnthropicErrorResponse` no formato nativo `{"type": "error", "error": {"type": "...", "message": "..."}}`.
  - **Conversão e Manipulação (`internal/gateway/anthropic.go`):**
    - `ExtractAnthropicSystem`: extrai instrução do sistema tanto de string quanto de arrays de blocos ou JSON cru.
    - `CollapseAnthropicMessages`: traduz mensagens do formato Anthropic para `CloudCodeRequest` (mapeando `assistant` para o papel `"model"` do CloudCode, empacotando imagens base64 em `inlineData` e texto em `parts`).
    - `HandleMessages`: processa `POST /v1/messages` e `/api/v1/messages`, suportando credenciais via `x-api-key` ou `Authorization: Bearer <key>`.
    - Integração transparente com `Router`: seleção de nós via `SelectProfile`, auto-failover em HTTP 429/403 com `StreamGenerateContentWithConnect`, e emissão dos headers de telemetria `X-Profile-Used`, `X-Failover-Count`, `X-Remaining-Profiles`, `X-Routing-Strategy`.
  - **Mapeamento e Normalização de Modelos (`internal/gateway/models.go`):**
    - Suporte a aliases com hífen e notação de ponto (`claude-3-7-sonnet`, `claude-3.7-sonnet`, `claude-3-5-sonnet`, `claude-3.5-sonnet`, `claude-3.5-haiku`, `claude-sonnet`, `claude-opus`, `claude-haiku`).
    - Heurística dinâmica para famílias Claude direcionando versões 3.7 para `gemini-3.6-flash-high`, Sonnet para `claude-sonnet-4-6`, Opus para `claude-opus-4-6` e Haiku para `gemini-3.5-flash-low`/`medium`.
  - **Rotas e CORS no Servidor (`internal/server`):**
    - Endpoints registrados: `POST /v1/messages` e `POST /api/v1/messages`.
    - `corsMiddleware` expandido para aceitar headers `x-api-key`, `anthropic-version`, `anthropic-beta` em `Access-Control-Allow-Headers`.

- **Contexto:** Em ambientes com múltiplos agentes ou automações concorrentes, contas isoladas esgotam cotas de 5h ou semanais em momentos distintos. Usuários necessitam de um balanceador inteligente com auto-failover transparente para que chamadas a `/v1/chat/completions` nunca sejam interrompidas enquanto houver ao menos um perfil com cota saudável no pool.
- **Decisões Técnicas:**
  - **Módulo `Router` em `internal/gateway/router.go`:**
    - Algoritmos implementados: `smart` (Smart Priority ponderado por cota restante, penalidade de erros e bônus de ociosidade), `round-robin` (rotação sequencial ignorando perfis em cooldown), `priority` (ordem declarada/alfabética de lista) e `sticky` (afinidade com o último perfil saudável até falhar).
    - Estado de nó `ProfileNode` com rastreamento thread-safe de `CooldownUntil`, `RemainingFraction`, `TotalRequests`, `TotalSuccess`, `RateLimitHits`, `TotalFailovers`, `ConsecutiveErrors`.
    - Sincronização dinâmica com `profile.ListProfiles()`.
  - **Mecanismo de Auto-Failover Transparente (`internal/gateway/client.go` e `gateway.go`):**
    - Definição do erro estruturado `UpstreamHTTPError` e detector `IsRateLimitOrQuotaExhausted`.
    - Método `StreamGenerateContentWithConnect`: em modo streaming, a conexão HTTP 200 é confirmada com o upstream antes de gravar os headers SSE (`text/event-stream`) no cliente. Se o upstream retornar HTTP 429 ou 403, o router coloca o perfil em cooldown, incrementa contadores de failover e tenta o próximo perfil sem que o cliente perceba qualquer anomalia.
    - Emissão de headers de transparência: `X-Profile-Used`, `X-Failover-Count`, `X-Remaining-Profiles`, `X-Routing-Strategy`.
  - **Endpoints de Gestão e CORS (`internal/server`):**
    - `GET /v1/router/status` (e alias `/api/v1/router/status`): telemetria completa do pool em JSON.
    - `POST /v1/router/reset`: limpa cooldowns de todos os perfis.
    - `POST /v1/router/strategy`: altera dinamicamente o algoritmo ativo (`{"strategy": "round-robin"}`).
    - `corsMiddleware` atualizado para aceitar e expor os novos headers.

### 2026-09-25 [Task 14.2] Gateway de Completions OpenAI-Compatible (`/v1/chat/completions`) no `multigravity serve` com SSE

- **Contexto:** Ferramentas externas de IA (Cursor, Aider, OpenCode, Continue, scripts Python/TypeScript com SDK OpenAI) requerem um endpoint HTTP local compatível com a especificação OpenAI (`POST /v1/chat/completions` e `GET /v1/models`) para consumir modelos do Antigravity/CloudCode via streaming SSE de baixa latência e respostas atômicas em JSON.
- **Decisões Técnicas:**
  - **Módulo Desacoplado `internal/gateway`:**
    - `types.go`: contratos estruturados para requests, responses, chunks de delta SSE, catálogo de modelos e erros padronizados OpenAI (`invalid_request_error`, `api_error`).
    - `models.go`: catálogo de modelos expostos e normalização inteligente (`NormalizeModel`), mapeando modelos Gemini (`gemini-2.5-pro`, `gemini-2.5-flash`, `gemini-3.5-flash`, etc.) e aliases 3P (`gpt-4o`, `gpt-4o-mini`, `claude-3-5-sonnet`, `claude-3-7-sonnet`, `claude-sonnet-4-6`, `gpt-oss-120b-medium`).
    - `collapse.go`: conversor e colapsador `CollapseOpenAIMessages()`, extraindo instruções `system` no bloco nativo `systemInstruction`, mapeando `assistant` para o papel `"model"` do CloudCode, e tratando partes multimodais com URIs de imagem base64 (`data:image/...;base64,...`) em `inlineData`.
    - `client.go`: cliente upstream para `POST https://cloudcode-pa.googleapis.com/v1internal:streamGenerateContent?alt=sse` (e fallback `daily-cloudcode-pa.googleapis.com`). Cumpre rigorosamente a **invariante de Chesterton de nunca enviar o cabeçalho `x-goog-user-project`**.
    - `gateway.go`: controlador central com suporte a streaming contínuo via `http.Flusher` (emitindo chunks `chat.completion.chunk`, chunk final de parada `finish_reason: "stop"` e marcador `data: [DONE]`) e modo atômico (`stream: false`).
  - **Integração no Servidor HTTP (`internal/server`):**
    - Rotas canônicas registradas: `POST /v1/chat/completions` e `GET /v1/models`, com aliases espelhados sob `/api/v1/`.
    - Atualizado `corsMiddleware` para expor o cabeçalho `X-Profile` em `Access-Control-Allow-Headers`.
    - Exportado método `SetGateway()` para injeção hermética de instâncias em testes de integração sem dependências de rede.

### 2026-09-25 [Benchmark & Roadmap] Arquitetura de Gateway Multi-Contas (Elysium) e Orquestrador de Agentes ADE (Orca / Alethe)

- **Contexto:** Benchmark realizado comparando o `multigravity-cli` com `mbl9898/multigravity-elysium`, `Kc1t/alethe-agents` e `Orca (onorca.dev)` para estruturar os Épicos 14 (Fase 1: Gateway de IA & Cotas) e 15 (Fase 2: Orquestrador de Agentes ADE).
- **Descobertas Técnicas (multigravity-elysium):**
  - **Heurística de Classificação de Janelas de Cota:** Diferenciação matemática precisa entre limite rotativo de 5 horas e semanal: $\Delta t_{\text{5h}} \le 5\text{h} \approx 0.208\text{ dias}$. Ponto de corte ótimo é $12\text{ horas}$ (`MIN_WEEKLY_RESET_DAYS = 0.5`). Reset $> 12\text{h}$ é garantidamente Semanal.
  - **Ping de Janela de 5 Horas:** O timer de 5h do CloudCode só dispara após o 1º token; disparar um ping de 1 token proativamente pela manhã antecipa a primeira janela de renovação.
  - **Gateway CloudCode Streaming:** Endpoint upstream interno `POST https://cloudcode-pa.googleapis.com/v1internal:streamGenerateContent?alt=sse` com `Authorization: Bearer <token>` e payload `{model, request: {contents: [...]}}`. Invariante crítica: **NUNCA** enviar header `x-goog-user-project`. Auto-failover em HTTP 429/403 chaveia de conta sem quebrar o stream do cliente.
- **Padrões de Orquestração ADE (Orca / Alethe-Agents):**
  - **Worktree-First:** Execução paralela sem conflito através de `git worktree add` para branches efêmeras dedicadas a cada tarefa de agente.
  - **PTY Multiplexing:** Terminais virtuais reais (`creack/pty` em Go) para CLI interativas (Claude Code, Aider, OpenCode) permitindo stdin/stdout streaming, perguntas de confirmação e captura de histórico.
  - **Cota-Pooling:** O `multigravity` atuará como provedor unificado de inteligência e roteador de tokens para todos os agentes filhos despachados.

### 2026-09-25 [Task 14.1] Heurística de Janela de Cotas (5h vs Semanal) e Ping de Aquecimento Proativo

- **Contexto:** Os serviços da Google (CloudCode / Gemini Code Assist) e 3P (Claude/GPT) organizam os limites em janelas rotativas de 5 horas e ciclos semanais. Anteriormente, a identificação dependia apenas de correspondências literais fixas de IDs. Além disso, o timer de renovação da janela de 5 horas só inicia a contagem regressiva após o consumo do primeiro token do ciclo, fazendo com que janelas ociosas demorassem para recarregar se o usuário só trabalhasse no final do dia.
- **Decisões Técnicas:**
  - **Heurística Matemática de Classificação de Janelas (`internal/quota/heuristic.go`):**
    - `ClassifyWindow(b QuotaBucket, now time.Time)`: analisa identificadores canônicos (`5h`, `rolling`, `sliding`, `hourly` $\to$ `Window5h`; `weekly`, `week`, `7d` $\to$ `WindowWeekly`) e o tempo até o reset $\Delta t_{\text{reset}}$. Se $\Delta t_{\text{reset}} \le 12\text{h}$, classifica como janela de 5 horas; se $> 12\text{h}$, classifica como ciclo semanal.
    - `CanWarm5hWindow(b QuotaBucket, parentWeekly *QuotaBucket, now time.Time)`: avalia se uma janela de 5 horas está ociosa e recém-resetada ($RemainingFraction \ge 0.999$), sem contagem regressiva ativa em andamento e com a cota semanal pai preservada ($> 5\%$).
  - **Enriquecimento Não-Quebrante em `QuotaBucket` (`internal/quota/types.go`):**
    - Campo aditivo `WindowType string json:"windowType,omitempty"`, preenchido deterministicamente em `RetrieveUserQuotaSummary`.
    - `RenderQuotaStatus` exibe selos `[5-Hour Window]` / `[Weekly Limit]` e emite a dica proativa `💡 Proactive 5h warm-up available: multigravity prime <profile> --warm-5h` quando o bucket de 5h estiver 100% livre.
  - **Motor de Aquecimento Proativo (`internal/prime`):**
    - `PrimeOptions` expandido com `Warm5h bool`.
    - Ao executar com `--warm-5h`, filtra exclusivamente os buckets de 5 horas ociosos, valida a integridade da cota semanal pai para evitar exaustão indevida e dispara o prompt leve para adiantar o ciclo de 5 horas.
    - `BucketStatusReport` e `BucketPrimeResult` enriquecidos com `window_type`, `can_warm` e `warm_type` (`"proactive_5h"` vs `"cycle_reset"`).
  - **CLI e API REST (`internal/cmd/prime.go` e `internal/server/routes.go`):**
    - Flag `--warm-5h` adicionada à CLI com reset determinístico.
    - Endpoints `POST /api/v1/profiles/{name}/prime` e `POST /api/v1/prime` suportam `warm_5h: true` via JSON e query param, emitindo eventos de progresso SSE detalhados.

### 2026-09-25 [Task 07.3] Priming e Aquecimento de Cotas via API com Emissão de Progresso

- **Contexto:** Agregadores, ferramentas de telemetria externa e agentes de IA necessitam de endpoints REST para consultar o status de priming e agendamento de watchdog (`GET /api/v1/profiles/{name}/prime`, `GET /api/v1/prime`), bem como acionar o aquecimento proativo de cotas (`POST /api/v1/profiles/{name}/prime`, `POST /api/v1/prime`) com feedback de progresso em tempo real via Server-Sent Events (SSE). Além disso, a CLI precisava do suporte a `--json` em `multigravity prime` (Regras de Ouro #6 e #7 do `AGENTS.md`).
- **Decisões Técnicas:**
  - **Desacoplamento Modular do Motor de Priming (`internal/prime`):**
    - `types.go`: structs estruturadas `ProfilePrimeStatus`, `BucketStatusReport`, `ProfilePrimeResult`, `BucketPrimeResult`, e evento de progresso `PrimeProgressEvent` (`stage`, `bucket`, `message`, `details`, `timestamp`).
    - `status.go`: implementação pura de `GetProfilePrimeStatus` e `GetAllProfilesPrimeStatus`. Em caso de Language Server offline, retorna dados consolidados e estado salvo sem quebrar a API HTTP (com `ServerMode: "offline"`).
    - `execute.go`: implementação pura de `ExecutePrime(opts PrimeOptions, cb ProgressCallback)` emitindo marcos em tempo real (`initializing`, `server_ready`, `checking`, `jitter_waiting`, `dispatching`, `primed`, `skipped`, `aborted`, `completed`).
    - `engine.go`: preservação 100% retrocompatível do comando CLI `RunPrime`, delegando para o callback de terminal estilizado com suporte a `--json`.
    - `SetTestHooks`: exportação de closures herméticas para simular servidores ativos, instâncias headless e `QuotaClient` sem dependência de processos reais do Antigravity.
  - **Endpoints REST (`internal/server/routes.go`):**
    - `GET /api/v1/profiles/{name}/prime` e `/api/profiles/{name}/prime`: status granular por perfil.
    - `GET /api/v1/prime` e `/api/prime`: status consolidado de todos os perfis.
    - `POST /api/v1/profiles/{name}/prime` e `/api/profiles/{name}/prime`: aciona priming com payload opcional (`force`, `check`, `include_5h`, `no_jitter`, `max_jitter`), transmitindo eventos SSE (`event: prime`) e finalizando com evento de ação (`event: action`, `action: prime`).
    - `POST /api/v1/prime` e `/api/prime`: aciona priming em lote.
  - **Flag `--json` na CLI (`internal/cmd/prime.go`):**
    - Suporte a `multigravity prime [profile] --status --json` e `multigravity prime [profile] --check --json`.
    - Adicionado reset determinístico de todas as variáveis de flag no `defer` para evitar vazamento de estado em chamadas sucessivas.
  - **Higiene em `profile.ListProfiles` (`internal/profile/profile.go`):**
    - Ignora deterministicamente quaisquer diretórios ocultos (prefixo `.`) ao listar perfis, evitando que pastas do sistema como `.local` ou `.cache` sejam computadas como perfis se `MULTIGRAVITY_HOME` coincidir com a home.

### 2026-09-25 [Pesquisa Técnica] Mapeamento do Endpoint Interno CloudCode PA (Cotas Google Cloud)

- **Fonte:** Engenharia reversa documentada no repositório público [`cryptogabovz/gestor-multigravity`](https://github.com/cryptogabovz/gestor-multigravity) (projeto derivado do *Antigravity Assistant*).
- **Status:** **Incerto / Não-oficial (Sob Observação).** Não implementado no núcleo do Multigravity devido a restrições rígidas de custódia de credenciais.
- **Contexto e Mecânica:**
  - O Antigravity utiliza os serviços de backend do Google Cloud Code / Gemini Code Assist para obter os limites de cota da conta conectada.
  - Endpoints identificados:
    1. **Descoberta do Projeto do Usuário:**
       - `POST https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist`
       - Headers: `Authorization: Bearer <access_token>`, `User-Agent: antigravity/1.11.3 Darwin/arm64`
       - Payload: `{"metadata": {"ideType": "ANTIGRAVITY"}}`
       - Retorno: Campo `cloudaicompanionProject` com o ID interno do projeto (ex: `anthropic-xxxx`).
    2. **Consulta Direta de Cotas e Modelos:**
       - `POST https://cloudcode-pa.googleapis.com/v1internal:fetchAvailableModels`
       - Headers: `Authorization: Bearer <access_token>`, `User-Agent: antigravity/1.11.3 Darwin/arm64`
       - Payload: `{"project": "<projectId>"}`
       - Retorno: Estrutura JSON `models` contendo cada modelo (`gemini-2.5-pro`, `gemini-2.5-flash`, `claude-3-5-sonnet`, etc.) com `quotaInfo.remainingFraction` (0.0 a 1.0) e `quotaInfo.resetTime` (timestamp ISO-8601).
- **Ressalvas, Incertezas e Invariantes:**
  - **API Privada (`v1internal`):** Não é um contrato público da Google. Pode sofrer alterações de schema, bloqueio de User-Agent ou revogação de escopos OAuth sem aviso.
  - **Custódia de Tokens (Regra de Ouro #5):** Essa chamada requer um `access_token` válido gerado por um `refresh_token` do Google OAuth. O `multigravity-cli` adota o princípio de **zero custódia de credenciais** (deixando as contas isoladas no armazenamento nativo do Antigravity/SO). Adotar este endpoint exigiria armazenar ou manipular segredos OAuth, o que contraria as diretrizes atuais.
  - **Utilidade Futura:** Registrado como possível fallback leve para telemetria externa caso um dia o usuário opte expressamente por um modo de monitoramento remoto sem processo local do `language_server`.

### 2026-09-25 [Pesquisa Técnica] Benchmark e Oportunidades Futuras: blugthek/Multigravity (OAuth2 PKCE, CLI Contratos & Subagents)

- **Fonte:** Repositório público no GitHub: [`https://github.com/blugthek/Multigravity`](https://github.com/blugthek/Multigravity)
  - **Autor:** BLUGTHEK
  - **Commit de Referência:** `4fef9e406bf1d8aabc4238e13eed263e9d2eb78e` (Branch `main`, Setembro/2026)
  - **Licença:** MIT
  - **Descrição:** *"Multi-account agy subagent runner. Run prompts across multiple Google accounts with separate quota consumption."*
- **Status:** **Mapeamento de Referência Técnica / Backlog de Oportunidades Futuras.**
- **Contexto:** Análise aprofundada do código-fonte identificou 5 pontos e mecanismos técnicos com alto valor de aproveitamento para o `multigravity-cli`:

#### 1. Parâmetros Canônicos do Fluxo Google OAuth2 PKCE para Antigravity
- **Descoberta:** O arquivo `oauth.py` documenta os parâmetros de autorização e troca de tokens que o Antigravity utiliza para vincular contas Google:
  - **OAuth Client ID:** `1071006060591-tmhssin2h21lcre235vtolojh4g403ep.apps.googleusercontent.com`
  - **Endpoints:**
    - Auth: `https://accounts.google.com/o/oauth2/auth`
    - Token: `https://oauth2.googleapis.com/token`
    - UserInfo: `https://www.googleapis.com/oauth2/v2/userinfo`
  - **Redirect URI:** `http://127.0.0.1:{port}/callback`
  - **Escopos Canônicos:**
    - `https://www.googleapis.com/auth/cloud-platform`
    - `https://www.googleapis.com/auth/userinfo.email`
    - `https://www.googleapis.com/auth/userinfo.profile`
    - `https://www.googleapis.com/auth/cclog`
    - `https://www.googleapis.com/auth/experimentsandconfigs`
    - `https://www.googleapis.com/auth/aicode`
    - `openid`
  - **Estrutura do Payload de Credencial no Keyring:**
    - Target: `gemini:antigravity`, Usuário: `antigravity`, `oauth_client_key: "antigravity_enterprise"`
    - Blob UTF-8:
      ```json
      {
        "token": {
          "access_token": "<token>",
          "token_type": "Bearer",
          "refresh_token": "<refresh>",
          "expiry": "2026-09-25T17:00:00+00:00"
        },
        "auth_method": "consumer"
      }
      ```
- **Aplicabilidade Futura:**
  - Viabiliza um comando `multigravity login <profile>` diretamente via CLI (iniciando listener HTTP efêmero local, abrindo o navegador com desafio PKCE S256 e gravando a credencial autenticada no ambiente isolado do perfil).
  - Permite provisionar e autenticar perfis headless sem nunca precisar iniciar a interface gráfica do Electron/IDE apenas para fazer login.
  - **Guarda de Segurança (Regra #5):** A credencial deve permanecer estritamente confinada ao vault/diretório do respectivo perfil, nunca centralizando refresh tokens em texto puro em arquivos compartilhados.

#### 2. Despacho Concorrente de Tarefas / Subagentes em Lote (`multigravity exec --all`)
- **Descoberta:** O `blugthek/Multigravity` propõe um comando `run-all "<prompt>"` para queimar cotas separadas entre contas ao despachar prompts sequenciais para a CLI `agy`.
- **Limitação no Modelo Deles:** Como operam sobrescrevendo o keyring global do Windows, só conseguem rodar sequencialmente (com risco de corromper a conta primária se abortados).
- **Vantagem e Oportunidade no Nosso Repositório:**
  - O `multigravity-cli` possui isolamento real de processos e sistema de arquivos via `--user-data-dir`, `--extensions-dir` e `$HOME` / `%USERPROFILE%` dedicados por perfil.
  - Isso possibilita criar uma evolução de primeira classe: `multigravity exec [perfil|--all] "<prompt>"`, capaz de despachar tarefas **em paralelo verdadeiro** através de múltiplos perfis (usando worker pool em Go), agregando as respostas estruturadas em JSON.
  - Integra-se perfeitamente ao motor de priming (`internal/prime`) e à API HTTP (`multigravity serve`), permitindo que orquestradores externos distribuam cargas de trabalho por perfis com cotas saudáveis.

#### 3. Contrato de Invocação e Schema Machine-Readable da CLI `agy`
- **Descoberta:** O arquivo `runner.py` mapeia os argumentos exatos e o schema JSON de saída retornado pelo binário `agy`:
  - **Flags de Execução Headless:**
    - `-p <prompt>`: Prompt textual a ser executado pelo subagente.
    - `--print-timeout <dur>`: Timeout de execução (ex: `120s`).
    - `--output-format json`: Força a saída em JSON estruturado via stdout.
    - `--dangerously-skip-permissions`: Suprime confirmações interativas de ferramentas e sandbox.
  - **Contrato de Retorno JSON:**
    ```json
    {
      "response": "Resposta do modelo...",
      "usage": {
        "total_tokens": 18505
      },
      "duration_seconds": 12.3
    }
    ```
- **Aplicabilidade Futura:**
  - Valida o contrato canônico para orquestração de subagentes via CLI `agy`, servindo de base para o parser de telemetria de tokens consumidos em tarefas headless do Multigravity.

#### 4. Renovação Direta de Tokens OAuth (`grant_type=refresh_token`) e Verificação de Identidade
- **Descoberta:** Os arquivos `oauth.py` e `accounts.py` implementam o ciclo de vida completo de tokens:
  - **Refresh Flow:** `POST https://oauth2.googleapis.com/token` com dados `client_id`, `refresh_token` e `grant_type=refresh_token` renova o access token sem interação humana.
  - **User Info API:** `GET https://www.googleapis.com/oauth2/v2/userinfo` com `Authorization: Bearer <access_token>` retorna imediatamente `{ "email": "...", "name": "..." }`.
- **Aplicabilidade Futura:**
  - Permite verificar a identidade ativa de um perfil e a validade de sua sessão diretamente via HTTP, sem precisar inicializar o pesado processo do `language_server` do Antigravity.

#### 5. Mapeamento do Ecossistema Comunitário Legado (`cockpit-tools`)
- **Descoberta:** O `accounts.py` implementa migração a partir de `~/.antigravity_cockpit/accounts/*.json` (ferramenta comunitária anterior em Rust).
- **Aplicabilidade Futura:**
  - Caso seja necessário oferecer retrocompatibilidade ou migração para usuários vindos do `cockpit-tools` ou de ferramentas antigas, o diretório e formato JSON dos dados já estão identificados.

### 2026-09-25 [Task 07.2] Mutação de Compartilhamento Dinâmico via API (Toggle de MCP, Skills, Config, Git/GitHub)

- **Contexto:** Agregadores de telemetria externa, extensões e agentes autônomos necessitam de endpoints REST para consultar e mutar dinamicamente os vínculos de compartilhamento e isolamento de recursos (`mcp`, `skills`, `config`, `gh`/`github`, e `git`/`dotfiles`) por perfil, com emissão de eventos em tempo real via Server-Sent Events (SSE).
- **Decisões Técnicas:**
  - **Suporte Canônico a Git / Dev Dotfiles:**
    - Formalizadas no pacote `profile` as funções `GetDotfilesStatus` (`GetGitStatus`), `DotfilesShare` (`GitShare`) e `DotfilesIsolate` (`GitIsolate`).
    - Integração de `git` em `GetAllSharingStatus`, expandindo a listagem para 5 recursos canônicos (`mcp`, `skills`, `config`, `gh`, `git`).
  - **Despachante Unificado de Compartilhamento (`profile.SetResourceSharing`):**
    - Suporta recursos `mcp`, `skills`, `config`, `gh` (e alias `github`), `git` (e alias `dotfiles`).
    - Suporta ações `share` (aliases `shared`, `true`, `enable`, `on`), `isolate` (aliases `isolated`, `false`, `disable`, `off`), `toggle` (alterna automaticamente o modo com base no estado atual) e `seed` (exclusivo para `config`).
  - **Endpoints REST (`routes.go`):**
    - `POST` / `PUT` `/api/v1/profiles/{name}/sharing/{resource}` e `/api/profiles/{name}/sharing/{resource}`: mutação granular de recurso com payload flexível (`action`, `mode`, `shared`).
    - `POST` / `PUT` `/api/v1/profiles/{name}/sharing` e `/api/profiles/{name}/sharing`: mutação em lote (batch) aceitando mapa de recursos (`{"mcp": "share", "git": "isolate"}`) ou lista de objetos (`[{"resource": "mcp", "action": "share"}]`).
    - `POST` `/api/v1/profiles/{name}/sharing/config/seed`: conveniência para semeadura imediata de permissões padrão read-only no `config.json`.
    - Atualizado CORS para incluir o método HTTP `PUT` em `Access-Control-Allow-Methods`.
  - **Emissão de Eventos SSE (`handleEvents`):**
    - Mutação individual emite evento `action` com `action: "sharing"`, `profile: name`, `resource: resource`, `mode: mode`, `status: "updated"`.
    - Mutação em lote emite evento `action` com `action: "sharing"`, `profile: name`, `status: "updated"`, `batch: true`.
  - **Documentação e Testes:**
    - `skills/multigravity/SKILL.md` atualizado com o catálogo dos novos endpoints.
    - Testes unitários completos adicionados em `internal/profile/sharing_test.go` (`TestGitStatusShareIsolate`, `TestSetResourceSharing`) e `internal/server/server_test.go` (`TestSharingMutationEndpoints`, `TestSharingMutationSSE`, e validação de `PUT` em `TestCORSHeaders`).

### 2026-09-25 [Task 07.1] Implementação de Endpoints de Mutação no Servidor HTTP (multigravity serve)

- **Contexto:** Agregadores, ferramentas de telemetria externa e agentes de IA necessitam de endpoints REST para controlar o ciclo de vida completo de instâncias e perfis (`new` com `--auth-only`, `delete`, `launch`, `stop`, `restart`, `rename`) sem recorrer a comandos de shell locais ou correr riscos de concorrência com instâncias ativas da IDE.
- **Decisões Técnicas:**
  - **Rotas com Prefixo Duplo (`/api/` e `/api/v1/`):**
    - Todos os novos endpoints e aliases preexistentes foram registrados simultaneamente sob `/api/profiles...` e `/api/v1/profiles...`, garantindo flexibilidade total para diferentes bibliotecas de frontend e clientes HTTP.
  - **CORS Estendido para Mutações:**
    - Atualizado o header `Access-Control-Allow-Methods` para `"GET, POST, DELETE, OPTIONS, HEAD"`, permitindo preflights bem-sucedidos em browsers.
  - **Criação de Perfis (`POST /api/profiles`):**
    - Payload flexível em `CreateProfileRequest`: aceita `auth_only`, `auth-only` (kebab-case) e `shared`, mapeando para o layout leve de ~2 MB introduzido na Task [11.1].
    - Suporta flags de isolamento granular (`isolated_dotfiles`, `isolated_mcp`, `isolated_skills`, `isolated_config`, `isolated_gh`), theming (`color`) e modelo inicial (`from_template`).
    - Retorna `201 Created` com o payload estruturado de `profile.ProfileInfo`. Emite evento SSE `create` e notifica observadores via `broker.CheckProfilesChange()`.
    - Respostas de erro padronizadas: `400 Bad Request` (nome inválido/ausente, cor ou template inexistente) e `409 Conflict` (perfil já existente).
  - **Exclusão Segura com Proteção contra Concorrência (`DELETE /api/profiles/:name`):**
    - Checagem ativa de execução (`IsProfileRunning`). Se o perfil estiver aberto e `force` não for passado (via query `?force=true` ou JSON `{"force": true}`), aborta com `409 Conflict` para prevenir corrupção de bancos SQLite (Regra de Ouro #4 do `AGENTS.md`).
    - Retorna `404 Not Found` para perfis inexistentes e `200 OK` ao deletar. Emite evento SSE `delete` e dispara `broker.CheckProfilesChange()`.
  - **Ações de Ciclo de Vida (`launch`, `restart`, `rename`):**
    - `POST /api/profiles/:name/launch`: recebe `{"args": [...]}` opcional e aciona `LaunchProfile`. Emite evento SSE `launch`.
    - `POST /api/profiles/:name/restart`: fecha graciosamente o perfil e o relança com os argumentos fornecidos. Emite evento SSE `restart`.
    - `POST /api/profiles/:name/rename`: recebe `{"new_name": "..."}`, valida sintaxe, verifica se está em execução (retornando `409 Conflict` se ativo) e renomeia diretório e atalhos de desktop. Emite evento SSE `rename`.
  - **Testabilidade Limpa em Go:**
    - Exportadas funções `profile.SetLaunchProfileFn` e `profile.SetGetProfilePIDsFn` retornando closures de restauração (RAII idiomático em testes), viabilizando simulações herméticas de execução e inicialização sem disparar janelas Electron reais.

### 2026-09-25 [Task 11.5] Landing Page Estática e Onboarding Visual Interativo para Iniciantes (GitHub Pages / Showcase)

- **Contexto:** Iniciantes e novos usuários necessitavam de uma vitrine interativa na web para conhecer o Multigravity, testar comandos visualmente, compreender a economia de disco de perfis Auth-Only (~2 MB) e obter instruções de instalação guiadas por sistema operacional sem barreiras.
- **Decisões Técnicas:**
  - **Hospedagem Nativa em `docs/`:** Estruturado o portal diretamente em `docs/` (`index.html`, `css/style.css`, `js/app.js`, `assets/`), padrão suportado nativamente pelo GitHub Pages a partir da branch principal sem requerer pipelines adicionais de build.
  - **Zero Dependências Externas (Pure Vanilla):** Construído em HTML5 semântico, CSS3 moderno (com variáveis de tema, glassmorphism e responsive grid/flexbox) e JavaScript vanilla sem frameworks pesados, garantindo carregamento instantâneo (< 100ms) e suporte a navegação offline.
  - **Design Engineering & Motion Craft (Emil Kowalski):**
    - Feedback de pressão tátil em botões: `transform: scale(0.97)` em `:active`.
    - Curvas de easing customizadas: `--ease-out: cubic-bezier(0.23, 1, 0.32, 1)` e `--ease-in-out: cubic-bezier(0.77, 0, 0.175, 1)`.
    - Transições suaves e respeitosas a acessibilidade (`@media (prefers-reduced-motion: reduce)`).
    - Suporte a tema escuro/claro com detecção automática do SO e persistência via `localStorage`.
  - **Simulador Interativo de Terminal:**
    - 4 cenários dinâmicos com digitação de comandos em tempo real e saída em cores ANSI:
      1. Menu interativo TUI (`multigravity`) com indicadores de status `● running` / `○ idle` e atalhos.
      2. Perfil Auth-Only (`new dev --auth-only --color blue`) com symlinks e pegada de 1.8 MB.
      3. Telemetria de Cotas de IA (`quota dev`) com barras ASCII multi-bucket (Gemini e 3P Claude/GPT) e watchdog prime.
      4. Limpeza de Caches (`clean --all`) com detecção de instâncias ativas e liberação segura de gigabytes.
    - Controles de reprodução: alternância de abas, replay e cópia de comando para a área de transferência.
  - **Onboarding Multi-SO (Linux, macOS, Windows):**
    - Abas específicas com comandos de 1 linha (`install.sh` / `install.ps1`), criação de primeiro perfil, lançamento via atalho nativo (.desktop/.app/.lnk) e verificação.
  - **Calculadora e Comparador Visual:**
    - Slider dinâmico (1 a 10 perfis) demonstrando o comparativo de consumo em disco (Full: N × 500 MB vs Auth-Only: N × 2 MB) com cálculo de economia acumulada (até 99.6%).
    - Tabela comparativa Full vs Auth-Only destacando o que é compartilhado vs o que é estritamente isolado.
  - **Explorador de Comandos e FAQ Interativo:**
    - Filtros por categoria e busca instantânea com botões de cópia.
    - FAQ acessível em `<details>` e `<summary>` com rotação suave de ícone SVG.
  - **Integração no README:**
    - Badges e links em destaque adicionados ao `README.md` e `README.pt-br.md` apontando para `https://yegear1.github.io/multigravity-cli/`.

### 2026-09-25 [Task 11.4] Matriz Comparativa Full vs Auth-Only e Documentação no README.md

- **Contexto:** Com a introdução do suporte a perfis Auth-Only (`--auth-only` / `--shared`) na Task 11.1 e medição de tamanho na Task 11.2, os usuários necessitavam de uma referência clara e didática no `README.md` comparando as dimensões operacionais (Extensões, Configurações, Isolamento de Login, Cotas e Consumo de Disco: ~500 MB vs ~2 MB) para guiar a escolha do tipo de perfil.
- **Decisões Técnicas:**
  - **Matriz Comparativa Didática:** Estruturada tabela no `README.md` e `README.pt-br.md` cobrindo 11 aspectos: comando de criação, pegada inicial de disco (~500 MB vs ~2 MB), isolamento de contas Google, cotas/limites independentes, histórico de IA, extensões da IDE (isoladas vs symlink), configurações e atalhos (`settings.json`, keybindings, snippets), theming visual (`--color` com desacoplamento seguro), dotfiles dev (`.gitconfig`, SSH), GitHub CLI e melhor cenário de uso.
  - **Guia Rápido de Escolha ("Which Profile Type Should I Choose?"):** Seção orientando quando usar Perfil Completo (stacks divergentes, linters/extensões distintas) vs Perfil Auth-Only (rotação de cotas entre contas Google, ferramental idêntico, consumo residual de disco).
  - **Reconhecimento Open Source:** Link direto adicionado para o repositório `Pulkit7070/multigravity-pro` na seção de Créditos e Agradecimentos, honrando a inspiração do paradigma de perfis leves e onboarding didático.


### 2026-09-25 [Task 11.3] Ícone Embutido (//go:embed) e Associação Automática em Atalhos Desktop

- **Contexto:** Os atalhos de desktop gerados para cada perfil dependiam de ícones externos do sistema (`Icon=antigravity` no Linux podia ficar genérico se o pacote da IDE não estivesse nos temas de ícones do sistema) ou download manual de `icon.icns` durante o `install.sh`. No Windows e no macOS, faltava gravação nativa de ícones padrão nos bundles e atalhos `.lnk`. Inspirado na facilidade de uso do fork `Pulkit7070/multigravity-pro`.
- **Decisões Técnicas:**
  - **Empacotamento via `//go:embed`:**
    - Armazenados em `internal/shortcut/assets/`:
      - `icon.icns`: Mac OS X icon multi-resolução (~710 KB).
      - `icon.png`: alta resolução PNG (512x512, 119 KB) extraído do canal `ic09` do ICNS, otimizado para o padrão XDG Desktop Entry Specification.
      - `icon.ico`: ícone padrão Microsoft Windows (256x256 PNG encapsulado, ~40 KB) compatível com o Windows Shell / Explorer.
    - Exportadas funções `GetIconICNS()`, `GetIconPNG()`, `GetIconICO()` e `HasEmbeddedIcon()`.
  - **Gravação e Associação Automática nos Atalhos:**
    - **macOS (`createShortcutDarwin`):** Grava `Contents/Resources/icon.icns` diretamente dentro de `Multigravity <profile>.app`, imediatamente associado pelo `CFBundleIconFile: icon` no `Info.plist`.
    - **Linux (`createShortcutLinux`):** Função `EnsureLinuxIcon()` extrai o ícone para `~/.local/share/multigravity/icon.png` (ou caminho hermético de teste) e injeta `Icon=<caminho_absoluto>` no arquivo `multigravity-<profile>.desktop`.
    - **Windows (`createShortcutWindows`):** Função `EnsureWindowsIcon()` extrai o ícone para `%APPDATA%\multigravity\icon.ico` e define `$Shortcut.IconLocation = "<caminho_absoluto>, 0"` via COM `WScript.Shell`, mantendo fallback para `app.FindApp()`.
  - **Auto-Contenção no Diagnóstico (`doctor`):**
    - `internal/doctor/doctor.go` atualizado para checar `shortcut.HasEmbeddedIcon()`, reportando `Application Icon: Embedded (built-in)` com `StatusOK` (zero warnings), dispensando dependência de rede em instalações offline.
  - **Hermeticidade em Testes:**
    - `shortcut_test.go` e `doctor_test.go` validam a presença dos assets embutidos, a gravação de `icon.png`, `icon.icns` e `icon.ico`, e os caminhos gerados nos arquivos `.desktop` e bundles `.app` sem poluir as pastas do usuário host.

### 2026-09-25 [Task 11.2] Contrato Machine-Readable (--json) e Consulta Granular em multigravity status

- **Contexto:** O comando `multigravity status` exibia apenas saída tabular estilizada em ANSI. UIs locais, agregadores de telemetria e agentes de IA necessitam de dados estruturados com tipagem precisa, incluindo tamanho em bytes, estado de execução, tipo do perfil e data de último uso (Regra de Ouro #7 do `AGENTS.md`).
- **Decisões Técnicas:**
  - **Cálculo de Tamanho em Bytes (`GetDirSizeBytes` e `SizeBytes`):** Implementada função pura `GetDirSizeBytes(dir string) int64` em `internal/profile/clean.go` utilizando `filepath.Walk`. Não segue links simbólicos para diretórios externos (evitando computar o diretório de extensões do host nos perfis `auth-only`), computando a soma exata de bytes de arquivos locais do perfil. Exportada também `FormatBytes(b int64) string`.
  - **Enriquecimento de `ProfileInfo`:** Adicionado o campo `SizeBytes int64` (`json:"size_bytes"`) à struct `ProfileInfo` em `internal/profile/profile.go`, preenchido deterministicamente em `GetProfile`. Como consequência positiva, as rotas `/api/v1/profiles` e `/api/v1/profiles/{name}` do servidor HTTP passam a fornecer a contagem de bytes automaticamente sem quebras de contrato.
  - **Flag `--json` e Consulta por Perfil em `statusCmd`:**
    - Atualizado `statusCmd` em `internal/cmd/status.go` para aceitar argumento opcional `status [profile]` (`Args: cobra.MaximumNArgs(1)`).
    - Quando chamado sem argumentos com `--json`: serializa o array `[]profile.ProfileInfo` com indentação.
    - Quando chamado com `[profile]` e `--json`: serializa o objeto individual `profile.ProfileInfo`.
    - Saída tabular de texto 100% preservada na ausência da flag `--json`, inclusive suportando filtro de perfil único.
    - Implementado reset limpo de flags (`cmd.Flags().Set("json", "false")` e `statusJSON = false`) via `defer` no `RunE` para garantir hermeticidade de execução sucessiva em testes e sessões interativas.
  - **Autocompletion Dinâmico:** Registrado `statusCmd.ValidArgsFunction = profileArgsCompletion` em `internal/cmd/root.go`.


### 2026-09-25 [Task 11.1] Suporte a Perfis Auth-Only Reais (--auth-only / --shared) com Symlinks de Host

- **Contexto:** Perfis isolados alocavam pastas vazias em `~/.antigravity/extensions` e forçavam a reinstalação de centenas de megabytes de extensões para cada nova conta. Usuários necessitavam de alternância rápida de contas com consumo residual de disco (~2 MB).
- **Decisões Técnicas:**
  - **Flag `--auth-only` e Alias `--shared`:** Adicionada flag `--auth-only` em `internal/cmd/new.go` mantendo `--shared` como alias totalmente funcional e retrocompatível.
  - **Symlink Seguro de Extensões (`LinkHostExtensions`):** Se o perfil possuir os sentinelas `.auth_only` ou `.shared`, vincula via symlink `$REAL_HOME/.antigravity/extensions` ao perfil. Preserva diretórios com arquivos existentes caso o usuário já possua extensões manuais, com fallback gracioso se o host ainda não possuir a pasta de extensões.
  - **Symlink de Configurações de Editor (`LinkUserSettings`):** Vincula via links simbólicos `settings.json`, `keybindings.json` e o diretório `snippets` de `GetUserDataDir(REAL_HOME)/User` para o diretório `User` do perfil.
  - **Preservação de Theming e Desacoplamento:** O desacoplamento seguro em `ApplyProfileColor` (`internal/profile/color.go`) desfaz o link de `settings.json` ao aplicar cores customizadas, preservando a instalação global intacta enquanto mantém `keybindings.json`, `snippets` e `extensions/` compartilhados.
  - **Classificação em `ProfileInfo`:** Perfis com `.auth_only` ou `.shared` são reportados com `Type: "auth-only"` em `status`, `list --json`, TUI e endpoints HTTP.
  - **Higiene em Templates:** Ao salvar um perfil como modelo (`template save`), remove os sentinelas `.shared` e `.auth_only` para evitar contaminação de novos perfis.

### 2026-09-25 [Benchmark & Backlog] Incorporação de Ideias do Pulkit7070/multigravity-pro e Análise de Licença

- **Contexto:** Análise comparativa entre o nosso repositório (`yegear1/multigravity-cli`) e o fork `Pulkit7070/multigravity-pro`.
- **Análise de Licença:**
  - O repositório `Pulkit7070/multigravity-pro` é distribuído sob licença **MIT**, assim como o `sujitagarwal/multigravity-cli` e o nosso projeto.
  - A licença MIT é irrestrita quanto à reutilização, modificação, adaptação de ideias e sublicenciamento, exigindo apenas aviso de copyright quando houver cópia literal de código.
  - Como a nossa stack é nativa em Go (enquanto a do Pulkit é em Bash/PowerShell), trata-se de reimplementação técnica e evolução conceitual. Manteremos crédito explícito a `Pulkit7070` na seção `Credits & Acknowledgments` do `README.md` e nos logs de tarefas, em conformidade com as melhores práticas open source.
- **Melhorias Mapeadas no Backlog (Épico 11):**
  - **[11.1]** Suporte a `--auth-only` no comando `new` com symlink real de `extensions` e `settings.json/keybindings.json/snippets` do host, garantindo perfil de ~2 MB para alternância rápida de logins.
  - **[11.2]** Adicionar `--json` ao `multigravity status` (conforme Regra de Ouro #7).
  - **[11.3]** Embutir ícone padrão (`icon.icns` / `.ico`) nos atalhos desktop gerados.
  - **[11.4]** Matriz comparativa didática (Full vs Auth-Only) no `README.md`.
  - **[11.5]** Portal / Landing page visual e interativa de documentação (GitHub Pages).

### 2026-09-12 [Task 00.1] Calibração de Template e Definição da Estratégia de Isolamento

- **Contexto:** O projeto é uma CLI em Bash e PowerShell para gerenciar instâncias isoladas do Antigravity IDE.
- **Decisão:** Manter o isolamento via export de `HOME` para o diretório do perfil (já que é o único meio conhecido de isolar `~/.gemini` e `~/.antigravity`), porém vincular via symlinks os dotfiles essenciais (`.gitconfig`, `.ssh`) para não quebrar Git/SSH no terminal integrado.
- **Alternativas consideradas:** Tentar usar apenas `--user-data-dir` sem mudar `HOME` foi descartado porque o assistente Gemini do Antigravity continua lendo/gravando no `$HOME` global, quebrando o isolamento de credenciais e histórico de IA.

### 2026-09-12 [Task 01.1] Preservação de Dotfiles de Dev (Git & SSH)

- **Contexto:** Usuários de perfis isolados tinham seus commits quebrados por falta de `.gitconfig` e clones/pushes falhando por falta de `~/.ssh/`.
- **Decisão:** Criar função `link_dev_dotfiles` (Bash) e `Link-DevDotfiles` (PowerShell) chamada tanto na criação de novos perfis quanto no lançamento de perfis existentes. Suportada a flag `--isolated-dotfiles` para quem explicitamente optar por isolamento total.
- **Windows:** No Windows, o diretório `.ssh` é vinculado via `Junction`, permitindo funcionamento sem requerer privilégios de Administrador.

### 2026-09-12 [Task 01.2] Suporte ao binário `agy` (Antigravity 2.0 / CLI)

- **Contexto:** Antigravity 2.0 introduziu o binário e alias de CLI `agy`, além da antiga nomenclatura `antigravity`.
- **Decisão:** Adicionar `agy` na busca automática de executáveis no Linux/macOS/Windows e suportar a variável de ambiente `AGY_APP` em conjunto com `MULTIGRAVITY_APP`.

### 2026-09-12 [Task 02.1] Theming Visual por Perfil via `--color` e Comando `multigravity color`

- **Contexto:** Usuários com várias janelas abertas de perfis diferentes (ex: pessoal vs trabalho) não tinham indicação visual de qual perfil pertencia cada janela.
- **Decisão:** Injetar configurações em `workbench.colorCustomizations` dentro do `settings.json` do perfil. Modificação totalmente não-destrutiva (preserva fontes, extensões e outras preferências do usuário).
- **Perfis Compartilhados (--shared):** Se `settings.json` for um link simbólico para a instalação global, ao aplicar cor a CLI desacopla o arquivo no perfil com segurança, evitando tingir a instalação global do usuário.
- **Paleta de Cores:** Suporte a códigos arbitrários `#RRGGBB` e a nomes amigáveis: `blue`, `green`, `red`, `purple`, `orange`, `cyan`, `pink`, `emerald`, `indigo`, `slate`, etc.

### 2026-09-12 [Task 02.2] Ciclo de Vida (`stop`, `restart`) e Travas contra Concorrência

- **Contexto:** Perfis não tinham como ser fechados via CLI. Se o usuário deletasse ou renomeasse um perfil com a IDE aberta, os arquivos eram movidos ou apagados em tempo de execução, corrompendo os bancos SQLite do VS Code.
- **Decisão:** Criar `stop` e `restart` com desligamento gracioso (SIGTERM com timeout de 3 segundos para flush do SQLite) e fallback `--force`. Adicionada trava ativa que impede `delete` e `rename` caso o perfil esteja em execução.
- **Armadilha evitada:** Em `get_profile_pids`, processos que coincidam com `$$` ou `$PPID` são ignorados no awk para evitar que chamadas em scripts ou subshells matem o próprio terminal ou processo pai.

### 2026-09-12 [Task 02.3] Otimização de Backup (`export` sem caches) e Comando `multigravity clean`

- **Contexto:** Perfis acumulavam centenas de megabytes ou gigabytes em diretórios de cache do Chromium/Electron (`GPUCache`, `Code Cache`, `Cache`, `DawnGraphiteCache`, `Crashpad`, `logs`, `.cache`, etc.), fazendo com que o `multigravity export` gerasse tarballs/zips gigantescos e lentos, além de desperdiçar disco.
- **Decisão:**
  - **Exclusão no `export`:** Adicionada exclusão padrão e cirúrgica de caches voláteis via `--exclude` no `tar` (Linux/macOS) e via staging temporário limpo com `Compress-Archive` (Windows). Adicionada a flag opcional `--include-cache` para usuários que explicitamente queiram preservar tudo no backup.
  - **Comando `clean`:** Implementado `multigravity clean <perfil|--all>` com medição amigável de espaço liberado antes/depois (`du -sh` / `Get-FolderSize`) e trava ativa de segurança contra concorrência (`is_profile_running` / `Test-ProfileRunning`), impedindo limpar caches com a IDE em execução.

### 2026-09-12 [Task 03.1] Menu TUI / Seletor Interativo sem Argumentos

- **Contexto:** Ao executar apenas `multigravity`, o utilitário exibia a tela de ajuda e saía com código 1. Em um terminal interativo com múltiplos perfis configurados, a experiência ideal é uma seleção rápida.
- **Decisão:**
  - **Detecção de Terminal:** Usar `[ -t 0 ] && [ -t 1 ]` no Bash e `![System.Console]::IsInputRedirected` no PowerShell. Redirecionamentos, pipes, scripts e automações continuam recebendo o `usage` e exit code 1 intactos.
  - **TUI Leve e Sem Dependências:** Renderiza lista numerada dos perfis com indicador de status (`● running` em verde / `○ idle` em cinza), tipo (`shared`/`isolated`) e cor configurada. Suporta atalhos diretos: número `[1..N]` ou nome do perfil para lançar, `[n]` para criar novo e `[q]` para sair com exit code 0.

### 2026-09-12 [Task 03.2] Migração e Exportação/Importação Granular de Chats de IA

- **Contexto:** Usuários queriam transferir conversas, análises e planos de IA entre perfis ou entre máquinas sem expor credenciais e tokens de acesso OAuth, além de poder inspecionar os tópicos de chats locais.
- **Decisão:**
  - **Mapeamento de Dados da IA:** No Antigravity, os chats e artefatos ficam em `~/.gemini/antigravity/` (especificamente `conversations/*.db`, `brain/<uuid>/*`, `annotations/<uuid>.pbtxt` e `knowledge/`).
  - **Sanitização Absoluta:** O empacotamento exclui ativamente arquivos de credenciais como `jetski-standalone-oauth-token`, `*token*`, `*oauth*`, `*auth*`, `*credential*` e `installation_id`.
  - **Mescla Não-Destrutiva:** No `import`, os chats e brains recebidos são mesclados dentro do perfil de destino sem deletar conversas preexistentes.
  - **Trava de Segurança:** A importação bloqueia a operação se o perfil de destino estiver aberto, impedindo corrupção por escrita concorrente no SQLite da IA.

### 2026-09-13 [Task 02.1] Compartilhamento e Sincronização Automática de config.json e Permissões

- **Contexto:** Permissões do assistente de IA (comandos shell e ferramentas MCP aprovadas com "Always allow") residem em `~/.gemini/config/config.json`. Em perfis isolados, o usuário era obrigado a reaprovar individualmente cada comando read-only ou ferramenta MCP em cada nova janela de perfil.
- **Decisão:**
  - Compartilhar `~/.gemini/config/config.json` via link simbólico por padrão na criação e lançamento de perfis (`link_user_config` / `Link-UserConfig`).
  - Implementar comando `multigravity config <status|share|isolate> <profile>` para gerenciar o vínculo e permitir opt-out via `--isolated-config` / `.isolated_config`.
  - Preservar integridade dos perfis existentes criando backup `config.json.bak` antes da substituição por symlink.
  - Garantir paridade 100% entre Bash e PowerShell.

### 2026-09-13 [Task 02.2] Telemetria e Monitoramento de Cotas e Tokens (multigravity quota)

- **Contexto:** Usuários não tinham visibilidade de consumo de tokens, porcentagem restante de cota nem do tempo exato para o reset de limites das janelas móveis nos seus perfis.
- **Descoberta Técnica:** O Antigravity executa localmente o binário `language_server` (em Go) com uma porta HTTPS dinâmica e um token `--csrf_token <uuid>`. Ele expõe o serviço gRPC/HTTPS `/exa.language_server_pb.LanguageServerService/RetrieveUserQuotaSummary`. A autenticação exige o cabeçalho HTTP `X-Codeium-Csrf-Token: <csrf_token>`.
- **Estrutura de Cota:** A resposta divide os limites em buckets:
  - `gemini-5h`: Janela deslizante de 5 horas para suavização de pico global de tráfego. Retorna `remainingFraction` (float) e `resetTime` (timestamp ISO).
  - `gemini-weekly`: Janela semanal atrelada ao tier individual da conta Google.
  - `3p-5h` e `3p-weekly`: Janelas para modelos externos (Claude Opus/Sonnet, GPT).
- **Decisão:** Criar `multigravity quota [perfil]` e alias `multigravity ai quota [perfil]`, mapeando os processos ativos de cada perfil e formatando as barras de progresso e contagens regressivas em horas/minutos, com paridade 100% entre Bash e PowerShell.

### 2026-09-13 [Task 02.3] Automação de Reset de Cota Semanal (`multigravity prime` & Watchdog)

- **Contexto:** A janela de cota semanal (`gemini-weekly`) só reinicia a contagem regressiva de 7 dias após o usuário enviar a primeira mensagem do novo ciclo. Se o usuário ficar 2 dias sem usar a ferramenta após o reset, ele perde 2 dias úteis de recarga futura.
- **Descoberta Técnica de RPCs do Language Server:**
  - O endpoint gRPC/HTTPS `/exa.language_server_pb.LanguageServerService/StartCascade` com payload `{"source": "CORTEX_TRAJECTORY_SOURCE_CLI"}` cria uma nova trajetória e retorna um `cascadeId`.
  - O endpoint `/exa.language_server_pb.LanguageServerService/SendUserCascadeMessage` permite o despacho direto de prompts. Para minimizar o impacto na cota (< 5 tokens), utiliza-se o modelo `gemini-3.6-flash-low`, mapeado internamente no enum como `MODEL_PLACEHOLDER_M73`:
    `{"cascadeId": "<id>", "items": [{"text": "ping"}], "cascadeConfig": {"plannerConfig": {"requestedModel": {"model": "MODEL_PLACEHOLDER_M73"}}}}`.
  - **Execução Headless:** Se a IDE estiver fechada, o binário `language_server` pode ser invocado diretamente em modo efêmero com os argumentos `--standalone --headless=true --gemini_dir <profile_dir>/.gemini --csrf_token <token>`, permitindo consultar a cota e disparar o priming sem sequer abrir a interface gráfica.
- **Anti-bot e Idempotência:**
  - Pool de prompts naturais e variados em inglês e português ("ping", "Hello! Quick status check.", "Olá! Tudo bem por aí?", "Oi! Teste rápido de status.", etc.), selecionados aleatoriamente a cada disparo para evitar repetições previsíveis de mensagens.
  - Implementado jitter randômico de 0 a 60 minutos (`--jitter <mins>`, padrão 60m, bypass com `--no-jitter`) para evitar requisições em intervalos fixos previsíveis.
  - Estado persistido em `~/.local/share/multigravity/prime_state.json` vinculando `last_primed_cycle_reset`, `last_prompt` e `last_primed_at` para assegurar que cada ciclo semanal só seja inicializado exatamente uma vez.
- **Automação Contínua:**
  - Suporte a instalação e desinstalação simplificada de rotinas em segundo plano:
    - Linux/macOS: Cron (`--install-cron` / `--uninstall-cron`) rodando a cada 30 minutos e Systemd User Timer (`--install-systemd` / `--uninstall-systemd`).
    - Windows: Tarefas agendadas nativas via `schtasks.exe` (`--install-task` / `--uninstall-task`).
  - **Atenção ao `$HOME`:** No Linux/macOS dentro do terminal integrado do Antigravity, a variável `$HOME` é redirecionada para a raiz do perfil (`~/AntigravityProfiles/<name>`). O watchdog e os instaladores de cron/systemd utilizam estritamente `${REAL_HOME:-$HOME}` para referenciar o diretório do usuário host.
  - **Isolamento de Credenciais em Modo Headless:** O binário `language_server` localiza `jetski-standalone-oauth-token` e os segredos do Keyring sempre relativos à variável de ambiente `HOME` (Linux/macOS) ou `USERPROFILE` (Windows), e não apenas pelo argumento `--gemini_dir`. Portanto, ao subir instâncias headless de múltiplos perfis, é mandatório exportar `HOME="$p_dir"` / `$env:USERPROFILE = $pDir` para garantir que cada perfil leia seu respectivo token e não misture cotas com a instalação global ou com outros perfis.

### 2026-09-13 [Task 02.4] Prime Dual-Bucket (Gemini + Claude/GPT), Catálogo prompts.json e Jitters Independentes

- **Contexto:** Os limites de cota para modelos de terceiros (Claude e GPT) operam sob um bucket independente (`3p-weekly`) com ciclo de 7 dias próprio e desvinculado do `gemini-weekly`. O usuário necessitava que o priming atuasse de forma independente em ambos os buckets, usando modelos leves específicos, mensagens distintas e janelas de jitter separadas.
- **Descoberta de Enums de Modelos:**
  - O bucket `3p-weekly` alimenta modelos como Claude Sonnet, Claude Opus e GPT-OSS.
  - O enum interno utilizado pela IDE Antigravity para despachar ao Claude Sonnet (o modelo leve do bucket 3P) é `MODEL_PLACEHOLDER_M35` (`claude-sonnet-4-6`).
  - O bucket `gemini-weekly` utiliza `MODEL_PLACEHOLDER_M73` (`gemini-3.6-flash-low`).
- **Arquitetura de Catálogo de Mensagens (`prompts.json`):**
  - Externalização das mensagens de priming para `~/.local/share/multigravity/prompts.json` (auto-criado caso inexistente, com 40 prompts naturais e casuais em inglês e português).
  - Cada disparo sorteia prompts distintos sem repetição imediata para cada bucket e ciclo.
- **Descoberta de Portas Headless no Linux:**
  - O utilitário `ss -tulpn` em ambientes Linux sem privilégios de root oculta a coluna de PID/Processo para sockets em certos kernels (ex: Debian/Ubuntu).
  - O binário `language_server` emite no `stderr` a linha canônica `Language server listening on random port at <PORT> for HTTPS (gRPC)`.
  - A captura direta não-bloqueante do `stderr` com fallback para `ss -tulpn` garante descoberta determinística da porta HTTPS em < 0.5s sem depender de privilégios elevados.
- **Estrutura de Estado e Migração:**
  - `prime_state.json` passou a estruturar o estado por bucket: `{"<perfil>": {"gemini-weekly": {...}, "3p-weekly": {...}}}`.
  - Implementada migração transparente e retrocompatível de arquivos de estado anteriores de bucket único.
- **Paridade de Plataformas:**
  - Implementação idêntica e simultânea no script Bash (`multigravity`) e PowerShell (`multigravity.ps1`).

### 2026-09-13 [Task 02.5] Sincronização de Credenciais (gh e git-credentials) e Preservação de PATH de Usuário

- **Contexto:** Perfis isolados forçam `HOME="$profile_path"`, o que deixava o GitHub CLI deslogado (`gh auth status` falhava por ausência de `~/.config/gh`), ignorava credenciais HTTPS salvas em `~/.git-credentials`, e omitia diretórios de binários instalados no usuário (`~/.local/bin`, `~/.cargo/bin`, etc.) no terminal integrado.
- **Decisão:**
  - **Credenciais do GitHub CLI:** Compartilhar por padrão `$REAL_HOME/.config/gh` (Linux/macOS) e `%APPDATA%\GitHub CLI` (Windows) via symlink/junction na criação e lançamento de perfis (`link_gh_config` / `Link-GhConfig`).
  - **Git HTTPS:** Sincronizar `$REAL_HOME/.git-credentials` em `link_dev_dotfiles` / `Link-DevDotfiles`.
  - **Opt-out Granular:** Criar comando `multigravity gh <status|share|isolate> <profile>` e flag `--isolated-gh` (sentinela `.isolated_gh`), além de respeitar `--isolated-dotfiles`.
  - **Paridade de Plataformas:** Implementado com 100% de paridade entre Bash (`multigravity`) e PowerShell (`multigravity.ps1`).

### 2026-09-13 [Task 02.6] Suporte a Auto-Priming de Janela de 5 Horas (gemini-5h e 3p-5h) e Checagem Pré-Prime

- **Contexto:** Além das cotas semanais (`gemini-weekly`, `3p-weekly`), o Antigravity gerencia janelas móveis de pico de 5 horas (`gemini-5h`, `3p-5h`). O usuário solicitou que o priming também inicializasse os ciclos de 5 horas assim que refresheds, com verificação de segurança caso o usuário tenha interagido manualmente com a IDE antes do disparo.
- **Estrutura dos 4 Buckets:**
  - `gemini-weekly`: Cota semanal do Gemini.
  - `gemini-5h`: Limite de 5 horas do Gemini (`parent_key: gemini`).
  - `3p-weekly`: Cota semanal de Claude & GPT.
  - `3p-5h`: Limite de 5 horas de Claude & GPT (`parent_key: 3p`).
- **Otimização de Disparo e Vínculo Automático:**
  - Enviar um prompt para um modelo consome simultaneamente tokens do bucket semanal e do bucket de 5h do mesmo provedor.
  - Se um provedor já foi primed no mesmo ciclo de execução (ex: semanal acabou de rodar), o script vincula automaticamente o ciclo de 5h (`(Linked with parent prime)`) sem enviar uma mensagem redundante, economizando tokens e evitando poluição do histórico de chat.
- **Proteção contra Exaustão Semanal:**
  - O prime de 5 horas só executa se a respectiva cota semanal tiver saldo (> 5%). Se a cota semanal estiver zerada, o bucket de 5 horas é omitido ou ignorado com aviso.
- **Checagem de Última Milha Pré-Prime:**
  - Imediatamente antes de despachar `StartCascade`, o script efetua uma re-consulta de telemetria ao vivo. Se a cota caiu (`rem_frac < 0.999`) ou o `resetTime` foi alterado por atividade manual do usuário, o prime é cancelado imediatamente.
- **Watchdog e Flags:**
  - Adicionadas flags `--5h` e `--include-5h` na CLI e nos instaladores de cron/systemd/Task Scheduler (`--install-cron --5h`).
  - Formatação visual no `--status` detalhando os 4 limites organizados por provedor e período.

### 2026-09-14 [Task 01.3] Correção de Crash no Menu TUI por Ausência de `get_profile_color` (Issue #1)

- **Contexto:** Ao iniciar o `multigravity` de forma interativa sem argumentos, o menu TUI chamava `custom_color="$(get_profile_color "$name")"`. A função `get_profile_color` não estava definida, provocando encerramento imediato via `command not found` devido a `set -euo pipefail`.
- **Decisão:**
  - Criar `get_profile_color <profile>` no Bash (`multigravity`) para consultar com segurança `workbench.colorCustomizations.titleBar.activeBackground` em `User/settings.json`, usando Python 3 e fallback via `grep`/`sed`.
  - Refatorar `profile_color_cmd` para reutilizar `get_profile_color`.
  - Criar função equivalente `Get-ProfileColor` no PowerShell (`multigravity.ps1`) e reutilizá-la tanto em `Invoke-ColorProfile` quanto no menu interativo, mantendo paridade integral de plataformas e eliminando código duplicado.

### 2026-09-15 [Task 02.1] Criação de Skill Canônica do Multigravity para Agentes de IA

- **Contexto:** Assistentes de IA autônomos (Antigravity e Cursor) precisam consultar cotas de tokens (`quota`), gerenciar limites e ciclos de priming (`prime`), orquestrar isolamento de credenciais (`mcp`, `skills`, `config`, `gh`), e manipular brains de conversas (`ai sync`, `ai export`) sem violar os contratos de `HOME` ou destruir perfis ativos.
- **Decisão:**
  - Criar especificação canônica em `skills/multigravity/SKILL.md` seguindo rigorosamente o template canônico de `000-template.md` (`agent-skills`).
  - Desenvolver `scripts/install-agent-skills.sh` seguindo a arquitetura de distribuição de skills da organização (`infra-victoria-logs`), com suporte a `--dry-run`, `--list`, `--antigravity`, `--cursor` e `--target`.
  - Sincronizar e disponibilizar a skill globalmente em `~/.gemini/config/skills/multigravity` e registrar o catálogo central em `ye-sandbox/agent-skills`.

### 2026-09-23 [Task 02.2] Injeção de Comandos Read-Only Padrão em "Always Allow" nos Perfis (config.json)

- **Contexto:** Ao criar novos perfis ou utilizar perfis com configuração isolada (`.isolated_config`), o assistente de IA ficava sem permissões de execução persistidas, forçando o usuário a aprovar manualmente no diálogo do terminal dezenas de comandos rotineiros de leitura (Git, POSIX, inspeção e runners de linters/testes).
- **Decisão:**
  - Definir lista canônica de comandos estritamente read-only cobrindo Git (`status`, `log`, `diff`, `show`, `branch`, `tag`, `remote`, `rev-parse`, `describe`, `config --get/--list`), ferramentas POSIX (`ls`, `cat`, `head`, `tail`, `grep`, `rg`, `find`, `which`, `whereis`, `where`, `file`, `stat`, `wc`, `uname`, `pwd`, `echo`, `env`, `printenv`, `df`, `du`, `ps`, `uptime`, `date`, `whoami`, `hostname`, `tree`), gerenciadores/ferramentas dev (`npm test/run lint/check/typecheck/list/view/audit/outdated`, `pnpm test/run lint/check/typecheck/list/audit/outdated`, `uv run ruff/pytest/pyright/mypy/pip list/tree`), linters diretos (`ruff check/format --check`, `pytest`, `pyright`, `mypy`, `eslint`, `tsc --noEmit`, `prettier --check`) e equivalentes Windows (`dir`, `type`, `Get-ChildItem`, `Get-Content`, `Get-Process`, `Get-Item`, `Get-Location`).
  - Mapear cada comando para as duas modalidades de execução do Antigravity: `command(<cmd>)` (sandbox padrão) e `unsandboxed(<cmd>)` (bypass sandbox), assegurando autonomia total sem quebras de segurança.
  - Implementar funções aditivas e idempotentes: `seed_default_permissions` no Bash (`multigravity`) e `Seed-DefaultPermissions` no PowerShell (`multigravity.ps1`).
  - Integrar o seeding automaticamente em `link_user_config` / `Link-UserConfig` (tanto para perfis compartilhados via `$host_config` quanto para perfis isolados) e na ação `isolate`.
  - Adicionar comando CLI `multigravity config seed <perfil|--all|--host>` (alias `allow-readonly`) com autocomplete e documentação.

### 2026-09-24 [Task 90.2] Implementação dos Comandos de Gestão de Perfis em Go (new, delete, rename)

- **Contexto:** Continuidade da reescrita em Go (`feat/go-rewrite`). Necessidade de portar os comandos de ciclo de vida de diretórios de perfil (`new`, `delete`, `rename`) mantendo paridade com as invariantes de isolamento e concorrência.
- **Decisão:**
  - **Toolchain Hermética:** Instalado Go 1.27.1 em `~/.local/go` com symlinks em `~/.local/bin/go`.
  - **Módulo `internal/profile/manager.go`:**
    - `CreateProfile`: validação de nome regex `^[a-zA-Z0-9][a-zA-Z0-9-]*$`, criação das subpastas por SO (Linux: `.config/Antigravity`, `.cache`, `.local/share`, `.local/state`; macOS: `Library/Application Support`, link para Keychains), flags de isolamento sentinelas (`.isolated_dotfiles`, `.isolated_mcp`, `.isolated_skills`, `.isolated_config`, `.isolated_gh`) e flag `.shared`.
    - `DeleteProfile`: verificação ativa se o perfil está em execução via `IsProfileRunning(name)`, abortando a menos que `--force` seja fornecido (com envio de sinal `SIGKILL` para processos remanescentes), e remoção limpa do diretório.
    - `RenameProfile`: validação de nomes de origem e destino, garantia de não sobrescrita de perfis existentes e verificação ativa de processos em execução.
  - **Comandos Cobra:** criados `internal/cmd/new.go`, `internal/cmd/delete.go` (com prompt interativo `[y/N]` na ausência de `--force`) e `internal/cmd/rename.go`.
  - **Testes Unitários:** implementados em `internal/profile/manager_test.go` e `internal/cmd/cmd_test.go` utilizando `t.TempDir()` e `MULTIGRAVITY_HOME`, cobrindo 100% dos fluxos de criação, renomeação, deleção e validações de erro.
### 2026-09-24 [Task 90.3] Implementação de Ciclo de Vida (`stop`, `restart`) e Limpeza de Caches (`clean`) em Go

- **Contexto:** Portar os comandos de controle de processos (`stop`, `restart`) e limpeza segura de caches (`clean`) para a CLI em Go (`feat/go-rewrite`).
- **Decisões:**
  - **Process Detection & Termination (`internal/profile/process_unix.go` e `process_windows.go`):** Separação por tags de build. No Unix, usa `ps -eo pid,ppid,args` inspecionando `dataDir` (`GetUserDataDir`) e `profileDir`, ignorando o próprio PID e PPID para evitar matar o próprio terminal/processo pai. No Windows, consulta `Win32_Process` via PowerShell/WMI e utiliza `taskkill`.
  - **Graceful Stop (`internal/profile/lifecycle.go`):** Envio de `SIGTERM` e espera ativa por até 3 segundos (15 iterações de 200ms) para flush de dados/SQLite, com fallback forçado via `SIGKILL` em caso de timeout. Flag `--force` (`-f`) pula a espera e envia `SIGKILL` imediatamente. Mensagens com 100% de paridade com o legado.
  - **Restart (`internal/profile/lifecycle.go`, `internal/cmd/restart.go`):** `restart <name> [args...]` efetua stop gracioso, aguarda 500ms e despacha relançamento via `LaunchProfile`. Utilizado `Flags().SetInterspersed(false)` no Cobra para preservar passthrough de flags arbitrárias destinadas ao Antigravity.
  - **Limpeza Cirúrgica (`internal/profile/clean.go`, `internal/cmd/clean.go`):** `CleanProfile` e `CleanSingleProfile` removem apenas caches voláteis (Chromium, Electron, GPUCache, Crashpad, Service Worker, crashes, npm cache), preservando intactos arquivos de preferências (`User/settings.json`), extensões e credenciais. Recria `.cache` vazio (e `AppData/Local/Temp` no Windows). Trava ativa impede limpar perfil em execução, e `--all` pula perfis abertos com warning.
  - **Cálculo de Tamanho de Diretório:** `GetDirSizeStr` utiliza `du -sh` no Unix com fallback para caminhamento puro em Go (`filepath.Walk`), garantindo independência de ferramentas externas no Windows.

### 2026-09-24 [Task 90.4] Implementação de Lançamento de Perfis, Detecção de Executável e Atalhos de Desktop em Go

- **Contexto:** Portar detecção do executável (`antigravity`/`agy`), inicialização e lançamento de perfis com passthrough de argumentos e geração/remoção de atalhos de desktop por plataforma (`feat/go-rewrite`).
- **Decisões:**
  - **Detecção de Executável (`internal/app/detector.go`):** Respeito prioritário a `MULTIGRAVITY_APP` e `AGY_APP`. Busca exaustiva em `$PATH` e diretórios canônicos por SO (Linux: `/opt`, `/usr/bin`, `~/.local/bin`, `~/apps`; macOS: `/Applications`, `~/Applications`; Windows: `%LOCALAPPDATA%`, `%PROGRAMFILES%`, Scoop). Implementado `FindLanguageServer` com localização de binário interno.
  - **Atalhos de Desktop (`internal/shortcut/`):**
    - Linux: cria script wrapper POSIX executável em `~/.local/share/multigravity/launchers/<name>.sh` e arquivo `.desktop` completo em `~/.local/share/applications/multigravity-<name>.desktop`.
    - macOS: bundle `Multigravity <name>.app` com `Contents/MacOS/run` e `Info.plist`.
    - Windows: cria atalho `.lnk` no Start Menu com PowerShell COM `WScript.Shell`.
    - Ciclo de vida: integrado a `CreateProfile` (criação), `DeleteProfile` (remoção) e `RenameProfile` (remoção do antigo e criação do novo). Suporte a isolamento hermético em testes via `MULTIGRAVITY_TEST_SHORTCUTS_DIR`.
  - **Lançamento e Isolamento de Ambiente (`internal/profile/launcher.go` e `layout.go`):**
    - Layout e links automáticos antes do lançamento (`EnsureProfileLayout`).
    - Preservação de ferramentas dev globais enriquecendo `PATH` com `~/.local/bin`, `~/.cargo/bin`, `~/.bun/bin`, `~/go/bin` sem quebrar o isolamento de `HOME`.
    - Injeção das flags Electron `--user-data-dir` e `--extensions-dir` com repasse de flags e caminhos do usuário.
    - Detecção de `--wait` / `-w` para aguardar encerramento quando utilizado em modo editor/git.
  - **Cobra CLI Routing (`internal/cmd/root.go`):**
    - Configurado `rootCmd.FParseErrWhitelist.UnknownFlags = true` e `rootCmd.Flags().SetInterspersed(false)`, permitindo invocar diretamente `multigravity <profile> [args...]` sem conflito com flags do VS Code / Antigravity.

### 2026-09-24 [Task 90.5] Implementação de Theming Visual (color) e Compartilhamento Modular (config, mcp, skills, gh) em Go

- **Contexto:** Portar theming visual e controle granular de compartilhamento/isolamento de MCP, skills, permissões read-only e GitHub CLI para a CLI em Go (`feat/go-rewrite`).
- **Decisões:**
  - **Theming Visual (`internal/profile/color.go`, `internal/cmd/color.go`):**
    - Resolução da paleta canônica (15 cores) e hex `#RRGGBB`.
    - Modificação não-destrutiva de `workbench.colorCustomizations` em `User/settings.json`, com desacoplamento seguro de symlinks em perfis compartilhados.
    - Suporte a `--clear`, consulta via `color <profile>` e integração transparente com flag `--color` em `CreateProfile`.
    - Adicionado campo `Color` em `ProfileInfo` recuperado deterministicamente via `GetProfileColor`.
  - **Injeção de Permissões Read-Only (`internal/profile/permissions.go`):**
    - Implementação pura em Go para injeção aditiva dos 58 comandos canônicos em `command(...)` e `unsandboxed(...)` (116 grants), eliminando a dependência do interpretador `python3` externo em tempo de execução.
    - Integrado automaticamente em `LinkUserConfig`, `ConfigIsolate` e `ConfigSeed`.
  - **Compartilhamento e Isolamento Modular (`internal/profile/sharing.go`, `internal/cmd/`):**
    - Módulos para `mcp`, `skills`, `config` e `gh` com subcomandos `status`, `share` e `isolate`.
    - Respeito integral a sentinelas `.isolated_mcp`, `.isolated_skills`, `.isolated_config`, `.isolated_gh` e `.isolated_dotfiles`.
    - Criação de backup `.bak` na transição de arquivos/diretórios locais standalone para symlinks compartilhados.
    - Suporte a aliases e flags em `config seed` (`--host`, `--all`) e comando direto `allow-readonly`.

### 2026-09-24 [Task 90.6] Implementação de Backup, Restauração e Templates (clone, export, import, template, stats) em Go

- **Contexto:** Portar operações de cópia de perfis, gerenciamento de templates, compactação/descompactação de backups e telemetria de uso de disco para a CLI em Go (`feat/go-rewrite`).
- **Decisões:**
  - **Cópia Preservando Symlinks (`internal/profile/copy.go`):** `CopyDir` e `CopyFile` implementados com leitura estrita de links via `os.Lstat` e recriação com `os.Symlink`. Evita desreferenciação acidental que duplicaria dotfiles globais do host (`.gitconfig`, `.ssh`) ou quebraria o isolamento e sincronização de `config.json`, `mcp_config.json` e skills.
  - **Clonagem e Templates (`internal/profile/clone.go`, `internal/profile/template.go`, `internal/cmd/clone.go`, `internal/cmd/template.go`):**
    - `clone <src> <dest>`: validação estrita de nomes, cópia com `CopyDir` e criação automática de atalhos de desktop por plataforma.
    - `template <save|list|delete>`: gerencia templates salvos em `$BASE/.templates`. Ao salvar um perfil como template, remove o sentinela `.shared` para garantir que novas instâncias geradas sejam limpas por padrão.
    - Integração com `new`: suporte a `--from <tpl>` e alias `--template <tpl>` em `internal/cmd/new.go` e `internal/profile/manager.go`, copiando o template e reaplicando `EnsureProfileLayout` sobre regras de isolamento.
  - **Exportação e Importação Herméticas (`internal/profile/archive.go`, `internal/cmd/export.go`, `internal/cmd/import.go`):**
    - `export <name> [path] [--include-cache]`: suporte a `.tar.gz` (padrão Unix) e `.zip` (padrão Windows ou por extensão explícita). Quando `!includeCache`, exclui cirurgicamente caches voláteis (Chromium, Electron, GPUCache, logs, Crashpad, Service Worker, npm cacache, Library/Caches, caches de IA) reduzindo drasticamente o tamanho do arquivo.
    - `import <archive> [name]`: descompactação em Go puro para `.tar.gz` e `.zip` com proteção ativa contra ataques de path traversal (*Zip Slip*). Ajusta a raiz do arquivo descompactado, move deterministicamente para o destino e gera atalhos de sistema.
  - **Estatísticas de Armazenamento (`internal/profile/stats.go`, `internal/cmd/stats.go`):**
    - `stats`: renderiza tabela de uso de disco (`PROFILE`, `SIZE`, `EXTENSIONS`) com contagem de extensões em `.antigravity/extensions` e cálculo de uso total de `$BASE` com paridade 100% com o legado.

### 2026-09-24 [Task 90.7] Implementação de Telemetria de Cotas (quota), Priming (prime) e Gestão de Chats (ai) em Go

- **Contexto:** Portar telemetria de cotas de IA (`quota`), automação de priming via Language Server (`prime`) e comandos de gerenciamento de histórico e chats de IA (`ai export`, `ai import`, `ai sync`, `ai list`) para a CLI em Go (`feat/go-rewrite`), eliminando qualquer dependência do runtime `python3` externo.
- **Decisões:**
  - **Cliente HTTPS e RPCs do Language Server (`internal/quota/`):**
    - Cliente HTTP TLS puro com `InsecureSkipVerify: true` para comunicação local com o serviço gRPC/HTTPS `/exa.language_server_pb.LanguageServerService/` via JSON (`RetrieveUserQuotaSummary`, `StartCascade`, `SendUserCascadeMessage`).
    - Geração de UUID v4 puro em Go (`crypto/rand`) para o token CSRF (`X-Codeium-Csrf-Token`).
    - Descoberta dinâmica de servidores ativos (`FindActiveServers`) via `ps`/sockets (`lsof`, `ss`) e processo pai para identificação automática do perfil dono.
    - Execução efêmera headless (`StartHeadlessServer`) com parsing do `stderr` em `< 0.5s` e preservação estrita do isolamento de credenciais via `$HOME`/`%USERPROFILE%`.
    - Renderização visual de barras de progresso ASCII e contagem regressiva em horas/minutos até o reset com paridade integral.
  - **Automação de Priming Multi-Bucket (`internal/prime/`):**
    - Suporte aos 4 buckets: `gemini` (semanal), `gemini_5h`, `3p` (semanal Claude/GPT), `3p_5h`.
    - Catálogo auto-semeado `prompts.json` com 40 mensagens variadas em PT/EN e seleção aleatória sem repetição imediata.
    - Persistência e migração transparente em `prime_state.json` com conversão automática de arquivos de estado legados de bucket único.
    - Proteção ativa: trava que impede disparo no bucket de 5h se a cota semanal pai estiver esgotada ($\le 5\%$).
    - Otimização de disparo: auto-link do ciclo de 5h sem segundo envio de mensagem quando o bucket pai semanal já foi disparado na mesma sessão.
    - Checagem pré-prime de última milha: aborta disparo se detectar atividade manual do usuário nos segundos prévios ao despacho.
    - Gestão de automações em background: suporte a crontab e systemd user timers no Linux/macOS, e Scheduled Tasks via `schtasks.exe` no Windows.
  - **Gerenciamento de Histórico e Conversas (`internal/chat/`):**
    - `ai list`: consulta `.db` SQLite, títulos em `annotations/*.pbtxt` e arquivos markdown em `brain/`.
    - `ai export` e `ai import`: empacotamento com sanitização cirúrgica de qualquer token ou credencial (`*token*`, `*oauth*`, `*auth*`, `*credential*`, `installation_id`).
    - `ai sync`: sincronização não-destrutiva entre perfis com trava ativa de concorrência.

### 2026-09-24 [Task 90.8] Portar Menu Interativo TUI sem argumentos, Diagnóstico do Sistema (doctor) e Shell Completion em Go

- **Contexto:** Finalizar a cobertura funcional dos utilitários de conveniência da CLI: menu interativo TUI ao executar `multigravity` sem parâmetros, diagnóstico completo do ambiente (`doctor`) e autocompletion para shells (`completion`) na branch `feat/go-rewrite`.
- **Decisões:**
  - **Menu Interativo TUI (`internal/tui/menu.go`):**
    - Detecção estrita de terminal interativo com `isatty.IsTerminal` e `isatty.IsCygwinTerminal` para `os.Stdin` e `os.Stdout`.
    - Preservação do contrato legado: ambientes não-interativos (redirecionamentos, pipes, automações, scripts) recebem a saída de `cmd.Help()` e código de saída 1.
    - Em terminais interativos, renderiza o menu estilizado em ANSI com cabeçalho canônico `MULTIGRAVITY PROFILES`, listagem com número, status (`● running` em verde / `○ idle`), tipo (`shared`/`isolated`) e cor configurada (`[color: ...]`).
    - Suporte a seleção por número de 1 a N, seleção direta pelo nome do perfil, criação de novo perfil (`n`) e saída suave (`q` / enter vazio).
  - **Diagnóstico de Ambiente (`doctor`, `internal/doctor/doctor.go`, `internal/cmd/doctor.go`):**
    - Bateria de diagnósticos: plataforma SO, executável Antigravity/Agy (`app.FindApp()`), binário global no `$PATH`, integridade de `icon.icns` no macOS e teste ativo de escrita em `$MULTIGRAVITY_HOME` com `.write-test`.
    - Sumário visual com contagem de avisos/erros e mensagens canônicas.
  - **Shell Completion (`completion`, `internal/cmd/completion.go`):**
    - Geração de autocompletion nativo via Cobra CLI para `bash`, `zsh`, `fish` e `powershell`.
    - Guia inteligente sem argumentos que detecta a shell atual (`$SHELL` no Unix ou `$PROFILE` no Windows) e instrui a adição correta ao arquivo de inicialização.
    - Implementado `ValidArgsFunction` em `rootCmd` e em todos os comandos aplicáveis (`color`, `stop`, `restart`, `clean`, `delete`, `rename`, `clone`, `export`, `quota`, `prime`, `mcp`, `skills`, `config`, `gh`) para autocompletion dinâmico e contextual dos perfis existentes.

### 2026-09-24 [Task 90.9] Validação de Paridade com Scripts Legados, Instalação e Troca do Ponto de Entrada Padrão

- **Contexto:** Conclusão da reescrita em Go na branch `feat/go-rewrite`. Validação de paridade integral de comandos, flags e aliases com os scripts legados Bash e PowerShell, preservação do legado em `legacy/`, troca do ponto de entrada padrão da raiz do repositório e modernização dos instaladores.
- **Decisões:**
  - **Paridade de Aliases e Comandos:**
    - Adicionados aliases canônicos nos comandos Cobra: `create` para `new`, `rm` para `delete`, `mv` para `rename`, `cp` para `clone` e `ls` para `list`.
    - Implementado `update` (`internal/cmd/update.go`) com verificação e download atômico da release correta por SO e arquitetura (`multigravity-{GOOS}-{GOARCH}`).
  - **Preservação dos Scripts Legados (`legacy/`):**
    - Scripts Bash e PowerShell legados movidos para `legacy/multigravity` e `legacy/multigravity.ps1` via `git mv`, preservando histórico Git e permitindo fallback seguro.
  - **Launchers Inteligentes como Ponto de Entrada na Raiz (`multigravity`, `multigravity.ps1`):**
    - Script raiz `./multigravity` (POSIX/Bash):
      1. Se `bin/multigravity` existir, executa imediatamente via `exec`.
      2. Se `go` estiver instalado, compila automaticamente para `bin/multigravity` e executa.
      3. Se `go` não estiver instalado nem houver binário, delega transparentemente para `legacy/multigravity`.
    - Script raiz `./multigravity.ps1` (PowerShell/Windows):
      1. Executa `bin\multigravity.exe` se presente.
      2. Se `go` estiver disponível, compila ou roda `go run`.
      3. Se não, delega transparentemente para `legacy\multigravity.ps1`.
  - **Instaladores e Desinstaladores Atualizados (`install.sh`, `install.ps1`, `uninstall.ps1`):**
    - Detecção automática de arquitetura (`amd64`, `arm64`) e SO (`linux`, `darwin`, `windows`).
    - Prioridade 1: compilação local se executado dentro do repositório clonado com `go`.
    - Prioridade 2: download do binário pré-compilado via GitHub Releases (`multigravity-$PLATFORM-$ARCH`).
    - Prioridade 3: fallback para script standalone se release não contiver o asset.
    - `uninstall.ps1` atualizado para remover `multigravity.exe` além dos scripts e wrappers.
    - Adicionado target `install` no `Makefile`.

### 2026-09-24 [Task 00.1] Diagnóstico e Auditoria de Integridade Pós-Release v2.0.0

- **Contexto:** Auditoria de integridade pós-lançamento da v2.0.0 na branch principal `main`.
- **Validação Executada:**
  - `go test -v ./...`: 100% dos testes unitários passando em todos os pacotes (`internal/app`, `internal/chat`, `internal/cmd`, `internal/config`, `internal/doctor`, `internal/prime`, `internal/profile`, `internal/quota`, `internal/shortcut`, `internal/tui`).
  - `bash -n multigravity install.sh uninstall.sh legacy/multigravity`: sintaxe validada sem erros ou warnings.
  - Compilação do binário Go em `bin/multigravity` bem-sucedida.
  - `./multigravity doctor` executado com sucesso validando runtime, executável do Antigravity, binário global e permissões de escrita em `$MULTIGRAVITY_HOME`.

### 2026-09-24 [Task 02.1] Padronização e Suporte a Contratos Machine-Readable (--json) nos Comandos de Consulta

- **Contexto:** Necessidade de permitir consumo automatizado de telemetria e estado de perfis por agentes de IA, VictoriaLogs e futuras UIs/APIs (Diretrizes #6 e #7 do `AGENTS.md`).
- **Decisões:**
  - **`list`:** Adicionada flag `--json` serializando `[]profile.ProfileInfo` com tags JSON completas (`name`, `path`, `is_running`, `pids`, `type`, `last_used`, `size`, `color`).
  - **`stats`:** Adicionada struct `profile.ProfileStatsReport` (`profiles` e `total_size`) e flag `--json` no comando `stats`.
  - **`doctor`:** Desacoplamento da lógica de diagnóstico em função pura `doctor.Diagnose() (*DiagnosticReport, error)` e structs `DiagnosticCheck` e `DiagnosticReport`, mantendo `doctor.RunDoctor(w)` como camada de apresentação e adicionando `--json`.
  - **`quota`:** Adicionada flag `--json` serializando as instâncias ativas do Language Server (`[]quota.ActiveServer`) com os respectivos buckets de limite, consumo e reset time.
  - **`ai list`:** Desacoplamento de `chat.GetConversations(profile) ([]ConversationInfo, error)` e suporte à flag `--json` retornando metadados de chats e artefatos.
  - **`mcp status`:** Criadas structs `profile.SharingStatus` e `profile.GetMcpStatus(profile)`, adicionando `--json` ao subcomando `mcp status <profile>`.
  - **Paridade e Retrocompatibilidade:** Saída padrão em texto/tabelas/ANSI 100% preservada na ausência da flag `--json`.

### 2026-09-24 [Task 03.1] Expansão da skill canônica do Multigravity e endpoints RPC/HTTP locais para Agregador

- **Contexto:** Necessidade de permitir que agregadores locais, dashboards, agentes e futuras UIs (Tauri/Wails/Svelte) consumam endpoints REST de forma contínua e segura, além de expandir a skill canônica do Multigravity com a documentação dos contratos de machine-readability e da API.
- **Decisões:**
  - **Servidor HTTP Nativo (`internal/server`):**
    - Construído usando apenas a biblioteca padrão `net/http` do Go 1.23+ com suporte a rotas com path parameters (`GET /api/v1/profiles/{name}`).
    - Middleware de CORS configurável permitindo integração direta com aplicações web e dashboards locais (`localhost:*`, `127.0.0.1:*`).
    - Bind seguro em loopback (`127.0.0.1:8989`) por padrão, evitando exposição não autorizada em interfaces de rede pública.
    - Endpoints implementados:
      - `GET /health` e `GET /api/v1/health`: uptime, versão e status.
      - `GET /api/v1/doctor`: relatório estruturado de diagnóstico.
      - `GET /api/v1/profiles`: listagem completa de perfis e estado running/idle.
      - `GET /api/v1/profiles/{name}`: metadados de perfil específico.
      - `GET /api/v1/profiles/{name}/stats`: estatísticas de uso em disco do perfil.
      - `GET /api/v1/stats`: agregação global de armazenamento.
      - `GET /api/v1/profiles/{name}/sharing`: status de compartilhamento de MCP, skills, config e GitHub CLI.
      - `GET /api/v1/profiles/{name}/conversations`: inventário de conversas de IA.
      - `GET /api/v1/quota`: métricas ao vivo de telemetria e cotas ativas.
      - `POST /api/v1/profiles/{name}/stop`: parada graciosa via HTTP.
      - `POST /api/v1/profiles/{name}/clean`: limpeza de caches voláteis via HTTP.
    - Suporte a graceful shutdown via captura de `SIGINT`/`SIGTERM`.
  - **Comando CLI `serve` (`internal/cmd/serve.go`):**
    - Flags `--host` (default `127.0.0.1`) e `--port` / `-p` (default `8989`, com fallback para `MULTIGRAVITY_PORT`).
  - **Expansão da Skill Canônica (`skills/multigravity/SKILL.md`):**
    - Adicionado Step 6 detalhando comandos `--json`, tabela de rotas da API HTTP local e exemplos de consumo com `curl` e `jq`.
    - Sincronização executada com sucesso via `scripts/install-agent-skills.sh --antigravity`.

### 2026-09-24 [Task 04.1] Suporte a Streaming em Tempo Real via Server-Sent Events (SSE) no Servidor HTTP

- **Contexto:** Aplicações clientes, UIs reativas (Tauri/Svelte/Wails) e agregadores precisavam consultar continuamente endpoints HTTP (`polling`) para detectar mudanças de estado nos perfis (running vs idle) e ações executadas, gerando sobrecarga de CPU e requisições repetitivas.
- **Decisões:**
  - **Broker de Eventos SSE (`internal/server/broker.go`):**
    - Gerenciador thread-safe (`sync.RWMutex`) de canais de clientes (`chan SSEEvent`).
    - Despacho não-bloqueante (`select { case ch <- ev: default: }`) evitando bloqueio head-of-line quando clientes consomem eventos lentamente.
    - Loop em background gerenciando verificação periódica de alterações de perfis (`CheckProfilesChange`) com comparação de hash/estado, emitindo evento `profiles` apenas quando houver mudanças efetivas.
    - Emissão de heartbeat periódico (`ping`) a cada 15 segundos para manter a conexão aberta e detectar conexões órfãs.
  - **Endpoints de Streaming (`internal/server/routes.go`):**
    - `GET /api/v1/events` e `GET /events`: configurados com `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive` e flush contínuo via `http.Flusher`.
    - Envio de snapshot inicial imediato (`event: init`) contendo a lista atual de perfis e versão do servidor.
    - Notificação instantânea de ações (`event: action`): endpoints `/stop` e `/clean` emitem evento push para todos os clientes conectados assim que a operação é concluída.
    - Desconexão e limpeza automática de clientes monitorando `r.Context().Done()`.
  - **Documentação e Testes:**
    - Documentado na skill canônica (`skills/multigravity/SKILL.md`) e no `README.md`.
    - Testes unitários herméticos em `internal/server/server_test.go` cobrindo handshake SSE, evento de inicialização, eventos customizados, notificação de ações e encerramento de conexões.

### 2026-09-24 [Task 05.1] Robustez na Detecção de Executável (FindApp), Resolução de REAL_HOME no Instalador e Validação no Host

- **Contexto:** Instalação e validação operacional da CLI Go compilada no ambiente real do usuário, preservando perfis preexistentes (`joaoww`, `luisfmb`, `yegear`).
- **Descobertas e Decisões Técnicas:**
  - **Detecção de Executável (`internal/app/detector.go`):** Quando `MULTIGRAVITY_APP` ou `AGY_APP` é fornecida como sobreposição explícita, se o arquivo apontado não existir ou não for executável, a função agora retorna erro imediatamente em vez de efetuar fallthrough silencioso para os diretórios padrão do sistema (`/opt/antigravity`, `/usr/bin`, etc.). Isso previne o lançamento acidental de binários não pretendidos e assegura que os testes unitários (`TestRequireAppNotFound` e `doctor`) passem em hosts com Antigravity instalado globalmente.
  - **Instalação Hermética (`install.sh`, `uninstall.sh`):** Suporte estrito a `REAL_HOME="${REAL_HOME:-$HOME}"` para garantir que instalações disparadas de dentro do terminal integrado do Antigravity (onde `$HOME` aponta para a pasta do perfil) instalem no diretório real do usuário (`/home/luis/.local/bin`), e compilação intermediária em `bin/multigravity` antes de copiar com `cp -f`, evitando erros de `go build` ao sobrescrever scripts texto legados existentes.
  - **Validação no Ambiente Real:** Executada bateria completa de comandos no host real (`doctor`, `list`, `stats`, `status`, `mcp status`, `skills status`, `config status`, `gh status`, `ai list`, `quota`). Confirmada detecção precisa do perfil ativo `yegear` (processos ativos, status running, cota via gRPC/HTTPS) e perfis idle `joaoww` e `luisfmb` com integridade 100% preservada.

### 2026-09-24 [Task 05.2] Detecção de Estado Arquivado/Ativo e Extração de Tópicos em 'ai list'

- **Contexto:** Necessidade de identificar e filtrar conversas de IA ativas vs arquivadas, além de extrair o tópico/prompt inicial dos chats que não possuem título explícito definido na anotação `.pbtxt`.
- **Descobertas e Decisões Técnicas:**
  - **Mapeamento de Arquivamento:** O Antigravity grava o estado de arquivamento em `~/.gemini/antigravity/annotations/<uuid>.pbtxt` através do campo `archived:true` e timestamp `archival_status_timestamp:{seconds:... nanos:...}`. Quando o chat está ativo, a chave `archived:true` inexiste e apenas `last_user_view_time` é registrado.
  - **Extração de Tópico do Transcript:** Para conversas sem título explícito (padrão `(untitled conversation)`), o parser agora inspeciona a primeira linha do log de execução em `~/.gemini/antigravity/brain/<uuid>/.system_generated/logs/transcript.jsonl`, extraindo e sanitizando o conteúdo do primeiro `USER_INPUT` (removendo tags `<USER_REQUEST>`, `@[...]`, quebras de linha e truncando em 60 caracteres).
  - **Ordenação Inteligente:** Conversas ativas aparecem priorizadas no topo da listagem, ordenadas de forma decrescente pelo timestamp da última interação (`last_view_at` / `archived_at`).
  - **Filtros e Contrato JSON/HTTP:**
    - CLI `ai list <profile>`: suporte a `--active`, `--archived` e `--all` (com saída padrão exibindo colunas `CONVERSATION ID`, `STATUS`, `LAST ACTIVITY`, `TITLE / TOPIC`, `ARTIFACTS`).
    - Flags `--json`: serialização dos novos campos `archived` (bool), `archived_at` (ISO timestamp) e `last_view_at` (ISO timestamp).
    - API REST HTTP: `GET /api/v1/profiles/{name}/conversations?filter=active|archived|all` suportando os mesmos filtros e formato.

### 2026-09-24 [Task 05.3] Detecção e Exibição de Tamanho de Conversas (bytes/human-readable) em 'ai list'

- **Contexto:** Necessidade de mensurar o impacto real em disco de cada chat individualmente e agregar o total por perfil/filtro.
- **Descobertas e Decisões Técnicas:**
  - **Composição de Tamanho do Chat:** O cálculo do espaço consumido por uma conversa é a soma exata de 3 partes:
    1. Arquivo de banco SQLite: `~/.gemini/antigravity/conversations/<uuid>.db`.
    2. Arquivo de anotações e metadados: `~/.gemini/antigravity/annotations/<uuid>.pbtxt`.
    3. Diretório cerebral do agente: caminhamento recursivo em `~/.gemini/antigravity/brain/<uuid>/` somando todos os artefatos markdown e logs de transcrição (`transcript.jsonl`, `transcript_full.jsonl`).
  - **Estrutura de Dados:** Adicionados os campos `size` (string human-readable como `5.9M`, `596.7K`, `201.7K`) e `size_bytes` (int64) na struct `chat.ConversationInfo`.
  - **Apresentação em Terminal:** Inserida a coluna `SIZE` entre `STATUS` e `LAST ACTIVITY` em `multigravity ai list`, e adicionado o somatório `Total size: X.YM` na linha de sumário final.
### 2026-09-24 [Task 06.1] Detecção de Chats de IA em Uso/Abertos no SO e Suporte a Sync Granular Seguro

- **Contexto:** `ai sync` e `ai export` abortavam preventivamente quando o perfil estava em execução (`IsProfileRunning`), impedindo o backup ou sincronização das demais conversas inativas e consistentes do perfil.
- **Descobertas e Decisões Técnicas:**
  - **Detecção de Handles no SO (`internal/chat/open_*.go`):**
    - O Antigravity/Language Server abre conexões SQLite em modo WAL (`<uuid>.db`, `<uuid>.db-wal`, `<uuid>.db-shm`) estritamente para as conversas que estão abertas em abas no momento.
    - No Linux/Unix (`open_unix.go`): inspeção direta de descritores de arquivos em `/proc/[pid]/fd/*` via `os.Readlink` identificando links para `conversations/<uuid>.db*` (< 10ms em Go puro), com fallback automático para `lsof -Fn +D <conversationsDir>`.
    - No Windows (`open_windows.go`): verificação de arquivos `-wal`/`-shm` ativos com tamanho > 0 e teste atômico de abertura com captura de `ERROR_SHARING_VIOLATION` (32) / `ERROR_LOCK_VIOLATION` (33).
  - **Enriquecimento de Metadados e Visualização:**
    - Adicionado campo `InUse bool `json:"in_use"` em `chat.ConversationInfo`.
    - Ordenação de listagem: conversas em uso aparecem priorizadas no topo da tabela com status `in use`.
    - Filtro `--in-use` (alias `--open`) adicionado a `multigravity ai list`, contrato JSON e API REST (`GET /api/v1/profiles/{name}/conversations?filter=in_use`).
  - **Sync e Export Granular Seguro (`--skip-in-use`, `--safe-only`):**
    - Regra de Ouro #4 preservada por padrão: sem a flag, o comando continua abortando caso o perfil esteja em execução.
    - Com `--skip-in-use`, detecta quais chats estão abertos pelo SO, emite aviso informativo (ex: `⚠ Profile 'yegear' has 2 open conversation(s) in use. Skipping in-use conversation(s)...`) e exporta/sincroniza com segurança todas as conversas inativas.

### 2026-09-24 [Task 90.1] Preservação de Scripts Legados em Branch Separada e Remoção do Diretório legacy/

- **Contexto:** A reescrita em Go atingiu 100% de paridade funcional com a implementação legada e superou o legado em features (REST, SSE, --json, handles de SO). O usuário solicitou arquivar/preservar os scripts legados em uma branch separada por precaução e removê-los da branch `main`.
- **Decisões Técnicas:**
  - **Preservação em Branch Dedicada (`legacy`):**
    - Criada a branch local `legacy` a partir do estado da branch `main`, preservando integralmente o diretório `legacy/` (`legacy/multigravity` e `legacy/multigravity.ps1`) com seu histórico Git completo.
  - **Expurgo Cirúrgico na Branch `main`:**
    - Diretório `legacy/` removido do índice e da árvore de trabalho via `git rm -r legacy`.
    - Launchers inteligentes da raiz (`multigravity` e `multigravity.ps1`) atualizados: remoção do passo 3 de fallback para scripts legados. Em caso de ausência do binário e do compilador `go`, o launcher agora exibe mensagem clara instruindo a compilar com Go ou baixar a release.
    - Scripts de instalação (`install.sh` e `install.ps1`): remoção da opção de fallback para scripts legados remotos. Caso o download da release ou a compilação local falhe, o instalador aborta com diagnóstico preciso.
    - `AGENTS.md`: atualizado para remover menção aos scripts em `legacy/` e ajustar a validação de sintaxe para `bash -n multigravity install.sh uninstall.sh`.
