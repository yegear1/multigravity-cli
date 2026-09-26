package dispatch

import (
	"regexp"
	"strconv"
	"strings"
)

var hunkHeaderRegex = regexp.MustCompile(`^@@\s+-(\d+)(?:,(\d+))?\s+\+(\d+)(?:,(\d+))?\s+@@(.*)$`)

// ParseUnifiedDiff parses a standard unified git diff output into a structured model.
func ParseUnifiedDiff(raw string) *StructuredDiff {
	result := &StructuredDiff{
		Files: []DiffFile{},
		Raw:   raw,
	}

	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return result
	}

	lines := strings.Split(raw, "\n")
	var currentFile *DiffFile
	var currentHunk *DiffHunk
	var oldLineNo, newLineNo int

	flushHunk := func() {
		if currentHunk != nil && currentFile != nil {
			currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
			currentHunk = nil
		}
	}

	flushFile := func() {
		flushHunk()
		if currentFile != nil {
			result.Files = append(result.Files, *currentFile)
			result.Summary.FilesChanged++
			result.Summary.Additions += currentFile.Additions
			result.Summary.Deletions += currentFile.Deletions
			currentFile = nil
		}
	}

	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r")

		// New file diff header: diff --git a/path b/path
		if strings.HasPrefix(line, "diff --git ") {
			flushFile()

			parts := strings.Split(line, " ")
			oldPath, newPath := "", ""
			if len(parts) >= 4 {
				oldPath = strings.TrimPrefix(parts[2], "a/")
				newPath = strings.TrimPrefix(parts[3], "b/")
			}

			currentFile = &DiffFile{
				OldPath: oldPath,
				NewPath: newPath,
				Status:  DiffFileModified,
				Hunks:   []DiffHunk{},
			}
			continue
		}

		if currentFile == nil {
			continue
		}

		// Extended headers
		if strings.HasPrefix(line, "new file mode ") {
			currentFile.Status = DiffFileAdded
			continue
		}
		if strings.HasPrefix(line, "deleted file mode ") {
			currentFile.Status = DiffFileDeleted
			continue
		}
		if strings.HasPrefix(line, "rename from ") {
			currentFile.OldPath = strings.TrimPrefix(line, "rename from ")
			currentFile.Status = DiffFileRenamed
			continue
		}
		if strings.HasPrefix(line, "rename to ") {
			currentFile.NewPath = strings.TrimPrefix(line, "rename to ")
			currentFile.Status = DiffFileRenamed
			continue
		}
		if strings.HasPrefix(line, "Binary files ") && strings.HasSuffix(line, "differ") {
			currentFile.Binary = true
			continue
		}

		// File marker lines: --- a/... and +++ b/...
		if strings.HasPrefix(line, "--- ") {
			p := strings.TrimPrefix(line, "--- ")
			p = strings.TrimSpace(p)
			if p == "/dev/null" {
				currentFile.Status = DiffFileAdded
				currentFile.OldPath = ""
			} else {
				currentFile.OldPath = strings.TrimPrefix(p, "a/")
			}
			continue
		}
		if strings.HasPrefix(line, "+++ ") {
			p := strings.TrimPrefix(line, "+++ ")
			p = strings.TrimSpace(p)
			if p == "/dev/null" {
				currentFile.Status = DiffFileDeleted
				currentFile.NewPath = ""
			} else {
				currentFile.NewPath = strings.TrimPrefix(p, "b/")
			}
			continue
		}

		// Hunk header: @@ -oldStart,oldLines +newStart,newLines @@
		if match := hunkHeaderRegex.FindStringSubmatch(line); match != nil {
			flushHunk()

			oStart, _ := strconv.Atoi(match[1])
			oLines := 1
			if match[2] != "" {
				oLines, _ = strconv.Atoi(match[2])
			}

			nStart, _ := strconv.Atoi(match[3])
			nLines := 1
			if match[4] != "" {
				nLines, _ = strconv.Atoi(match[4])
			}

			headerNote := strings.TrimSpace(match[5])

			currentHunk = &DiffHunk{
				Header:   headerNote,
				OldStart: oStart,
				OldLines: oLines,
				NewStart: nStart,
				NewLines: nLines,
				Lines:    []DiffLine{},
			}
			oldLineNo = oStart
			newLineNo = nStart
			continue
		}

		// Lines inside a hunk
		if currentHunk != nil {
			if strings.HasPrefix(line, "+") {
				currentFile.Additions++
				currentHunk.Lines = append(currentHunk.Lines, DiffLine{
					Type:      DiffLineAddition,
					Content:   line[1:],
					NewLineNo: newLineNo,
				})
				newLineNo++
			} else if strings.HasPrefix(line, "-") {
				currentFile.Deletions++
				currentHunk.Lines = append(currentHunk.Lines, DiffLine{
					Type:      DiffLineDeletion,
					Content:   line[1:],
					OldLineNo: oldLineNo,
				})
				oldLineNo++
			} else if strings.HasPrefix(line, " ") {
				currentHunk.Lines = append(currentHunk.Lines, DiffLine{
					Type:      DiffLineContext,
					Content:   line[1:],
					OldLineNo: oldLineNo,
					NewLineNo: newLineNo,
				})
				oldLineNo++
				newLineNo++
			} else if strings.HasPrefix(line, `\ No newline at end of file`) {
				currentHunk.Lines = append(currentHunk.Lines, DiffLine{
					Type:    DiffLineHeader,
					Content: line,
				})
			}
		}
	}

	flushFile()

	return result
}
