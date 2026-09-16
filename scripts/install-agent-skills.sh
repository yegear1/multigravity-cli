#!/usr/bin/env bash
# ==============================================================================
# Multigravity Agent Skills Installer & Synchronizer
# ==============================================================================
# Creates atomic symbolic links of canonical skills in this repository
# into global AI assistant discovery folders (~/.gemini/config/skills, ~/.cursor/skills).
# ==============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
SKILLS_DIR="${ROOT_DIR}/skills"

DRY_RUN=false
TARGET_DIRS=()

usage() {
  cat <<HELP_EOF
Usage: $(basename "$0") [OPTIONS]

Options:
  --cursor            Install symlinks in ~/.cursor/skills
  --antigravity       Install symlinks in ~/.gemini/config/skills and ~/.gemini/antigravity/skills
  --all               Install across all detected AI client directories
  --target DIR        Specify a custom destination directory
  -d, --dry-run       Show actions without modifying disk
  -l, --list          List canonical skills available in this repository
  -h, --help          Show this help message and exit
HELP_EOF
  exit 0
}

list_skills() {
  echo "Canonical Skills in ${SKILLS_DIR}:"
  for skill_path in "${SKILLS_DIR}"/*; do
    if [[ -d "${skill_path}" && -f "${skill_path}/SKILL.md" ]]; then
      local s_name
      s_name=$(basename "${skill_path}")
      echo "  • ${s_name}"
    fi
  done
  exit 0
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --cursor)
      TARGET_DIRS+=("${HOME}/.cursor/skills")
      shift
      ;;
    --antigravity)
      TARGET_DIRS+=("${HOME}/.gemini/config/skills")
      TARGET_DIRS+=("${HOME}/.gemini/antigravity/skills")
      shift
      ;;
    --all)
      TARGET_DIRS+=("${HOME}/.cursor/skills")
      TARGET_DIRS+=("${HOME}/.gemini/config/skills")
      TARGET_DIRS+=("${HOME}/.gemini/antigravity/skills")
      shift
      ;;
    --target)
      if [[ $# -gt 1 ]]; then
        TARGET_DIRS+=("$2")
        shift 2
      else
        echo "Error: --target requires a directory path." >&2
        exit 1
      fi
      ;;
    -d|--dry-run)
      DRY_RUN=true
      shift
      ;;
    -l|--list)
      list_skills
      ;;
    -h|--help)
      usage
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage
      ;;
  esac
done

if [[ ${#TARGET_DIRS[@]} -eq 0 ]]; then
  TARGET_DIRS+=("${HOME}/.cursor/skills")
  TARGET_DIRS+=("${HOME}/.gemini/config/skills")
fi

if [[ ! -d "${SKILLS_DIR}" ]]; then
  echo "Error: Skills directory not found at ${SKILLS_DIR}" >&2
  exit 1
fi

echo "================================================================================"
echo "🔗 [Multigravity Skills Installer] Syncing skills with AI environments..."
echo "================================================================================"

UNIQUE_TARGETS=()
while IFS= read -r dir; do
  [[ -n "$dir" ]] && UNIQUE_TARGETS+=("$dir")
done < <(printf "%s\n" "${TARGET_DIRS[@]}" | sort -u)

for target in "${UNIQUE_TARGETS[@]}"; do
  echo -e "\n🎯 Target: ${target}"

  if [[ "$DRY_RUN" == false ]]; then
    mkdir -p "${target}"
  fi

  for skill_path in "${SKILLS_DIR}"/*; do
    if [[ -d "${skill_path}" && -f "${skill_path}/SKILL.md" ]]; then
      skill_name=$(basename "${skill_path}")
      link_dest="${target}/${skill_name}"

      if [[ "$DRY_RUN" == true ]]; then
        echo "   [DRY-RUN] ln -sfn \"${skill_path}\" \"${link_dest}\""
      else
        ln -sfn "${skill_path}" "${link_dest}"
        echo "   Link created/updated: ${skill_name} -> ${skill_path}"
      fi
    fi
  done
done

echo -e "\n================================================================================"
if [[ "$DRY_RUN" == true ]]; then
  echo "Dry-run complete. No changes made."
else
  echo "All canonical skills successfully linked!"
fi
echo "================================================================================"
