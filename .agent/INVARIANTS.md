# INVARIANTS.md — Invariantes e Regras Intocáveis do Sistema

> 🧱 **O Princípio do Muro de Chesterton:**
> *"Nunca remova uma regra, filtro ou trecho de código estranho até descobrir por que ele foi colocado ali."*
>
> Este arquivo é a fonte da verdade para **comportamentos, peculiaridades e contratos do sistema legado** que PARECEM errados, redundantes ou feios à primeira vista, mas que **NÃO DEVEM SER ALTERADOS** sem aprovação explícita e justificada do desenvolvedor.

---

## 1. Contratos de Nomenclatura e Argumentos da CLI

| Ponto de Contato | Regra Inquebrável | Motivo / Impacto |
| :--- | :--- | :--- |
| `validate_name` | `^[a-zA-Z0-9][a-zA-Z0-9-]*$` | Nomes de perfis criam pastas no SO e arquivos `.desktop`/`.app`/`.lnk`. Espaços ou símbolos quebram escaping de shell e caminhos do XDG. |
| Invocação de Perfil | `multigravity <nome> [args...]` | O primeiro argumento quando não coincide com um comando interno deve ser interpretado como nome de perfil para lançamento imediato com passthrough de argumentos. |
| Flags do Antigravity | `--user-data-dir` e `--extensions-dir` | São as flags canônicas do Electron/Code OSS para isolamento de dados do usuário e extensões. Não alterar para flags customizadas. |

---

## 2. Variáveis de Ambiente e Diretórios Canônicos

- **`MULTIGRAVITY_HOME`:** Diretório base de armazenamento de perfis. Default: `$HOME/AntigravityProfiles` (ou `$USERPROFILE\AntigravityProfiles`).
- **`MULTIGRAVITY_APP`:** Permite sobrescrever o caminho exato do executável da IDE.
- **Isolamento de `HOME`:**
  - O Antigravity grava configurações do assistente de IA em `~/.gemini` e `~/.antigravity`.
  - Redirecionar `HOME` para o diretório do perfil é a forma como o CLI força o isolamento dessas pastas.
  - **Atenção:** Qualquer tentativa de remover a exportação de `HOME` sem alternativa quebrará o isolamento de sessões de IA. Por isso, a solução correta para Git/SSH é **preservar os dotfiles essenciais (`.gitconfig`, `.ssh`) via symlink** dentro do perfil, e não deixar de isolar o `HOME`.

---

## 3. Comportamento Específico de Plataforma

### macOS (Darwin)
- **Keychains:** `Library/Keychains` deve ser linkado para `$HOME/Library/Keychains` se não existir, caso contrário o macOS recusa salvar credenciais de forma persistente.
- **Atalhos:** Criados em `$HOME/Applications/Multigravity <nome>.app` contendo estrutura padrão de bundle macOS com `Contents/MacOS/run`, `Info.plist` e `Contents/Resources/icon.icns` (gravado a partir do asset embutido via `//go:embed`).

### Linux
- **XDG Base Directory:** `XDG_CONFIG_HOME`, `XDG_CACHE_HOME`, `XDG_DATA_HOME` e `XDG_STATE_HOME` devem apontar para as respectivas subpastas dentro do perfil para não poluir o sistema.
- **Atalhos:** Gerados em `~/.local/share/applications/multigravity-<nome>.desktop` apontando para o wrapper script em `~/.local/share/multigravity/launchers/<nome>.sh` e referenciando deterministicamente o ícone em `~/.local/share/multigravity/icon.png` (extraído dos assets embutidos via `//go:embed`).

### Windows
- **Variáveis de Usuário:** `$env:USERPROFILE`, `$env:APPDATA` e `$env:LOCALAPPDATA` devem ser apontados para as subpastas correspondentes dentro de `$PROFILE_DIR`.
- **Atalhos:** `.lnk` gerados em `$env:APPDATA\Microsoft\Windows\Start Menu\Programs\` com `IconLocation` apontando para `%APPDATA%\multigravity\icon.ico` (extraído dos assets embutidos via `//go:embed`).

---

## 4. Compartilhamento de MCP (Model Context Protocol)

- **Configuração Global vs Tokens:** O Antigravity armazena as definições dos servidores MCP em `~/.gemini/config/mcp_config.json` e schemas em `~/.gemini/antigravity/mcp`. Os tokens de autenticação (ex: `jetski-standalone-oauth-token`) residem na raiz de `~/.gemini/`, e **NÃO** dentro de `~/.gemini/config/`.
- **Symlink Seguro:** Vincular `~/.gemini/config/mcp_config.json` e `~/.gemini/antigravity/mcp` por symlink (ou Junction no Windows) **NÃO** expõe as credenciais de autenticação da IA do usuário host.
- **Opt-out (.isolated_mcp):** Qualquer perfil contendo o arquivo sentinela `.isolated_mcp` não deve ser sobrescrito nem receber links para o host, mantendo configuração e schemas totalmente privados e isolados.

---

## 5. Compartilhamento de Skills e Plugins

- **Customizações Globais vs Tokens:** Skills residem em `~/.gemini/config/skills/` e Plugins em `~/.gemini/config/plugins/`. Tratam-se apenas de arquivos de instrução markdown (`SKILL.md`), scripts e templates (`plugin.json`), sem nenhum token ou credencial.
- **Symlink Seguro:** Vincular `~/.gemini/config/skills` e `~/.gemini/config/plugins` por symlink (ou Junction no Windows) permite compartilhar o ecossistema de habilidades do assistente sem expor sessões ou credenciais.
- **Opt-out (.isolated_skills):** Perfis contendo o arquivo sentinela `.isolated_skills` mantêm suas pastas `skills/` e `plugins/` totalmente isoladas e privadas.

---

## 6. Compartilhamento de Configuração e Permissões (`config.json`)

- **Permissões Globais do Assistente:** As permissões de execução (aprovações persistidas de ferramentas MCP, comandos shell e Git concedidas via *"Always allow"*) são salvas em `~/.gemini/config/config.json` no campo `.userSettings.globalPermissionGrants.allow`.
- **Inexistência de Credenciais:** O `config.json` armazena apenas preferências de interface (ex: `conversationWidth`, `themeMode`) e o array de grants de segurança. Tokens OAuth ou senhas nunca são gravados neste arquivo.
- **Symlink Seguro e Tempo Real:** Vincular `~/.gemini/config/config.json` por symlink garante que concessões concedidas em qualquer janela reflitam imediatamente em todos os outros perfis em tempo real.
- **Opt-out (.isolated_config):** Qualquer perfil contendo o arquivo sentinela `.isolated_config` mantém seu próprio `config.json` desvinculado do host.
- **Preservação de Dados:** Migrações ou trocas para modo compartilhado via CLI realizam backup prévio (`config.json.bak`) de arquivos existentes.
- **Injeção Canônica de Permissões Read-Only:** Perfis criados, lançados ou semeados via `multigravity config seed` garantem a presença de permissões padrão em *"Always allow"* (`command(...)` para sandbox e `unsandboxed(...)` para bypass) para comandos estritamente de leitura (Git, POSIX, inspeção, npm/pnpm/uv lint/test/check). Essa injeção é aditiva, preservando quaisquer concessões manuais existentes sem duplicatas.

---

## 7. Compartilhamento de Credenciais de Dev e Preservação de PATH

- **Git e GitHub CLI (`gh`):** A autenticação do `gh` (`~/.config/gh` no Linux/macOS ou `%APPDATA%\GitHub CLI` no Windows) e o arquivo `~/.git-credentials` são compartilhados por symlink/junction por padrão para permitir pull/push e operações dev no terminal integrado sem atrito.
- **Opt-out granular (.isolated_gh):** Perfis contendo o arquivo sentinela `.isolated_gh` ou `.isolated_dotfiles` mantêm suas credenciais do GitHub CLI estritamente isoladas, permitindo perfis dedicados a múltiplas contas/identidades.
- **Preservação de PATH de Usuário:** O Antigravity enriquece o `PATH` com os diretórios de binários de usuário do host (`~/.local/bin`, `~/.cargo/bin`, etc.) no momento do lançamento (`launch_profile`), assegurando acesso aos executáveis do usuário sem quebrar o isolamento de `HOME`.

---

## 8. Mapeamento Canônico de Diretórios do Antigravity

Mapa de referência de onde a IDE Antigravity e seus subsistemas de IA armazenam dados, credenciais, configurações e artefatos (relativos ao `$HOME` do perfil ou do host):

| Recurso | Caminho Canônico no Antigravity | Finalidade / Conteúdo | Isolamento / Sensibilidade |
| :--- | :--- | :--- | :--- |
| **Skills** | `~/.gemini/config/skills/<skill-name>/SKILL.md` | Playbooks markdown, instruções, scripts e recursos dos agentes. | Seguro compartilhar via symlink (sem credenciais). Opt-out: `.isolated_skills`. |
| **Plugins** | `~/.gemini/config/plugins/<plugin-name>/` | Plugins da IDE e manifests de extensão de IA. | Seguro compartilhar via symlink. Opt-out: `.isolated_skills`. |
| **Permissões / Always Allow** | `~/.gemini/config/config.json` | Array `.userSettings.globalPermissionGrants.allow` com comandos aprovados (`command(...)`, `unsandboxed(...)`). | Seguro compartilhar via symlink (apenas preferências e grants, zero tokens). Opt-out: `.isolated_config`. |
| **MCP Config** | `~/.gemini/config/mcp_config.json` | Definições e endpoints de servidores MCP globais. | Seguro compartilhar via symlink. Opt-out: `.isolated_mcp`. |
| **MCP Schemas** | `~/.gemini/antigravity/mcp/<server>/` | Schemas de ferramentas locais e specs de chamadas MCP gerados pela IDE. | Seguro compartilhar via symlink. Opt-out: `.isolated_mcp`. |
| **Auth Tokens (OAuth / IA)** | Raiz de `~/.gemini/` (`jetski-standalone-oauth-token`, `installation_id`) + Keyrings/Keychains do SO | Tokens de autenticação de sessão e login de IA. | **ESTRITAMENTE ISOLADO.** Fica fora de `config/`. Nunca compartilhar nem exportar em backups. |
| **Conversas, Chats e Brains** | `~/.gemini/antigravity/` (`conversations/*.db`, `brain/<uuid>/*`, `annotations/*.pbtxt`) | Histórico de chats, planos, logs de passos e memórias das sessões de agentes. | Manipulado via `multigravity ai export/sync/import` com sanitização automática de tokens. |
| **Configurações UI da IDE** | `$MULTIGRAVITY_HOME/<perfil>/User/settings.json` | Tema, fontes, atalhos e preferências de editor do perfil. | Específico de cada perfil. |
| **Dev Auth (Git & GitHub CLI)** | `~/.config/gh/` (ou `%APPDATA%\GitHub CLI`), `~/.git-credentials`, `~/.gitconfig`, `~/.ssh` | Identidade dev, chaves SSH, configs Git e sessão CLI do GitHub. | Compartilhado por symlink para usabilidade dev; opt-out granular via `.isolated_gh` ou `.isolated_dotfiles`. |

---

## 9. Proibição de Manipulação Global de Keyring / Credential Manager

- **Muro de Chesterton:** Ferramentas comunitárias simplificadas (como `blugthek/Multigravity`) alternam contas sobrescrevendo dinamicamente a entrada global do Windows Credential Manager (`gemini:antigravity` via `advapi32.dll`) ou do Keychain antes de cada chamada e restaurando em seguida.
- **Por que é Estritamente Proibido no Multigravity:**
  1. **Inviabilidade Concorrente:** O keyring do SO é um recurso global compartilhado por usuário. Múltiplos agentes ou janelas rodando simultaneamente geram condições de corrida incontroláveis, corrompendo as credenciais ativas.
  2. **Risco Crítico de Perda de Sessão:** Caso o processo sofra crash, interrupção forçada (`kill -9`, falta de energia) ou timeout antes do teardown, o perfil primário do usuário perde sua credencial original e fica deslogado ou apontando para uma conta secundária.
  3. **Isolamento de Processo é Superior:** O `multigravity-cli` isola perfis via `--user-data-dir`, `--extensions-dir` e ambientes isolados de `$HOME` / `%USERPROFILE%`. Cada perfil mantém sua própria sessão independente, permitindo N instâncias da IDE ou agentes headless rodando simultaneamente sem qualquer sobreposição.
- **Cofre de arquivo do perfil (`multigravity login`):** A autenticação headless grava o refresh token apenas em `<perfil>/.gemini/antigravity-cli/antigravity-oauth-token` e na cópia `<perfil>/.gemini/jetski-standalone-oauth-token` (modo `0600`). O comando não chama CredWrite, Keychain nem Secret Service do host. Quando esse arquivo existe, o ambiente isolado do perfil exporta `GEMINI_FORCE_FILE_STORAGE=true` para o `agy` ler o cofre do perfil.
- **`multigravity exec` não é um motor novo:** O fan-out concorrente só chama o runner headless já existente (`RunAgentPrompt`) por perfil, com pool de workers. Não alterna o keyring do host, não provisiona worktree e não fala com o gateway. Despacho com worktree/PTY continua em `dispatch`; completions continuam no gateway.

