package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/pkg/database"
	"github.com/Sejutacita/cs-agent-bot/internal/tracer"
	"github.com/rs/zerolog"
)

type NotificationRepository interface {
	Create(ctx context.Context, n *entity.Notification) (*entity.Notification, error)
	List(ctx context.Context, filter entity.NotificationFilter) ([]entity.Notification, int64, error)
	CountUnread(ctx context.Context, workspaceID, recipientEmail string) (int64, error)
	MarkRead(ctx context.Context, id, recipientEmail string) error
	MarkAllRead(ctx context.Context, workspaceID, recipientEmail string) error
}

type notificationRepo struct {
	db           *sql.DB
	queryTimeout time.Duration
	tracer       tracer.Tracer
	logger       zerolog.Logger
}

func NewNotificationRepo(db *sql.DB, queryTimeout time.Duration, tr tracer.Tracer, logger zerolog.Logger) NotificationRepository {
	return &notificationRepo{db: db, queryTimeout: queryTimeout, tracer: tr, logger: logger}
}

func (r *notificationRepo) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

const notificationColumns = "id::text, workspace_id::text, recipient_email, type, icon, message, href, source_feature, source_id, read, read_at, telegram_sent, email_sent, created_at"

func scanNotification(scanner interface {
	Scan(dest ...interface{}) error
}) (*entity.Notification, error) {
	var n entity.Notification
	err := scanner.Scan(
		&n.ID, &n.WorkspaceID, &n.RecipientEmail, &n.Type, &n.Icon,
		&n.Message, &n.Href, &n.SourceFeature, &n.SourceID,
		&n.Read, &n.ReadAt, &n.TelegramSent, &n.EmailSent, &n.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &n, nil
}

func (r *notificationRepo) Create(ctx context.Context, n *entity.Notification) (*entity.Notification, error) {
	ctx, span := r.tracer.Start(ctx, "notification.repository.Create")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := database.PSQL.
		Insert("notifications").
		Columns("workspace_id", "recipient_email", "type", "icon", "message", "href", "source_feature", "source_id").
		Values(n.WorkspaceID, n.RecipientEmail, n.Type, n.Icon, n.Message, n.Href, n.SourceFeature, n.SourceID).
		Suffix("RETURNING " + notificationColumns).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build insert: %w", err)
	}
	out, err := scanNotification(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		return nil, fmt.Errorf("insert notification: %w", err)
	}
	return out, nil
}

func (r *notificationRepo) List(ctx context.Context, filter entity.NotificationFilter) ([]entity.Notification, int64, error) {
	ctx, span := r.tracer.Start(ctx, "notification.repository.List")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	conds := sq.And{
		sq.Expr("workspace_id::text = ?", filter.WorkspaceID),
		sq.Eq{"recipient_email": filter.RecipientEmail},
	}
	if filter.UnreadOnly {
		conds = append(conds, sq.Eq{"read": false})
	}
	if filter.Type != "" {
		conds = append(conds, sq.Eq{"type": filter.Type})
	}

	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	query, args, err := database.PSQL.
		Select(notificationColumns).
		From("notifications").
		Where(conds).
		OrderBy("created_at DESC").
		Limit(uint64(limit)).
		Offset(uint64(filter.Offset)).
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("build query: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query notifications: %w", err)
	}
	defer rows.Close()

	var out []entity.Notification
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan notification: %w", err)
		}
		out = append(out, *n)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	countQuery, countArgs, err := database.PSQL.
		Select("COUNT(*)").
		From("notifications").
		Where(conds).
		ToSql()
	if err != nil {
		return nil, 0, fmt.Errorf("build count: %w", err)
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count notifications: %w", err)
	}
	return out, total, nil
}

func (r *notificationRepo) CountUnread(ctx context.Context, workspaceID, recipientEmail string) (int64, error) {
	ctx, span := r.tracer.Start(ctx, "notification.repository.CountUnread")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	var n int64
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM notifications WHERE workspace_id::text = $1 AND recipient_email = $2 AND read = FALSE",
		workspaceID, recipientEmail,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count unread: %w", err)
	}
	return n, nil
}

func (r *notificationRepo) MarkRead(ctx context.Context, id, recipientEmail string) error {
	ctx, span := r.tracer.Start(ctx, "notification.repository.MarkRead")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	res, err := r.db.ExecContext(ctx,
		"UPDATE notifications SET read = TRUE, read_at = NOW() WHERE id::text = $1 AND recipient_email = $2",
		id, recipientEmail,
	)
	if err != nil {
		return fmt.Errorf("mark read: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("notification not found")
	}
	return nil
}

func (r *notificationRepo) MarkAllRead(ctx context.Context, workspaceID, recipientEmail string) error {
	ctx, span := r.tracer.Start(ctx, "notification.repository.MarkAllRead")
	defer span.End()
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	_, err := r.db.ExecContext(ctx,
		"UPDATE notifications SET read = TRUE, read_at = NOW() WHERE workspace_id::text = $1 AND recipient_email = $2 AND read = FALSE",
		workspaceID, recipientEmail,
	)
	if err != nil {
		return fmt.Errorf("mark all read: %w", err)
	}
	return nil
}
