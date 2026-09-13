# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.4.0] - 2026-09-12

### Added
- **Interactive TUI:** Quick-select menu when running `multigravity` without arguments in an interactive terminal, displaying live status indicators (`● running` / `○ idle`).
- **Antigravity 2.0 (`agy`) Support:** Native binary detection for `agy` alongside `antigravity`, with expanded search across portable paths (`~/apps/antigravity/antigravity`, `/opt/`, and Scoop).
- **Profile Window Theming:** Distinctive window and workbench accent colors per profile via `--color` and `multigravity color <name> [color]`.
- **Graceful Lifecycle Controls:** `stop` and `restart` commands with SQLite flush safety and active concurrency locks against accidental deletion or renaming.
- **Cache Cleaning & Lean Backups:** `multigravity clean` command to free disk space from volatile Electron caches, plus automated cache exclusion during `export`.
- **Granular AI Session Migration:** `multigravity ai (list, export, import)` for non-destructive chat migration with strict OAuth token/credential sanitization.
- **Developer Dotfiles Integration:** Automatic linking of host `.gitconfig` and `~/.ssh` into isolated profiles (with `--isolated-dotfiles` opt-out).
- **CLI Version Command:** Dedicated `version`, `-v`, and `--version` flags with autocompletion support for Bash, Zsh, and PowerShell.
- **Documentation:** Complete Brazilian Portuguese documentation in `README.pt-br.md` and MIT licensing clarification for fork contributions.
