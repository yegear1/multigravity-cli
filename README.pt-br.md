![Multigravity](assets/multigravity-logo.jpg)

# Multigravity

**Execute múltiplos perfis do Antigravity (e `agy`) simultaneamente — cada um com suas próprias contas, extensões, configurações e conversas de IA.**

Chega de fazer login e logout a todo momento. Abra quantos perfis precisar, todos ao mesmo tempo.

[English](README.md) | **Português**

[![Repositório GitHub](https://img.shields.io/badge/GitHub-Repository-blue?logo=github)](https://github.com/yegear1/multigravity-cli)
[![Perfil GitHub](https://img.shields.io/badge/GitHub-Profile-lightgrey?logo=github)](https://github.com/yegear1)
[![Licença: MIT](https://img.shields.io/badge/Licen%C3%A7a-MIT-yellow.svg)](LICENSE)
[![Plataformas](https://img.shields.io/badge/plataforma-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey)](#instalação)

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

- **Menu TUI Interativo:** Execute `multigravity` sem argumentos em um terminal interativo para abrir uma interface rápida de seleção com indicadores de status ao vivo (`● em execução` / `○ ocioso`).
- **Pronto para Antigravity 2.0 (`agy`):** Detecta nativamente os binários `antigravity` e `agy`.
- **Temas Visuais de Janela:** Atribua cores distintas na barra de título e no workbench por perfil (`--color` ou `multigravity color`) para nunca confundir janelas pessoais com as do trabalho.
- **Dotfiles de Dev Preservados:** Os arquivos `.gitconfig` e chaves `~/.ssh` da sua máquina são vinculados automaticamente nos perfis isolados, garantindo que commits Git e conexões SSH funcionem de imediato (com opção `--isolated-dotfiles` caso prefira isolamento estrito).
- **Gerenciamento Seguro de Ciclo de Vida:** Comandos `stop` e `restart` graciosos, com travas de concorrência que impedem excluir ou renomear perfis em execução.
- **Limpeza de Cache e Backups Otimizados:** O comando `multigravity clean` recupera gigabytes de caches voláteis do Electron/Chromium, e o `export` remove caches automaticamente para gerar backups leves e rápidos.
- **Migração Granular de Sessões de IA:** `multigravity ai export` / `import` permite transferir bancos de conversas e artefatos de IA entre perfis ou máquinas com total sanitização e sem expor tokens OAuth ou credenciais.

---

## Comandos

### Gerenciamento de Perfis

| Comando | Descrição |
|---------|-----------|
| `multigravity` | Abre o seletor interativo de perfis (TUI) |
| `multigravity new <nome>` | Cria um novo perfil completo e isolado |
| `multigravity new <nome> --shared` | Cria um perfil compartilhado leve (extensões e configurações compartilhadas, contas isoladas) |
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
| `multigravity list` | Lista todos os perfis cadastrados |
| `multigravity status` | Exibe status de execução, tipo, última utilização e tamanho de cada perfil |
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

### Utilitários

| Comando | Descrição |
|---------|-----------|
| `multigravity stats` | Exibe o uso de disco detalhado por perfil |
| `multigravity doctor` | Diagnostica o ambiente, caminhos e detecção de binários (`antigravity` / `agy`) |
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

## Perfis Compartilhados (Shared)

Os perfis completos são totalmente isolados — extensões, configurações e contas separadas. Esse é o padrão.

Os **perfis compartilhados** são mais leves: utilizam links simbólicos para as extensões e configurações da sua instalação principal do Antigravity, isolando apenas as contas e a camada de autenticação. Ideal para quando você precisa de uma conta secundária sem duplicar gigabytes de extensões.

```bash
multigravity new cliente-x --shared
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
- **Fork Aprimorado e Mantenedor:** [yegear1](https://github.com/yegear1)
