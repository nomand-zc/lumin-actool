package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
	"github.com/nomand-zc/lumin-actool/storage"
)

// Storage SQLite 持久化存储实现
type Storage struct {
	db *sql.DB
}

// New 创建 SQLite 存储实例
func New(dbPath string) (storage.StateStorage, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open database: %w", err)
	}

	s := &Storage{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite: migrate: %w", err)
	}

	return s, nil
}

// migrate 自动创建表结构
func (s *Storage) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS pipelines (
			id TEXT PRIMARY KEY,
			email_producer TEXT NOT NULL,
			provider_registrar TEXT NOT NULL,
			count INTEGER NOT NULL DEFAULT 0,
			completed_count INTEGER NOT NULL DEFAULT 0,
			failed_count INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'running',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS email_accounts (
			id TEXT PRIMARY KEY,
			pipeline_id TEXT NOT NULL,
			email TEXT NOT NULL,
			password TEXT NOT NULL,
			provider TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'available',
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS credentials (
			id TEXT PRIMARY KEY,
			pipeline_id TEXT NOT NULL,
			provider_type TEXT NOT NULL,
			email TEXT NOT NULL,
			credential_json TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'active',
			created_at TEXT NOT NULL,
			expires_at TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_email_accounts_pipeline ON email_accounts(pipeline_id)`,
		`CREATE INDEX IF NOT EXISTS idx_email_accounts_status ON email_accounts(status)`,
		`CREATE INDEX IF NOT EXISTS idx_credentials_pipeline ON credentials(pipeline_id)`,
		`CREATE INDEX IF NOT EXISTS idx_credentials_provider ON credentials(provider_type)`,
	}

	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("execute migration: %w", err)
		}
	}

	return nil
}

func (s *Storage) SavePipeline(ctx context.Context, p *storage.PipelineState) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO pipelines (id, email_producer, provider_registrar, count, completed_count, failed_count, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.EmailProducer, p.ProviderRegistrar, p.Count, p.CompletedCount, p.FailedCount, p.Status, p.CreatedAt, p.UpdatedAt,
	)
	return err
}

func (s *Storage) GetPipeline(ctx context.Context, id string) (*storage.PipelineState, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, email_producer, provider_registrar, count, completed_count, failed_count, status, created_at, updated_at
		 FROM pipelines WHERE id = ?`, id,
	)

	p := &storage.PipelineState{}
	err := row.Scan(&p.ID, &p.EmailProducer, &p.ProviderRegistrar, &p.Count, &p.CompletedCount, &p.FailedCount, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Storage) ListPipelines(ctx context.Context) ([]*storage.PipelineState, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, email_producer, provider_registrar, count, completed_count, failed_count, status, created_at, updated_at
		 FROM pipelines ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pipelines []*storage.PipelineState
	for rows.Next() {
		p := &storage.PipelineState{}
		if err := rows.Scan(&p.ID, &p.EmailProducer, &p.ProviderRegistrar, &p.Count, &p.CompletedCount, &p.FailedCount, &p.Status, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		pipelines = append(pipelines, p)
	}
	return pipelines, rows.Err()
}

func (s *Storage) SaveEmailAccount(ctx context.Context, a *storage.EmailAccountState) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO email_accounts (id, pipeline_id, email, password, provider, status, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.PipelineID, a.Email, a.Password, a.Provider, a.Status, a.CreatedAt,
	)
	return err
}

func (s *Storage) ListEmailAccounts(ctx context.Context, filter *storage.EmailFilter) ([]*storage.EmailAccountState, error) {
	query := `SELECT id, pipeline_id, email, password, provider, status, created_at FROM email_accounts WHERE 1=1`
	var args []any

	if filter != nil {
		if filter.PipelineID != "" {
			query += ` AND pipeline_id = ?`
			args = append(args, filter.PipelineID)
		}
		if filter.Provider != "" {
			query += ` AND provider = ?`
			args = append(args, filter.Provider)
		}
		if filter.Status != "" {
			query += ` AND status = ?`
			args = append(args, filter.Status)
		}
	}

	query += ` ORDER BY created_at DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []*storage.EmailAccountState
	for rows.Next() {
		a := &storage.EmailAccountState{}
		if err := rows.Scan(&a.ID, &a.PipelineID, &a.Email, &a.Password, &a.Provider, &a.Status, &a.CreatedAt); err != nil {
			return nil, err
		}
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}

func (s *Storage) SaveCredential(ctx context.Context, c *storage.CredentialState) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO credentials (id, pipeline_id, provider_type, email, credential_json, status, created_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.PipelineID, c.ProviderType, c.Email, c.CredentialJSON, c.Status, c.CreatedAt, c.ExpiresAt,
	)
	return err
}

func (s *Storage) ListCredentials(ctx context.Context, filter *storage.CredentialFilter) ([]*storage.CredentialState, error) {
	query := `SELECT id, pipeline_id, provider_type, email, credential_json, status, created_at, expires_at FROM credentials WHERE 1=1`
	var args []any

	if filter != nil {
		if filter.PipelineID != "" {
			query += ` AND pipeline_id = ?`
			args = append(args, filter.PipelineID)
		}
		if filter.ProviderType != "" {
			query += ` AND provider_type = ?`
			args = append(args, filter.ProviderType)
		}
		if filter.Status != "" {
			query += ` AND status = ?`
			args = append(args, filter.Status)
		}
	}

	query += ` ORDER BY created_at DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var creds []*storage.CredentialState
	for rows.Next() {
		c := &storage.CredentialState{}
		if err := rows.Scan(&c.ID, &c.PipelineID, &c.ProviderType, &c.Email, &c.CredentialJSON, &c.Status, &c.CreatedAt, &c.ExpiresAt); err != nil {
			return nil, err
		}
		creds = append(creds, c)
	}
	return creds, rows.Err()
}

func (s *Storage) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
