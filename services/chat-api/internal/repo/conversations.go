package repo

import (
	"context"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/priyanshu360/chatbot-llm/pkg"
)

type ConversationRepo struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

func NewConversationRepo(pool *pgxpool.Pool, logger *slog.Logger) *ConversationRepo {
	return &ConversationRepo{pool: pool, logger: logger}
}

func (r *ConversationRepo) Create(ctx context.Context, title string) (*pkg.Conversation, error) {
	r.logger.Debug("creating conversation", "title", title)
	c := &pkg.Conversation{}
	err := r.pool.QueryRow(ctx,
		`INSERT INTO conversations (title)
		 VALUES ($1)
		 RETURNING id, title, status, created_at, updated_at`,
		title,
	).Scan(&c.ID, &c.Title, &c.Status, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (r *ConversationRepo) GetByID(ctx context.Context, id string) (*pkg.Conversation, error) {
	c := &pkg.Conversation{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, title, status, created_at, updated_at
		 FROM conversations WHERE id = $1`, id,
	).Scan(&c.ID, &c.Title, &c.Status, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (r *ConversationRepo) List(ctx context.Context) ([]pkg.Conversation, error) {
	r.logger.Debug("listing conversations")
	rows, err := r.pool.Query(ctx,
		`SELECT id, title, status, created_at, updated_at
		 FROM conversations ORDER BY updated_at DESC LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var conversations []pkg.Conversation
	for rows.Next() {
		var c pkg.Conversation
		if err := rows.Scan(&c.ID, &c.Title, &c.Status, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		conversations = append(conversations, c)
	}
	return conversations, nil
}

func (r *ConversationRepo) UpdateStatus(ctx context.Context, id, status string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE conversations SET status = $1, updated_at = now() WHERE id = $2`,
		status, id,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *ConversationRepo) Delete(ctx context.Context, id string) error {
	r.logger.Debug("deleting conversation", "id", id)
	tag, err := r.pool.Exec(ctx, `DELETE FROM conversations WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *ConversationRepo) Touch(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE conversations SET updated_at = now() WHERE id = $1`, id)
	return err
}
