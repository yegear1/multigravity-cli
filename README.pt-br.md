![Multigravity](assets/multigravity-logo.jpg)

# Multigravity

**A Plataforma de Desenvolvimento Agêntico e Gateway de IA Multi-Contas para o Google Antigravity (e `agy`).**

Orquestre agentes autônomos de código, compartilhe e balanceie cotas de IA entre múltiplas contas Google com auto-failover, gerencie perfis isolados da IDE e execute tarefas em Git worktrees efêmeros.

[English](README.md) | **Português**

[![Repositório GitHub](https://img.shields.io/badge/GitHub-Repository-blue?logo=github)](https://github.com/yegear1/multigravity-cli)
[![Perfil GitHub](https://img.shields.io/badge/GitHub-Profile-lightgrey?logo=github)](https://github.com/yegear1)
[![Licença: MIT](https://img.shields.io/badge/Licen%C3%A7a-MIT-yellow.svg)](LICENSE)
[![Plataformas](https://img.shields.io/badge/plataforma-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey)](#instalação)
[![Showcase e Onboarding Interativo](https://img.shields.io/badge/Showcase-Demo%20ao%20Vivo-6366f1?logo=googlechrome&logoColor=white)](https://yegear1.github.io/multigravity-cli/)

> 🌐 **Showcase e Onboarding Interativo:** Confira nossa landing page visual com simulador de terminal em tempo real, guia por sistema operacional e calculadora de economia de disco em **[yegear1.github.io/multigravity-cli](https://yegear1.github.io/multigravity-cli/)**.

---

## Instalação

**macOS / Linux**

```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/yegear1/multigravity-cli/main/install.sh)"
```

**Windows** — abra o PowerShell e execute:

```powershell
irm https://raw.githubusercontent.com/yegear1/multigravity-cli/main/install.ps1 | iex
```

---

## Início Rápido

```bash
# Abre o menu interativo (TUI) para inspecionar, selecionar ou criar perfis
multigravity

# Cria novos perfis com cores personalizadas de tema
multigravity new trabalho --color blue
multigravity new pessoal --color emerald

# Inicia um perfil existente
multigravity trabalho

# Repassa argumentos diretamente para o Antigravity / agy
multigravity trabalho .
multigravity trabalho caminho/do/projeto
multigravity trabalho --new-window
```

Cada perfil recebe automaticamente um atalho executável integrado ao sistema operacional:

| Plataforma | Localização do Atalho |
|------------|-----------------------|
| macOS      | `~/Applications/Multigravity <nome>.app` |
| Windows    | Menu Iniciar → Programas |
| Linux      | `~/.local/share/applications/multigravity-<nome>.desktop` |

---

## Principais Recursos

- **Gateway de IA Multi-Contas:** Endpoints locais compatíveis com OpenAI (`/v1/chat/completions`) e Anthropic (`/v1/messages`) com auto-failover transparente em HTTP 429/403 e balanceamento inteligente entre contas.
- **Orquestrador de Agentes Autônomos:** Despacho de agentes CLI (Claude Code, Aider, OpenCode) com multiplexador de terminais PTY virtuais, ambiente isolado por perfil e streaming de logs em tempo real (`multigravity dispatch`).
- **Git Worktrees Efêmeros:** Provisionamento de branches e diretórios de trabalho isolados por tarefa em `.multigravity/worktrees/`, automaticamente ignorados pelo rastreamento Git do host (`multigravity worktree`).
- **Visualizador Web de Diffs Integrado:** Painel web de tarefas (`/ui/tasks`) e visualizador de diffs lado a lado/unificado (`/ui/tasks/:id/diff`) embutidos diretamente no binário Go, sem dependências externas.
- **Inteligência de Workspaces e Repositórios:** Mapeamento automático de projetos, branches ativas e identificação de qual perfil detém determinado diretório (`multigravity workspace`).
- **Menu TUI Interativo:** Execute `multigravity` sem argumentos em um terminal interativo para abrir uma interface rápida de seleção com indicadores de status ao vivo (`● em execução` / `○ ocioso`).
- **Pronto para Antigravity 2.0 (`agy`):** Detecta nativamente os binários `antigravity` e `agy` em Linux, macOS e Windows.
- **Perfis Auth-Only Ultraleves:** Crie perfis em segundos com extensões e preferências compartilhadas do host com apenas ~2 MB de consumo de disco (`--auth-only` / `--shared`).
- **Temas Visuais de Janela:** Atribua cores distintas na barra de título e no workbench por perfil (`--color` ou `multigravity color`) para nunca confundir janelas pessoais com as do trabalho.
- **Dotfiles de Dev Preservados:** Os arquivos `.gitconfig` e chaves `~/.ssh` da sua máquina são vinculados automaticamente nos perfis isolados (com opção `--isolated-dotfiles`).
- **Gerenciamento Seguro de Ciclo de Vida:** Comandos `stop` e `restart` graciosos, com travas de concorrência que impedem excluir ou renomear perfis em execução.
- **Migração Granular de Sessões de IA:** `multigravity ai export` / `import` permite transferir bancos de conversas e artefatos de IA entre perfis ou máquinas com total sanitização de tokens OAuth.

---

## Comandos

### Gerenciamento de Perfis

| Comando | Descrição |
|---------|-----------|
| `multigravity` | Abre o seletor interativo de perfis (TUI) |
| `multigravity new <nome>` | Cria um novo perfil completo e isolado |
| `multigravity new <nome> --auth-only` | Cria um perfil auth-only (~2 MB: extensões e configurações do host compartilhadas, contas isoladas; alias: `--shared`) |
| `multigravity new <nome> --from <modelo>` | Cria um perfil a partir de um modelo salvo |
| `multigravity new <nome> --color <cor>` | Cria um perfil com tema de cor personalizado na janela |
| `multigravity new <nome> --isolated-dotfiles` | Não vincula `.gitconfig` ou `.ssh` do host ao perfil |
| `multigravity new <nome> --isolated-mcp` | Não compartilha servidores do Model Context Protocol (MCP) do host |
| `multigravity new <nome> --isolated-skills` | Não compartilha skills globais e plugins do host |
| `multigravity new <nome> --isolated-config` | Não compartilha o config.json e permissões do assistente do host |
| `multigravity new <nome> --isolated-gh` | Não compartilha as credenciais do GitHub CLI do host |
| `multigravity <nome> [args...]` | Inicia um perfil (repassando argumentos para a IDE) |
| `multigravity stop <nome> [--force]` | Encerra um perfil em execução graciosamente (ou forçado) |
| `multigravity restart <nome>` | Reinicia um perfil em execução |
| `multigravity color <nome> [cor]` | Define, exibe ou redefine a cor do tema da janela do perfil |
| `multigravity clean <nome\|--all>` | Remove caches voláteis do Electron/Chromium para liberar disco |
| `multigravity list [--json]` | Lista todos os perfis cadastrados (ou formato JSON) |
| `multigravity status [nome] [--json]` | Exibe status de execução, tipo, última utilização e tamanho de cada perfil (ou formato JSON) |
| `multigravity clone <origem> <destino>` | Clona um perfil existente |
| `multigravity rename <antigo> <novo>` | Renomeia um perfil (bloqueado se estiver aberto) |
| `multigravity delete <nome>` | Exclui um perfil e todos os seus dados (bloqueado se estiver aberto) |

### Sessões e Conversas de IA

| Comando | Descrição |
|---------|-----------|
| `multigravity ai list <nome>` | Lista títulos de conversas de IA e contagem de artefatos do perfil |
| `multigravity ai export <nome> [caminho]` | Exporta conversas e dados do cérebro da IA (sanitizado de tokens OAuth e chaves) |
| `multigravity ai import <arquivo> <nome>` | Importa conversas de IA para um perfil existente de forma não-destrutiva |
| `multigravity ai sync <origem> <destino>` | Sincroniza conversas de IA diretamente entre dois perfis locais |
| `multigravity mcp status <nome>` | Consulta o status de compartilhamento de servidores MCP |
| `multigravity mcp share <nome>` | Compartilha os servidores MCP do host (`~/.gemini/config/mcp_config.json`) |
| `multigravity mcp isolate <nome>` | Isola o perfil com uma cópia independente das configurações MCP |
| `multigravity skills status <nome>` | Consulta o status de compartilhamento de skills e plugins |
| `multigravity skills share <nome>` | Compartilha skills e plugins do host (`~/.gemini/config/skills`, `plugins`) |
| `multigravity skills isolate <nome>` | Isola o perfil com uma cópia independente de skills e plugins |
| `multigravity config status <nome>` | Consulta o status de compartilhamento de permissões e config.json |
| `multigravity config share <nome>` | Compartilha config.json e permissões do host (`~/.gemini/config/config.json`) |
| `multigravity config isolate <nome>` | Isola o perfil com uma cópia independente do config.json |
| `multigravity config seed [nome\|--all\|--host]` | Semeia permissões padrão read-only (git, posix, npm, pnpm, uv) no config.json |
| `multigravity gh status <nome>` | Consulta o status de compartilhamento de credenciais do GitHub CLI |
| `multigravity gh share <nome>` | Compartilha as credenciais do GitHub CLI (`~/.config/gh` ou `%APPDATA%\GitHub CLI`) |
| `multigravity gh isolate <nome>` | Isola o perfil com cópia local independente do GitHub CLI |
| `multigravity quota [nome]` | Exibe limites de tokens, porcentagem de uso e contagem regressiva para o reset |
| `multigravity ai quota [nome]` | Alias para `multigravity quota` |
| `multigravity prime [nome] [opt]` | Prime automático dos ciclos semanais de tokens no reset (dual-bucket, jitter, cron/systemd) |
| `multigravity ai prime [nome] [opt]` | Alias para `multigravity prime` |

### Agentes Autônomos e Despacho de Tarefas

| Comando | Descrição |
|---------|-----------|
| `multigravity dispatch run <cmd> [flags]` | Despacha uma tarefa de agente autônomo com isolamento de perfil e worktree opcional (alias: `dp run`) |
| `multigravity dispatch list [--json]` | Lista tarefas despachadas com status, duração e metadados (alias: `dp list`) |
| `multigravity dispatch status <task-id> [--json]` | Exibe o estado de execução detalhado e manifesto de uma tarefa |
| `multigravity dispatch logs <task-id> [-f\|--tail N]` | Exibe ou transmite logs de execução em tempo real |
| `multigravity dispatch diff <task-id> [--web\|--structured\|--json]` | Inspeciona diff Git no terminal, tabela estruturada, JSON ou abre visualizador web |
| `multigravity dispatch dashboard [--json]` | Dashboard executivo com contadores de tarefas em execução, concluídas e com falha |
| `multigravity dispatch cancel <task-id> [--force]` | Cancela uma tarefa em execução graciosamente (ou forçado) |
| `multigravity dispatch delete <task-id> [--worktree]` | Exclui registro da tarefa e limpa opcionalmente seu worktree |
| `multigravity dispatch prune [--max-age <dur>]` | Remove tarefas concluídas anteriores ao limite de retenção |
| `multigravity agent run <perfil> [--] <cmd>` | Executa agente CLI interativo (Claude Code, Aider, OpenCode) em PTY dedicado (alias: `ag run`) |
| `multigravity agent list [--json]` | Lista sessões ativas de agentes em PTY |
| `multigravity agent attach <id>` | Conecta o terminal diretamente a uma sessão de agente em PTY |
| `multigravity agent stop <id>` | Encerra uma sessão PTY de agente graciosamente |

### Git Worktrees Efêmeros

| Comando | Descrição |
|---------|-----------|
| `multigravity worktree list [--repo <caminho>] [--json]` | Lista git worktrees ativos gerenciados pelo Multigravity (alias: `wt list`) |
| `multigravity worktree create <nome> [--branch <b>] [--json]` | Cria um worktree efêmero em `.multigravity/worktrees/` |
| `multigravity worktree status <nome> [--json]` | Consulta status Git, arquivos modificados e untracked no worktree |
| `multigravity worktree diff <nome> [--stat] [--json]` | Exibe o diff gerado no worktree em relação à branch base |
| `multigravity worktree remove <nome> [--force]` | Limpa e remove um worktree efêmero |
| `multigravity worktree prune` | Remove registros de worktrees órfãos ou obsoletos |

### Workspaces e Repositórios

| Comando | Descrição |
|---------|-----------|
| `multigravity workspace list [--active] [--json]` | Lista workspaces mapeados e status Git entre perfis (alias: `ws list`) |
| `multigravity workspace active [--json]` | Lista workspaces abertos em instâncias ativas da IDE |
| `multigravity workspace current [--json]` | Detecta qual perfil e workspace detêm o diretório atual de trabalho (alias: `ws here`) |
| `multigravity workspace show <perfil> <ws> [--json]` | Exibe status detalhado do repositório, branch ativa e políticas |

### Modelos (Templates)

| Comando | Descrição |
|---------|-----------|
| `multigravity template save <perfil> <nome>` | Salva um perfil existente como modelo reutilizável |
| `multigravity template list` | Lista todos os modelos salvos |
| `multigravity template delete <nome>` | Remove um modelo |

### Backup e Transferência

| Comando | Descrição |
|---------|-----------|
| `multigravity export <nome> [caminho] [--include-cache]` | Compacta um perfil em `.tar.gz` (`.zip` no Windows), enxuto por padrão |
| `multigravity import <arquivo> [nome]` | Restaura um perfil a partir de um arquivo compactado |

### Servidor e Gateway de IA

| Comando | Descrição |
|---------|-----------|
| `multigravity serve [--port <p>] [--host <h>]` | Inicia daemon HTTP local com API REST, streaming SSE, Gateway de IA (`/v1/chat/completions`, `/v1/messages`) e UI Web (`/ui/tasks`) |
| `multigravity stats [--json]` | Exibe o uso de disco detalhado por perfil |
| `multigravity doctor [--json]` | Diagnostica o ambiente, caminhos e detecção de binários (`antigravity` / `agy`) |
| `multigravity update` | Atualiza o Multigravity para a versão mais recente |
| `multigravity completion` | Configura o autocompletar de comandos no shell |
| `multigravity version` | Exibe a versão do Multigravity |
| `multigravity help` | Exibe a mensagem de ajuda |

---

## Temas Visuais de Janela

Diferencie seus ambientes de trabalho rapidamente com cores de destaque:

```bash
# Cores nomeadas suportadas: blue, green, emerald, red, purple, orange, cyan, pink, indigo, slate, etc.
multigravity color trabalho blue
multigravity color pessoal emerald

# Ou use qualquer código hexadecimal customizado
multigravity color cliente-x "#8b5cf6"

# Para remover a personalização e voltar ao padrão:
multigravity color trabalho --reset
```

---

## Migração e Sincronização de Conversas de IA

Transfira ou sincronize histórico de conversas do Gemini/Antigravity e conhecimento de artefatos entre perfis com segurança:

```bash
# Listar conversas em um perfil
multigravity ai list trabalho

# Exportar chats de IA (remove automaticamente tokens sensíveis de OAuth e credenciais)
multigravity ai export trabalho ./conversas-trabalho.tar.gz

# Importar para outro perfil sem sobrescrever conversas existentes
multigravity ai import ./conversas-trabalho.tar.gz pessoal

# Sincronização direta entre dois perfis locais sem gerar arquivos temporários
multigravity ai sync trabalho pessoal
```

---

## Compartilhamento de Servidores MCP (Model Context Protocol)

Por padrão, todos os perfis vinculam as configurações do Model Context Protocol do host (`~/.gemini/config/mcp_config.json` e schemas em `~/.gemini/antigravity/mcp`), permitindo acesso imediato aos servidores e ferramentas MCP locais sem retrabalho de configuração:

```bash
# Consultar o status de compartilhamento de MCP de um perfil
multigravity mcp status trabalho

# Criar um perfil isolado dos servidores MCP do host
multigravity new cliente-x --isolated-mcp

# Alternar um perfil existente entre os modos compartilhado e isolado
multigravity mcp isolate trabalho
multigravity mcp share trabalho
```

---

## Compartilhamento de Skills e Plugins

Por padrão, todos os perfis vinculam as customizações e plugins globais do host (`~/.gemini/config/skills` e `~/.gemini/config/plugins`), fornecendo acesso imediato às skills e fluxos de trabalho do assistente em qualquer perfil:

```bash
# Consultar o status de compartilhamento de skills/plugins de um perfil
multigravity skills status trabalho

# Criar um perfil com skills e plugins isolados
multigravity new cliente-x --isolated-skills

# Alternar um perfil existente entre os modos compartilhado e isolado
multigravity skills isolate trabalho
multigravity skills share trabalho
```

---

## Compartilhamento de Configuração e Permissões (`config.json`)

Por padrão, todos os perfis vinculam as configurações do assistente do host (`~/.gemini/config/config.json`), compartilhando permissões globais e preferências de interface em tempo real entre janelas sem duplicar aprovações.

Além disso, o multigravity **semeia automaticamente permissões padrão de leitura** no *"Always allow"* (`.userSettings.globalPermissionGrants.allow`) para execução tanto em sandbox padrão (`command(...)`) quanto em bypass (`unsandboxed(...)`). Isso inclui:
- **Git:** `git status`, `git log`, `git diff`, `git show`, `git branch`, `git tag`, `git remote`, etc.
- **POSIX e Sistema:** `ls`, `cat`, `head`, `tail`, `grep`, `rg`, `find`, `which`, `stat`, `df`, `ps`, etc.
- **Ferramentas Dev e Linters:** `npm test`, `npm run lint/check`, `pnpm test/lint`, `uv run pytest/ruff/pyright/mypy`, `ruff check`, `eslint`, `tsc --noEmit`, etc.

```bash
# Consultar o status de compartilhamento de config/permissões de um perfil
multigravity config status trabalho

# Semear ou atualizar permissões padrão de leitura em todos os perfis
multigravity config seed --all

# Criar um perfil com config e permissões isoladas (semeadas automaticamente)
multigravity new cliente-x --isolated-config

# Alternar um perfil existente entre os modos compartilhado e isolado
multigravity config isolate trabalho
multigravity config share trabalho
```

---

## Telemetria de Cotas e Limites de Tokens (`multigravity quota`)

Consulte o consumo de tokens de IA em tempo real, porcentagem de uso, fração restante e contagem regressiva exata para o reset entre perfis do Antigravity em execução:

```bash
# Consulta cotas de todos os perfis ativos
multigravity quota

# Consulta cota de um perfil específico
multigravity quota trabalho
multigravity ai quota trabalho
```

---

## Gateway de IA Multi-Contas e Auto-Failover

O Multigravity inclui um Gateway local de IA em `multigravity serve` que disponibiliza endpoints compatíveis com OpenAI (`/v1/chat/completions`) e Anthropic (`/v1/messages`), alimentados pelas suas contas isoladas do Antigravity/Google.

### Recursos Principais
- **Pooling Multi-Contas e Auto-Failover:** Se um perfil ativo atingir limites de taxa (HTTP 429 ou esgotamento de cota 403), o gateway coloca o perfil em cooldown e alterna automaticamente para o próximo perfil com cota saudável sem interromper o streaming do cliente.
- **Estratégias de Roteamento Configuráveis:**
  - `smart` (padrão): Prioriza perfis com maior cota disponível, penaliza taxa de erros e balanceia carga dinamicamente.
  - `round-robin`: Rotaciona requisições sequencialmente entre os perfis saudáveis.
  - `priority`: Utiliza a ordem declarada de prioridade dos perfis.
  - `sticky`: Mantém as requisições no mesmo perfil até que ocorra um limite de taxa.
- **Mapeamento de Modelos:** Suporte nativo aos modelos Gemini (`gemini-2.5-pro`, `gemini-2.5-flash`, `gemini-3.5-flash`) e aliases transparentes de terceiros (`claude-3-7-sonnet`, `claude-3-5-sonnet`, `claude-opus`, `gpt-4o`).
- **Zero Custódia de Credenciais:** As chamadas utilizam os tokens em memória do Language Server local; senhas ou segredos da conta Google nunca são persistidos em texto puro.

### Exemplos de Uso

```bash
# 1. Iniciar o daemon do Gateway Multigravity
multigravity serve

# 2. Requisitar via endpoint OpenAI Chat Completions (com streaming SSE)
curl -s -N http://127.0.0.1:8989/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gemini-2.5-pro",
    "messages": [{"role": "user", "content": "Explique Git worktrees em uma frase."}],
    "stream": true
  }'

# 3. Requisitar via endpoint Anthropic Messages
curl -s http://127.0.0.1:8989/v1/messages \
  -H "Content-Type: application/json" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-3-7-sonnet",
    "messages": [{"role": "user", "content": "Olá Claude!"}],
    "max_tokens": 100
  }'

# 4. Apontar o Claude Code CLI para o Gateway do Multigravity
export ANTHROPIC_BASE_URL="http://127.0.0.1:8989"
claude

# 5. Apontar Aider / OpenCode / Continue para o Gateway do Multigravity
export OPENAI_BASE_URL="http://127.0.0.1:8989/v1"
aider --model gpt-4o
```

---

## Orquestrador de Agentes Autônomos e Despacho de Tarefas

O Multigravity coordena agentes autônomos de desenvolvimento (Claude Code, Aider, OpenCode, Agy) em tarefas de background ou pseudoterminais interativos (PTY), isolados em perfis dedicados e Git worktrees efêmeros.

```
┌─────────────────────────────────────────────────────────────┐
│                    multigravity dispatch                    │
│                                                             │
│   ┌────────────────┐   ┌────────────────┐   ┌───────────┐   │
│   │ Autenticação   │   │ Git Worktree   │   │ Terminal  │   │
│   │ e Pool de Cota │ + │ Efêmero        │ + │ Virtual   │   │
│   │ (~/Antigravity)│   │ (.multigravity)│   │ PTY (TTY) │   │
│   └────────────────┘   └────────────────┘   └───────────┘   │
└──────────────────────────────┬──────────────────────────────┘
                               │
            ┌──────────────────┴──────────────────┐
            ▼                                     ▼
     Logs e Streaming                     Visualizador Web de Diffs
  (multigravity dispatch logs)         (http://localhost:8989/ui/tasks)
```

### Despachando Tarefas com Git Worktrees Efêmeros

```bash
# Despachar tarefa em um worktree isolado sem afetar a branch atual
multigravity dispatch run --profile trabalho --worktree --prompt "Refatorar autenticação em auth.go"

# Acompanhar logs de execução em tempo real
multigravity dispatch logs <task-id> -f

# Inspecionar diff estruturado gerado pelo agente no terminal
multigravity dispatch diff <task-id> --structured

# Abrir visualizador gráfico interativo de diffs no navegador
multigravity dispatch diff <task-id> --web
# Ou acesse o painel diretamente: http://127.0.0.1:8989/ui/tasks
```

### Sessões Interativas de PTY

Para agentes CLI que exigem confirmações interativas no terminal (`[y/n]`, concessão de ferramentas, ANSI rico):

```bash
# Executa Claude Code ou Aider dentro de sessão PTY isolada
multigravity agent run trabalho -- claude

# Lista e reconecta a sessões ativas de agentes em PTY
multigravity agent list
multigravity agent attach <session-id>
```

---

## Inteligência de Workspaces e Repositórios

Rastreie quais projetos pertencem a cada perfil do Antigravity, descubra repositórios ativos e saiba onde você está sem sair do terminal:

```bash
# Mostra qual perfil e workspace detêm o diretório atual
multigravity workspace current

# Lista todos os workspaces mapeados com status Git (branch, clean/dirty)
multigravity workspace list

# Filtra apenas workspaces abertos em instâncias ativas da IDE
multigravity workspace active

# Exibe detalhes de um workspace específico
multigravity workspace show trabalho backend-api
```

---

## Perfis Auth-Only (`--auth-only` / `--shared`)

Os perfis completos são ambientes totalmente isolados — extensões, configurações, caches e contas separadas. Esse é o padrão.

Os **perfis auth-only** adotam uma abordagem enxuta: utilizam links simbólicos para a pasta `extensions/` e configurações essenciais do editor (`settings.json`, `keybindings.json`, `snippets`) diretamente da instalação host do Antigravity, isolando **estritamente a camada de autenticação/conta e o histórico de IA**. Isso proporciona um consumo de disco de apenas **~2 MB** por perfil (em comparação aos **~500 MB+** de uma instalação completa), preservando 100% de sessões de login e cotas de tokens independentes entre contas Google.

### Matriz Comparativa: Full vs. Auth-Only

| Recurso / Aspecto | Perfil Completo (Padrão) | Perfil Auth-Only (`--auth-only` / `--shared`) |
| :--- | :--- | :--- |
| **Comando de Criação** | `multigravity new <nome>` | `multigravity new <nome> --auth-only` *(ou `--shared`)* |
| **Uso Inicial de Disco** | **~500 MB** *(cresce com extensões duplicadas e caches)* | **~2 MB** *(enxuto, quase zero espaço adicional em disco)* |
| **Contas Antigravity / Gemini** | **Isoladas** *(tokens OAuth e logins Google independentes)* | **Isoladas** *(tokens OAuth e logins Google independentes)* |
| **Cotas de IA e Limites de Tokens** | **Independentes** *(gerenciadas por conta Google separada)* | **Independentes** *(gerenciadas por conta Google separada)* |
| **Histórico e Chats de IA** | **Isolados** *(gerenciados via `ai export/import/sync`)* | **Isolados** *(gerenciados via `ai export/import/sync`)* |
| **Extensões da IDE** | **Isoladas** *(exige instalar extensões em cada perfil)* | **Compartilhadas via symlink** *(espelha extensões do host de imediato)* |
| **Configurações e Atalhos** | **Isolados** *(`User/settings.json`, keybindings, snippets)* | **Compartilhados via symlink** *(espelha preferências do host)* |
| **Temas de Janela (`--color`)** | **Suportado** *(cor personalizada na barra de título e acentos)* | **Suportado** *(desacopla `settings.json` com segurança e sem alterar o host)* |
| **Dotfiles Dev (`.gitconfig`, `.ssh`)** | **Vinculados por padrão** *(opt-out via `--isolated-dotfiles`)* | **Vinculados por padrão** *(opt-out via `--isolated-dotfiles`)* |
| **GitHub CLI (`gh`) e Config** | **Vinculados por padrão** *(opt-out via `--isolated-gh`)* | **Vinculados por padrão** *(opt-out via `--isolated-gh`)* |
| **Ideal Para** | Stacks divergentes (Trabalho vs Pessoal, linters e extensões distintas) | Rotação de cotas entre contas Google, logins secundários mantendo o mesmo ferramental |

### Qual Tipo de Perfil Escolher?

- **Escolha Perfil Completo** se:
  - Você precisa de conjuntos de extensões completamente diferentes para cada projeto (ex: backend Go vs mobile Flutter vs machine learning em Python).
  - Você deseja testar atalhos ou configurações experimentais de editor sem interferir no seu ambiente diário.
  - Você precisa de segregação corporativa rígida entre ambientes da empresa e pessoais.

- **Escolha Perfil Auth-Only** se:
  - Seu objetivo principal é alternar **múltiplas contas Google** para rotacionar cotas semanais e janelas de 5 horas do Gemini.
  - Você quer manter exatamente o mesmo tema, extensões, snippets e atalhos de teclado do host sem reinstalar nada.
  - Você deseja criar novos perfis em segundos com consumo de disco residual (~2 MB vs ~500 MB).

```bash
# Cria um perfil auth-only (alias: --shared)
multigravity new conta-secundaria --auth-only

# Cria um perfil auth-only com tema de cor personalizado
multigravity new conta-secundaria --auth-only --color purple
```

---

## Modelos (Templates)

Salve um perfil já configurado como modelo e gere novos perfis a partir dele instantaneamente:

```bash
# Salve sua configuração ideal como modelo
multigravity template save trabalho base

# Crie novos perfis a partir deste modelo
multigravity new projeto-a --from base
multigravity new projeto-b --from base

# Veja os modelos disponíveis
multigravity template list
```

## Integração de Skill para Agentes de IA

O Multigravity inclui uma Skill canônica para agentes de IA ([`skills/multigravity/SKILL.md`](skills/multigravity/SKILL.md)) seguindo o padrão `agent-skills`, ensinando assistentes autônomos (como Antigravity e Cursor) a inspecionar cotas, acionar ciclos de priming, gerenciar fronteiras de isolamento de perfis e sincronizar brains de conversas com total segurança.

Para instalar e sincronizar a skill nas pastas globais de descoberta da IA (`~/.gemini/config/skills` e `~/.cursor/skills`):

```bash
./scripts/install-agent-skills.sh
```

---

## Autocompletar no Shell

Ative o autocompletar via tecla Tab para comandos e nomes de perfis:

```bash
multigravity completion
```

Siga as instruções exibidas para adicionar ao seu `.zshrc`, `.bashrc` ou `$PROFILE` do PowerShell.

---

## Desinstalação

**macOS / Linux**

```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/yegear1/multigravity-cli/main/uninstall.sh)"
```

**Windows**

```powershell
irm https://raw.githubusercontent.com/yegear1/multigravity-cli/main/uninstall.ps1 | iex
```

Você será perguntado se deseja remover os dados dos perfis — nenhum dado é excluído sem confirmação.

---

## Regras para Nomes de Perfis

Apenas letras, números e hifens. Deve começar com uma letra ou número.

```
✅  trabalho   cliente-a   teste1
❌  -nome      meu_perfil
```

---

## Licença

As modificações, melhorias e novos recursos introduzidos neste fork são licenciados sob a [Licença MIT](LICENSE).  
A base de código original permanece sob os direitos autorais de Sujit Agarwal e colaboradores originais, conforme os Termos de Serviço do GitHub.

---

## Créditos e Agradecimentos

- **Autor e Criador Original:** [Sujit Agarwal](https://github.com/sujitagarwal)
- **Suporte ao Windows:** [Samin Yeasar](https://github.com/Solez-ai)
- **Suporte ao Linux:** [Md Rayyan Nawaz](https://github.com/therayyanawaz)
- **Inspiração Comunitária (Multigravity Pro):** [Pulkit](https://github.com/Pulkit7070) (pioneirismo no conceito de perfis `--auth-only` e guias visuais de onboarding em [Pulkit7070/multigravity-pro](https://github.com/Pulkit7070/multigravity-pro))
- **Fork Aprimorado e Mantenedor:** [yegear1](https://github.com/yegear1)

