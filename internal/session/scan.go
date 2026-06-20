// Package session scans Claude Code transcript files and extracts lightweight
// session metadata for display in the TUI.
//
// Transcripts live at ${CLAUDE_CONFIG_DIR:-~/.claude}/projects/<proj>/<id>.jsonl.
// Each line is one JSON record. We only parse the few fields we need (type +
// payload) rather than fully decoding every line — transcripts reach multiple MB
// with individual lines up to ~1.8MB, so cheap-and-streaming matters.
package session

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Session is the display-ready metadata for one Claude Code conversation.
type Session struct {
	ID         string    // session-id (jsonl filename without extension)
	Path       string    // absolute path to the .jsonl file
	Title      string    // ai-title if present, else first human prompt, else "(untitled)"
	LastPrompt string    // most recent user prompt (preview)
	CWD        string    // working directory the session ran in (from jsonl, never derived from dir name)
	GitBranch  string    // git branch at session start
	ModTime    time.Time // file mtime, used for sort order
}

// Source lists Claude Code sessions. FilesystemSource is the current
// implementation; the interface keeps the UI independent from where session
// metadata comes from.
type Source interface {
	List() ([]Session, error)
}

type fileFingerprint struct {
	modTime time.Time
	size    int64
}

type cachedSession struct {
	session     Session
	fingerprint fileFingerprint
}

// FilesystemSessionSource lists sessions from the Claude Code projects
// directory and reuses parsed metadata for unchanged transcript files.
type FilesystemSessionSource struct {
	projectsDir string
	cache       map[string]cachedSession
	parse       func(string) (Session, error)
}

// NewFilesystemSessionSource creates a cached session source backed by the
// Claude Code projects directory.
func NewFilesystemSessionSource(projectsDir string) *FilesystemSessionSource {
	if abs, err := filepath.Abs(projectsDir); err == nil {
		projectsDir = abs
	}
	return &FilesystemSessionSource{
		projectsDir: projectsDir,
		cache:       make(map[string]cachedSession),
		parse:       parseFile,
	}
}

// rawRecord is a partial view of a transcript line. Only fields we consume are
// declared; everything else is ignored by encoding/json.
type rawRecord struct {
	Type       string          `json:"type"`
	AITitle    string          `json:"aiTitle"`
	LastPrompt string          `json:"lastPrompt"`
	CWD        string          `json:"cwd"`
	GitBranch  string          `json:"gitBranch"`
	Message    json.RawMessage `json:"message"` // decoded lazily only for fallback titles
}

// userMessage is the shape of a "user" record's message field when we need a
// fallback title (no ai-title present). content is either a string (real human
// prompt) or an array (tool results — skipped).
type userMessage struct {
	Content json.RawMessage `json:"content"`
}

// ProjectsDir returns the Claude Code projects directory, honoring
// CLAUDE_CONFIG_DIR.
func ProjectsDir() string {
	base := os.Getenv("CLAUDE_CONFIG_DIR")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".claude")
	}
	return filepath.Join(base, "projects")
}

// Scan walks the projects directory and returns all top-level sessions, newest
// first. It deliberately skips <proj>/<id>/subagents/*.jsonl — those are
// sub-agent transcripts, not user-facing sessions.
func Scan(projectsDir string) ([]Session, error) {
	return NewFilesystemSessionSource(projectsDir).List()
}

// List walks the projects directory and returns all top-level sessions, newest
// first. Unchanged transcript files reuse cached metadata; new or changed files
// are parsed with the same rules as Scan.
func (s *FilesystemSessionSource) List() ([]Session, error) {
	if s.parse == nil {
		s.parse = parseFile
	}
	if s.cache == nil {
		s.cache = make(map[string]cachedSession)
	}

	entries, err := os.ReadDir(s.projectsDir)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var sessions []Session
	for _, projEntry := range entries {
		if !projEntry.IsDir() {
			continue
		}
		projPath := filepath.Join(s.projectsDir, projEntry.Name())
		files, err := os.ReadDir(projPath)
		if err != nil {
			continue // unreadable project dir — skip, don't fail the whole scan
		}
		for _, f := range files {
			// Only top-level *.jsonl. Subdirectories (e.g. <id>/subagents/) are
			// skipped because f.IsDir() is true for them.
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".jsonl") {
				continue
			}
			info, err := f.Info()
			if err != nil {
				continue
			}
			full := filepath.Join(projPath, f.Name())
			seen[full] = true
			fp := fileFingerprint{modTime: info.ModTime(), size: info.Size()}
			if cached, ok := s.cache[full]; ok && cached.fingerprint == fp {
				sessions = append(sessions, cached.session)
				continue
			}

			parsed, err := s.parse(full)
			if err != nil {
				// A single bad file shouldn't sink the list. If we have a previous
				// parse result, keep it to avoid flicker during transient writes.
				if cached, ok := s.cache[full]; ok {
					sessions = append(sessions, cached.session)
				}
				continue
			}
			parsed.ModTime = fp.modTime
			s.cache[full] = cachedSession{session: parsed, fingerprint: fp}
			sessions = append(sessions, parsed)
		}
	}

	for path := range s.cache {
		if !seen[path] {
			delete(s.cache, path)
		}
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].ModTime.After(sessions[j].ModTime)
	})
	return sessions, nil
}

// parseFile streams one transcript and extracts metadata.
//
// Title/last-prompt rules: ai-title and last-prompt records are regenerated
// through a session, so the LAST occurrence is the current one. cwd is taken
// from the FIRST record that carries it (the session's starting directory).
func parseFile(path string) (Session, error) {
	s := Session{
		ID:   strings.TrimSuffix(filepath.Base(path), ".jsonl"),
		Path: path,
	}

	if fi, err := os.Stat(path); err == nil {
		s.ModTime = fi.ModTime()
	}

	f, err := os.Open(path)
	if err != nil {
		return s, err
	}
	defer f.Close()

	// bufio.Reader (not Scanner) because individual lines can exceed Scanner's
	// 64KB token limit — transcripts have lines up to ~1.8MB.
	reader := bufio.NewReader(f)

	var firstHumanPrompt string
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			var rec rawRecord
			if json.Unmarshal(line, &rec) == nil {
				switch rec.Type {
				case "ai-title":
					if rec.AITitle != "" {
						s.Title = rec.AITitle // keep overwriting: last wins
					}
				case "last-prompt":
					if rec.LastPrompt != "" {
						s.LastPrompt = rec.LastPrompt // last wins
					}
				case "user":
					if s.CWD == "" && rec.CWD != "" {
						s.CWD = rec.CWD // first wins
					}
					if s.GitBranch == "" && rec.GitBranch != "" {
						s.GitBranch = rec.GitBranch
					}
					if firstHumanPrompt == "" && len(rec.Message) > 0 {
						if p := extractHumanPrompt(rec.Message); p != "" {
							firstHumanPrompt = p
						}
					}
				default:
					// Other record types may still carry cwd (e.g. attachment).
					if s.CWD == "" && rec.CWD != "" {
						s.CWD = rec.CWD
					}
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
	}

	// Title fallback chain: ai-title → first human prompt → untitled.
	if s.Title == "" {
		if firstHumanPrompt != "" {
			s.Title = firstHumanPrompt
		} else {
			s.Title = "(untitled)"
		}
	}
	if s.LastPrompt == "" {
		s.LastPrompt = firstHumanPrompt
	}
	return s, nil
}

// extractHumanPrompt returns the text of a user message only when it is a real
// human prompt: content must be a JSON string (array content = tool results),
// and must not be a local-command echo / slash / bang command.
func extractHumanPrompt(message json.RawMessage) string {
	var um userMessage
	if json.Unmarshal(message, &um) != nil {
		return ""
	}
	// content must be a plain string; array content is tool_result, skip.
	var content string
	if json.Unmarshal(um.Content, &content) != nil {
		return ""
	}
	content = strings.TrimLeft(content, "›\t \n")
	if content == "" {
		return ""
	}
	// Skip local-command echoes, XML-ish tags, and slash/bang commands.
	switch content[0] {
	case '<', '/', '!':
		return ""
	}
	// Collapse whitespace for a clean one-line preview.
	content = strings.Join(strings.Fields(content), " ")
	return content
}
