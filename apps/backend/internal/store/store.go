package store

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
}

type Project struct {
	ID            uuid.UUID `json:"id"`
	Title         string    `json:"title"`
	InitialPrompt string    `json:"initialPrompt"`
	UserID        uuid.UUID `json:"userId"`
	SandboxID     *string   `json:"sandboxId"`
	SandboxURL    *string   `json:"sandboxUrl"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type Conversation struct {
	ID        uuid.UUID `json:"id"`
	ProjectID uuid.UUID `json:"projectId"`
	From      string    `json:"from"`
	Type      string    `json:"type"`
	Contents  string    `json:"contents"`
	ToolCall  *string   `json:"toolCall,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type ProjectFile struct {
	ID        uuid.UUID `json:"id"`
	ProjectID uuid.UUID `json:"projectId"`
	Path      string    `json:"path"`
	Content   string    `json:"content,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type FileNode struct {
	Name     string     `json:"name"`
	Path     string     `json:"path"`
	Type     string     `json:"type"`
	Children []FileNode `json:"children,omitempty"`
}

//go:embed migrations/*.sql
var migrationFiles embed.FS

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect database: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

func (s *Store) Migrate(ctx context.Context) error {
	entries, err := fs.Glob(migrationFiles, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(entries)
	for _, name := range entries {
		sql, err := migrationFiles.ReadFile(name)
		if err != nil {
			return err
		}
		if _, err := s.pool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
	}
	return nil
}

func (s *Store) CreateUser(ctx context.Context, email, passwordHash string) (User, error) {
	user := User{ID: uuid.New(), Email: strings.ToLower(strings.TrimSpace(email)), PasswordHash: passwordHash}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO users (id, email, password_hash) VALUES ($1,$2,$3) RETURNING id,email,password_hash`,
		user.ID, user.Email, user.PasswordHash,
	).Scan(&user.ID, &user.Email, &user.PasswordHash)
	return user, err
}

func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	var user User
	err := s.pool.QueryRow(ctx,
		`SELECT id,email,password_hash FROM users WHERE email=$1`,
		strings.ToLower(strings.TrimSpace(email)),
	).Scan(&user.ID, &user.Email, &user.PasswordHash)
	return user, err
}

func (s *Store) UserByID(ctx context.Context, id uuid.UUID) (User, error) {
	var user User
	err := s.pool.QueryRow(ctx, `SELECT id,email,password_hash FROM users WHERE id=$1`, id).
		Scan(&user.ID, &user.Email, &user.PasswordHash)
	return user, err
}

func (s *Store) CreateProject(ctx context.Context, userID uuid.UUID, title, prompt string) (Project, error) {
	var p Project
	err := s.pool.QueryRow(ctx, `
		INSERT INTO projects (id,title,initial_prompt,user_id)
		VALUES ($1,$2,$3,$4)
		RETURNING id,title,initial_prompt,user_id,sandbox_id,sandbox_url,created_at,updated_at`,
		uuid.New(), title, prompt, userID,
	).Scan(&p.ID, &p.Title, &p.InitialPrompt, &p.UserID, &p.SandboxID, &p.SandboxURL, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func (s *Store) Projects(ctx context.Context, userID uuid.UUID) ([]Project, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id,title,initial_prompt,user_id,sandbox_id,sandbox_url,created_at,updated_at
		FROM projects WHERE user_id=$1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var projects []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Title, &p.InitialPrompt, &p.UserID, &p.SandboxID, &p.SandboxURL, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (s *Store) Project(ctx context.Context, userID, projectID uuid.UUID) (Project, error) {
	var p Project
	err := s.pool.QueryRow(ctx, `
		SELECT id,title,initial_prompt,user_id,sandbox_id,sandbox_url,created_at,updated_at
		FROM projects WHERE id=$1 AND user_id=$2`, projectID, userID).
		Scan(&p.ID, &p.Title, &p.InitialPrompt, &p.UserID, &p.SandboxID, &p.SandboxURL, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func (s *Store) SaveSandbox(ctx context.Context, projectID uuid.UUID, sandboxID, sandboxURL string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE projects SET sandbox_id=$2,sandbox_url=$3,updated_at=NOW()
		WHERE id=$1`, projectID, sandboxID, sandboxURL)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) ClearSandbox(ctx context.Context, projectID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE projects SET sandbox_id=NULL,sandbox_url=NULL,updated_at=NOW()
		WHERE id=$1`, projectID)
	return err
}

func (s *Store) AddConversation(ctx context.Context, projectID uuid.UUID, from, messageType, contents string, toolCall *string) (Conversation, error) {
	var c Conversation
	err := s.pool.QueryRow(ctx, `
		INSERT INTO conversations (id,project_id,sender,message_type,contents,tool_call)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id,project_id,sender,message_type,contents,tool_call,created_at,updated_at`,
		uuid.New(), projectID, from, messageType, contents, toolCall,
	).Scan(&c.ID, &c.ProjectID, &c.From, &c.Type, &c.Contents, &c.ToolCall, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

func (s *Store) UpsertFile(ctx context.Context, projectID uuid.UUID, path, content string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO project_files (id,project_id,path,content)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (project_id,path) DO UPDATE SET content=EXCLUDED.content,updated_at=NOW()`,
		uuid.New(), projectID, path, content)
	return err
}

func (s *Store) DeleteFile(ctx context.Context, projectID uuid.UUID, path string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM project_files WHERE project_id=$1 AND path=$2`, projectID, path)
	return err
}

func (s *Store) RenameFile(ctx context.Context, projectID uuid.UUID, oldPath, newPath string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE project_files SET path=$3,updated_at=NOW() WHERE project_id=$1 AND path=$2`,
		projectID, oldPath, newPath)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) File(ctx context.Context, projectID uuid.UUID, path string) (ProjectFile, error) {
	var f ProjectFile
	err := s.pool.QueryRow(ctx, `
		SELECT id,project_id,path,content,created_at,updated_at
		FROM project_files WHERE project_id=$1 AND path=$2`, projectID, path).
		Scan(&f.ID, &f.ProjectID, &f.Path, &f.Content, &f.CreatedAt, &f.UpdatedAt)
	return f, err
}

func (s *Store) Files(ctx context.Context, projectID uuid.UUID) ([]ProjectFile, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id,project_id,path,content,created_at,updated_at
		FROM project_files WHERE project_id=$1 ORDER BY path`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var files []ProjectFile
	for rows.Next() {
		var f ProjectFile
		if err := rows.Scan(&f.ID, &f.ProjectID, &f.Path, &f.Content, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

func BuildFileTree(files []ProjectFile) []FileNode {
	type trieNode struct {
		name     string
		path     string
		kind     string
		children map[string]*trieNode
	}
	root := map[string]*trieNode{}
	for _, file := range files {
		parts := strings.Split(strings.TrimPrefix(file.Path, "/"), "/")
		if len(parts) == 0 {
			continue
		}
		current := root
		var prefix []string
		for i, part := range parts {
			if part == "" {
				continue
			}
			prefix = append(prefix, part)
			node, ok := current[part]
			if !ok {
				kind := "folder"
				if i == len(parts)-1 {
					kind = "file"
				}
				node = &trieNode{
					name:     part,
					path:     strings.Join(prefix, "/"),
					kind:     kind,
					children: map[string]*trieNode{},
				}
				current[part] = node
			}
			if node.kind == "folder" {
				current = node.children
			}
		}
	}
	var convert func(map[string]*trieNode) []FileNode
	convert = func(nodes map[string]*trieNode) []FileNode {
		keys := make([]string, 0, len(nodes))
		for key := range nodes {
			keys = append(keys, key)
		}
		sort.Slice(keys, func(i, j int) bool {
			left, right := nodes[keys[i]], nodes[keys[j]]
			if left.kind != right.kind {
				return left.kind == "folder"
			}
			return left.name < right.name
		})
		result := make([]FileNode, 0, len(keys))
		for _, key := range keys {
			node := nodes[key]
			result = append(result, FileNode{
				Name:     node.name,
				Path:     node.path,
				Type:     node.kind,
				Children: convert(node.children),
			})
		}
		return result
	}
	return convert(root)
}

func IsNotFound(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
