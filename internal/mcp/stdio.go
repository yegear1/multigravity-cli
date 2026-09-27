package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
)

// ServeStdio starts an MCP JSON-RPC server reading requests from in and writing responses to out.
// It complies strictly with the MCP stdio transport standard (newline-delimited JSON-RPC messages).
func (s *Server) ServeStdio(ctx context.Context, in io.Reader, out io.Writer) error {
	if in == nil {
		in = os.Stdin
	}
	if out == nil {
		out = os.Stdout
	}

	scanner := bufio.NewScanner(in)
	// Allow large messages (e.g. prompts, diffs, tool responses up to 16MB)
	const maxScanCapacity = 16 * 1024 * 1024
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, maxScanCapacity)

	var writeMu sync.Mutex

	writeResponse := func(resp *JSONRPCResponse) error {
		if resp == nil {
			return nil
		}
		data, err := json.Marshal(resp)
		if err != nil {
			return err
		}
		data = append(data, '\n')

		writeMu.Lock()
		defer writeMu.Unlock()
		_, err = out.Write(data)
		return err
	}

	lines := make(chan []byte)
	errCh := make(chan error, 1)

	go func() {
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			// Copy line bytes because scanner buffer is reused
			lineCopy := make([]byte, len(line))
			copy(lineCopy, line)
			lines <- lineCopy
		}
		if err := scanner.Err(); err != nil && err != io.EOF {
			errCh <- err
		} else {
			close(lines)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case err := <-errCh:
			return err

		case line, ok := <-lines:
			if !ok {
				// Clean EOF from client
				return nil
			}

			var req JSONRPCRequest
			if err := json.Unmarshal(line, &req); err != nil {
				_ = writeResponse(&JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      nil,
					Error: &JSONRPCError{
						Code:    CodeParseError,
						Message: fmt.Sprintf("Parse error: %v", err),
					},
				})
				continue
			}

			resp := s.HandleRequest(ctx, &req)
			if resp != nil {
				if err := writeResponse(resp); err != nil {
					return fmt.Errorf("failed to write response: %w", err)
				}
			}
		}
	}
}
