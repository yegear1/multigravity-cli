package dispatch

import (
	"strings"
	"testing"
)

func TestExtractASTOutlineGo(t *testing.T) {
	src := `package sample

import (
	"fmt"
	"strings"
)

type Config struct {
	Name  string
	Limit int
}

type Runner interface {
	Run(cmd string) (string, error)
}

func NewConfig(name string) *Config {
	return &Config{Name: name, Limit: 100}
}

func (c *Config) Execute(input string) error {
	fmt.Println("doing work...", input)
	for i := 0; i < c.Limit; i++ {
		strings.TrimSpace(input)
	}
	return nil
}
`
	outline, err := ExtractASTOutline("sample.go", []byte(src))
	if err != nil {
		t.Fatalf("unexpected error extracting Go AST outline: %v", err)
	}

	// Must contain package, imports, struct, interface, and func signatures
	expectedSnippets := []string{
		"package sample",
		"import (",
		"type Config struct",
		"type Runner interface",
		"func NewConfig(name string) *Config",
		"func (c *Config) Execute(input string) error",
	}

	for _, snippet := range expectedSnippets {
		if !strings.Contains(outline, snippet) {
			t.Errorf("expected outline to contain %q, got:\n%s", snippet, outline)
		}
	}

	// Must NOT contain function body internals
	forbiddenSnippets := []string{
		"doing work...",
		"strings.TrimSpace",
		"for i := 0",
	}

	for _, snippet := range forbiddenSnippets {
		if strings.Contains(outline, snippet) {
			t.Errorf("outline should not contain body statement %q, got:\n%s", snippet, outline)
		}
	}
}

func TestExtractASTOutlineTypeScript(t *testing.T) {
	src := `import { useState } from 'react';

export interface User {
  id: string;
  name: string;
}

export type UserRole = 'admin' | 'member';

export class UserService {
  constructor(private readonly api: string) {}

  public async fetchUser(id: string): Promise<User> {
    const res = await fetch(this.api + '/' + id);
    return res.json();
  }
}

export function formatUser(u: User): string {
  return u.name + " (" + u.id + ")";
}
`
	outline, err := ExtractASTOutline("service.ts", []byte(src))
	if err != nil {
		t.Fatalf("unexpected error extracting TS outline: %v", err)
	}

	if !strings.Contains(outline, "export interface User") {
		t.Errorf("expected outline to contain 'export interface User'")
	}
	if !strings.Contains(outline, "export type UserRole") {
		t.Errorf("expected outline to contain 'export type UserRole'")
	}
	if !strings.Contains(outline, "export class UserService") {
		t.Errorf("expected outline to contain 'export class UserService'")
	}
	if !strings.Contains(outline, "export function formatUser") {
		t.Errorf("expected outline to contain 'export function formatUser'")
	}
}

func TestExtractASTOutlineUnsupportedOrInvalid(t *testing.T) {
	// Unsupported extension
	_, err := ExtractASTOutline("data.json", []byte(`{"key": "value"}`))
	if err == nil {
		t.Errorf("expected error for unsupported extension .json, got nil")
	}

	// Invalid Go syntax
	_, err = ExtractASTOutline("bad.go", []byte(`package 123 invalid syntax {{{`))
	if err == nil {
		t.Errorf("expected parse error for invalid Go syntax, got nil")
	}
}
