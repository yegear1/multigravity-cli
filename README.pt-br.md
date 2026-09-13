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

## Migração de Conversas de IA

Transfira histórico de conversas do Gemini/Antigravity e conhecimento de artefatos entre perfis ou máquinas de maneira segura:

```bash
# Listar conversas em um perfil
multigravity ai list trabalho

# Exportar chats de IA (remove automaticamente tokens sensíveis de OAuth e credenciais)
multigravity ai export trabalho ./conversas-trabalho.tar.gz

# Importar para outro perfil sem sobrescrever conversas existentes
multigravity ai import ./conversas-trabalho.tar.gz pessoal
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
