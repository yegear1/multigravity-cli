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
- **Atalhos:** Criados em `$HOME/Applications/Multigravity <nome>.app` contendo estrutura padrão de bundle macOS com `Contents/MacOS/run`, `Info.plist` e `icon.icns`.

### Linux
- **XDG Base Directory:** `XDG_CONFIG_HOME`, `XDG_CACHE_HOME`, `XDG_DATA_HOME` e `XDG_STATE_HOME` devem apontar para as respectivas subpastas dentro do perfil para não poluir o sistema.
- **Atalhos:** Gerados em `~/.local/share/applications/multigravity-<nome>.desktop` apontando para o wrapper script em `~/.local/share/multigravity/launchers/<nome>.sh`.

### Windows
- **Variáveis de Usuário:** `$env:USERPROFILE`, `$env:APPDATA` e `$env:LOCALAPPDATA` devem ser apontados para as subpastas correspondentes dentro de `$PROFILE_DIR`.
- **Atalhos:** `.lnk` gerados em `$env:APPDATA\Microsoft\Windows\Start Menu\Programs\`.
