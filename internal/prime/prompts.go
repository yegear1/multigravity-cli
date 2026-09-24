package prime

import (
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var DefaultPrompts = []string{
	// English
	"ping",
	"Hello! Quick status check.",
	"Hi, are you ready?",
	"Good morning! Ready for today's tasks?",
	"Quick ping test, thanks!",
	"Hi there! How is everything running?",
	"Ready to assist today?",
	"Hello, just checking in.",
	"Quick connectivity check.",
	"Hi! All systems operational?",
	"Hello! Quick sanity check.",
	"Hi, checking in for a new session.",
	"Good day! Ready when you are.",
	"Quick check: system online?",
	"Hello! Confirming connection.",
	"Hi there, ready for coding?",
	"Ping test, please acknowledge.",
	"Good morning! Everything running smoothly?",
	"Hi! Quick hello before getting started.",
	"Testing connection, thanks!",
	// Portuguese
	"Olá! Tudo bem por aí?",
	"Oi! Teste rápido de status.",
	"Bom dia! Pronto para os trabalhos de hoje?",
	"Olá, tudo funcionando certinho?",
	"Oi, checagem rápida de conexão.",
	"Pronto para ajudar hoje?",
	"Olá! Sistema operacional?",
	"Checagem rápida de status, valeu!",
	"Oi, apenas confirmando conexão.",
	"Olá! Pronto para começar?",
	"Bom dia! Tudo certo por aqui?",
	"Olá, teste rápido de comunicação.",
	"Oi! Sistema ativo?",
	"Checagem de rotina, tudo ok?",
	"Olá, pronto para mais uma sessão?",
	"Oi, confirmando disponibilidade.",
	"Teste de ping, obrigado!",
	"Olá, tudo tranquilo?",
	"Bom dia, pronto para codar?",
	"Oi, verificação rápida do assistente.",
}

type PromptsFile struct {
	Prompts []string `json:"prompts"`
}

func GetPromptsFilePath() string {
	userHome, _ := os.UserHomeDir()
	if realHome := os.Getenv("REAL_HOME"); realHome != "" {
		userHome = realHome
	}

	if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData != "" {
			return filepath.Join(localAppData, "multigravity", "prompts.json")
		}
	}
	return filepath.Join(userHome, ".local", "share", "multigravity", "prompts.json")
}

func LoadPrompts() []string {
	path := GetPromptsFilePath()
	if data, err := os.ReadFile(path); err == nil {
		var pf PromptsFile
		if err := json.Unmarshal(data, &pf); err == nil && len(pf.Prompts) > 0 {
			var clean []string
			for _, p := range pf.Prompts {
				trimmed := strings.TrimSpace(p)
				if trimmed != "" {
					clean = append(clean, trimmed)
				}
			}
			if len(clean) > 0 {
				return clean
			}
		}
	}

	// Auto-seed file if missing or invalid
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	pf := PromptsFile{Prompts: DefaultPrompts}
	if b, err := json.MarshalIndent(pf, "", "  "); err == nil {
		_ = os.WriteFile(path, b, 0644)
	}

	return append([]string(nil), DefaultPrompts...)
}

func SelectRandomPrompt(promptPool []string, usedPrompts map[string]bool) string {
	if len(promptPool) == 0 {
		return "ping"
	}

	var available []string
	for _, p := range promptPool {
		if !usedPrompts[p] {
			available = append(available, p)
		}
	}

	if len(available) == 0 {
		available = promptPool
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return available[r.Intn(len(available))]
}
