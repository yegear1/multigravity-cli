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


