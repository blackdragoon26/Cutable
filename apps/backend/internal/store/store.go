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
	Name         string
	Email        string
	PasswordHash string
	GoogleSub    string
}

type DemoUsage struct {
	Used         int  `json:"used"`
	Limit        int  `json:"limit"`
	Remaining    int  `json:"remaining"`
	RequiresKeys bool `json:"requiresKeys"`
}

type Project struct {
	ID            uuid.UUID           `json:"id"`
	Title         string              `json:"title"`
	InitialPrompt string              `json:"initialPrompt"`
	UserID        uuid.UUID           `json:"userId"`
	SandboxID     *string             `json:"sandboxId"`
	SandboxURL    *string             `json:"sandboxUrl"`
	Attachments   []ProjectAttachment `json:"attachments"`
	CreatedAt     time.Time           `json:"createdAt"`
	UpdatedAt     time.Time           `json:"updatedAt"`
}

type ProjectAttachmentInput struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	MimeType string `json:"mimeType"`
	Content  string `json:"content"`
	Size     int    `json:"size"`
}

type ProjectAttachment struct {
	ID        uuid.UUID `json:"id"`
	ProjectID uuid.UUID `json:"projectId"`
	Name      string    `json:"name"`
	Kind      string    `json:"kind"`
	MimeType  string    `json:"mimeType"`
	Content   string    `json:"content"`
	Size      int       `json:"size"`
	CreatedAt time.Time `json:"createdAt"`
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

func (s *Store) CreateUser(ctx context.Context, name, email, passwordHash string) (User, error) {
	user := User{ID: uuid.New(), Name: strings.TrimSpace(name), Email: strings.ToLower(strings.TrimSpace(email)), PasswordHash: passwordHash}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO users (id,name,email,password_hash) VALUES ($1,$2,$3,$4)
		 RETURNING id,name,email,COALESCE(password_hash,''),COALESCE(google_sub,'')`,
		user.ID, user.Name, user.Email, user.PasswordHash,
	).Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash, &user.GoogleSub)
	return user, err
}

func (s *Store) UserByEmail(ctx context.Context, email string) (User, error) {
	var user User
	err := s.pool.QueryRow(ctx,
		`SELECT id,name,email,COALESCE(password_hash,''),COALESCE(google_sub,'') FROM users WHERE email=$1`,
		strings.ToLower(strings.TrimSpace(email)),
	).Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash, &user.GoogleSub)
	return user, err
}

func (s *Store) UserByID(ctx context.Context, id uuid.UUID) (User, error) {
	var user User
	err := s.pool.QueryRow(ctx,
		`SELECT id,name,email,COALESCE(password_hash,''),COALESCE(google_sub,'') FROM users WHERE id=$1`, id).
		Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash, &user.GoogleSub)
	return user, err
}

func (s *Store) DemoUsage(ctx context.Context, id uuid.UUID, limit int) (DemoUsage, error) {
	usage := DemoUsage{Limit: limit}
	err := s.pool.QueryRow(ctx, `
		SELECT demo_runs_used, GREATEST($2 - demo_runs_used, 0)
		FROM users
		WHERE id=$1`, id, limit).
		Scan(&usage.Used, &usage.Remaining)
	usage.RequiresKeys = usage.Remaining == 0
	return usage, err
}

func (s *Store) ClaimDemoRun(ctx context.Context, id uuid.UUID, limit int) (DemoUsage, bool, error) {
	if limit <= 0 {
		usage, err := s.DemoUsage(ctx, id, limit)
		return usage, false, err
	}
	var used int
	err := s.pool.QueryRow(ctx, `
		UPDATE users
		SET demo_runs_used=demo_runs_used+1, updated_at=NOW()
		WHERE id=$1 AND demo_runs_used < $2
		RETURNING demo_runs_used`, id, limit).Scan(&used)
	if errors.Is(err, pgx.ErrNoRows) {
		usage, usageErr := s.DemoUsage(ctx, id, limit)
		return usage, false, usageErr
	}
	if err != nil {
		return DemoUsage{}, false, err
	}
	usage := DemoUsage{
		Used:      used,
		Limit:     limit,
		Remaining: limit - used,
	}
	if usage.Remaining < 0 {
		usage.Remaining = 0
	}
	usage.RequiresKeys = usage.Remaining == 0
	return usage, true, nil
}

func (s *Store) UpsertGoogleUser(ctx context.Context, name, email, googleSub string) (User, error) {
	name = strings.TrimSpace(name)
	email = strings.ToLower(strings.TrimSpace(email))
	googleSub = strings.TrimSpace(googleSub)
	if email == "" || googleSub == "" {
		return User{}, errors.New("Google identity is incomplete")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback(ctx)

	var user User
	err = tx.QueryRow(ctx, `
		SELECT id,name,email,COALESCE(password_hash,''),COALESCE(google_sub,'')
		FROM users
		WHERE google_sub=$1 OR email=$2
		ORDER BY CASE WHEN google_sub=$1 THEN 0 ELSE 1 END
		LIMIT 1
		FOR UPDATE`, googleSub, email).
		Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash, &user.GoogleSub)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		user = User{ID: uuid.New(), Name: name, Email: email, GoogleSub: googleSub}
		err = tx.QueryRow(ctx, `
			INSERT INTO users (id,name,email,password_hash,google_sub)
			VALUES ($1,$2,$3,NULL,$4)
			RETURNING id,name,email,COALESCE(password_hash,''),google_sub`,
			user.ID, user.Name, user.Email, user.GoogleSub).
			Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash, &user.GoogleSub)
	case err != nil:
		return User{}, err
	case user.GoogleSub != "" && user.GoogleSub != googleSub:
		return User{}, errors.New("email is already linked to another Google account")
	default:
		err = tx.QueryRow(ctx, `
			UPDATE users
			SET name=CASE WHEN $2 <> '' THEN $2 ELSE name END,
			    email=$3,
			    google_sub=$4,
			    updated_at=NOW()
			WHERE id=$1
			RETURNING id,name,email,COALESCE(password_hash,''),google_sub`,
			user.ID, name, email, googleSub).
			Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash, &user.GoogleSub)
	}
	if err != nil {
		return User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *Store) CreateProject(ctx context.Context, userID uuid.UUID, title, prompt string, attachments []ProjectAttachmentInput) (Project, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Project{}, err
	}
	defer tx.Rollback(ctx)

	var p Project
	err = tx.QueryRow(ctx, `
		INSERT INTO projects (id,title,initial_prompt,user_id)
		VALUES ($1,$2,$3,$4)
		RETURNING id,title,initial_prompt,user_id,sandbox_id,sandbox_url,created_at,updated_at`,
		uuid.New(), title, prompt, userID,
	).Scan(&p.ID, &p.Title, &p.InitialPrompt, &p.UserID, &p.SandboxID, &p.SandboxURL, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return Project{}, err
	}
	p.Attachments = make([]ProjectAttachment, 0, len(attachments))
	for _, input := range attachments {
		var attachment ProjectAttachment
		err = tx.QueryRow(ctx, `
			INSERT INTO project_attachments (id,project_id,name,kind,mime_type,content,size_bytes)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
			RETURNING id,project_id,name,kind,mime_type,content,size_bytes,created_at`,
			uuid.New(), p.ID, input.Name, input.Kind, input.MimeType, input.Content, input.Size,
		).Scan(
			&attachment.ID,
			&attachment.ProjectID,
			&attachment.Name,
			&attachment.Kind,
			&attachment.MimeType,
			&attachment.Content,
			&attachment.Size,
			&attachment.CreatedAt,
		)
		if err != nil {
			return Project{}, err
		}
		p.Attachments = append(p.Attachments, attachment)
	}
	if err := tx.Commit(ctx); err != nil {
		return Project{}, err
	}
	return p, nil
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
	if err != nil {
		return p, err
	}
	p.Attachments, err = s.projectAttachments(ctx, p.ID)
	return p, err
}

func (s *Store) projectAttachments(ctx context.Context, projectID uuid.UUID) ([]ProjectAttachment, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id,project_id,name,kind,mime_type,content,size_bytes,created_at
		FROM project_attachments WHERE project_id=$1 ORDER BY created_at`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	attachments := []ProjectAttachment{}
	for rows.Next() {
		var attachment ProjectAttachment
		if err := rows.Scan(
			&attachment.ID,
			&attachment.ProjectID,
			&attachment.Name,
			&attachment.Kind,
			&attachment.MimeType,
			&attachment.Content,
			&attachment.Size,
			&attachment.CreatedAt,
		); err != nil {
			return nil, err
		}
		attachments = append(attachments, attachment)
	}
	return attachments, rows.Err()
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
