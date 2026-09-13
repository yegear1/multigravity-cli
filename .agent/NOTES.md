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
