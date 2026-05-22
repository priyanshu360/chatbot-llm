package repo

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/priyanshu360/chatbot-llm/pkg"
)

type MessageRepo struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

func NewMessageRepo(pool *pgxpool.Pool, logger *slog.Logger) *MessageRepo {
	return &MessageRepo{pool: pool, logger: logger}
}

func (r *MessageRepo) Insert(ctx context.Context, conversationID, role, content, contentRedacted string, seq int) (*pkg.Message, error) {
	r.logger.Debug("inserting message", "conversation_id", conversationID, "role", role, "seq", seq)
	m := &pkg.Message{}
	err := r.pool.QueryRow(ctx,
		`INSERT INTO messages (conversation_id, role, content, content_redacted, seq)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, conversation_id, role, content, seq, created_at`,
		conversationID, role, content, contentRedacted, seq,
	).Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &m.Seq, &m.CreatedAt)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (r *MessageRepo) GetByConversation(ctx context.Context, conversationID string) ([]pkg.Message, error) {
	r.logger.Debug("getting messages", "conversation_id", conversationID)
	rows, err := r.pool.Query(ctx,
		`SELECT id, conversation_id, role, content, seq, created_at
		 FROM messages WHERE conversation_id = $1 ORDER BY seq ASC`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []pkg.Message
	for rows.Next() {
		var m pkg.Message
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &m.Seq, &m.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}
	return messages, nil
}

func (r *MessageRepo) NextSeq(ctx context.Context, conversationID string) (int, error) {
	var seq int
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(MAX(seq), 0) + 1 FROM messages WHERE conversation_id = $1`,
		conversationID,
	).Scan(&seq)
	if err != nil {
		return 0, err
	}
	return seq, nil
}
