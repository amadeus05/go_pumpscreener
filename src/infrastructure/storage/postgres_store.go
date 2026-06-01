package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"pumpscreener/src/domain"
)

type PostgresStore struct {
	db *sql.DB
}

func OpenPostgres(ctx context.Context, databaseURL string) (*PostgresStore, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, fmt.Errorf("database url is empty")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}
	store := &PostgresStore{db: db}
	if err := store.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *PostgresStore) Close() error {
	return s.db.Close()
}

func (s *PostgresStore) ListRules(ctx context.Context) ([]domain.Rule, error) {
	rows, err := s.db.QueryContext(ctx, `
		select id, direction, percent, interval_seconds, cooldown_seconds, mode, hold_seconds, enabled, created_at
		from rules
		order by id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []domain.Rule
	for rows.Next() {
		rule, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

func (s *PostgresStore) CreateRule(ctx context.Context, rule domain.Rule) (domain.Rule, error) {
	err := s.db.QueryRowContext(ctx, `
		insert into rules (direction, percent, interval_seconds, cooldown_seconds, mode, hold_seconds, enabled, created_at)
		values ($1, $2, $3, $4, $5, $6, $7, $8)
		returning id`,
		rule.Direction.String(),
		rule.Percent,
		int64(rule.Interval.Seconds()),
		int64(rule.Cooldown.Seconds()),
		rule.EffectiveMode().String(),
		int64(rule.Hold.Seconds()),
		rule.Enabled,
		rule.CreatedAt.UTC(),
	).Scan(&rule.ID)
	return rule, err
}

func (s *PostgresStore) SetRuleEnabled(ctx context.Context, id domain.RuleID, enabled bool) error {
	result, err := s.db.ExecContext(ctx, `update rules set enabled = $1 where id = $2`, enabled, id)
	if err != nil {
		return err
	}
	return requireAffected(result, "rule %d not found", id)
}

func (s *PostgresStore) DeleteRule(ctx context.Context, id domain.RuleID) error {
	result, err := s.db.ExecContext(ctx, `delete from rules where id = $1`, id)
	if err != nil {
		return err
	}
	return requireAffected(result, "rule %d not found", id)
}

func (s *PostgresStore) ListBlacklist(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `select symbol from blacklist order by symbol`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []string
	for rows.Next() {
		var symbol string
		if err := rows.Scan(&symbol); err != nil {
			return nil, err
		}
		items = append(items, symbol)
	}
	return items, rows.Err()
}

func (s *PostgresStore) AddBlacklist(ctx context.Context, symbol string) error {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		insert into blacklist (symbol)
		values ($1)
		on conflict (symbol) do nothing`, symbol)
	return err
}

func (s *PostgresStore) RemoveBlacklist(ctx context.Context, symbol string) error {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	_, err := s.db.ExecContext(ctx, `delete from blacklist where symbol = $1`, symbol)
	return err
}

func (s *PostgresStore) SaveSignal(ctx context.Context, signal domain.Signal) error {
	_, err := s.db.ExecContext(ctx, `
		insert into signals (rule_id, symbol, daily_no, price, move, triggered_at)
		values ($1, $2, $3, $4, $5, $6)`,
		signal.Rule.ID,
		signal.Symbol,
		signal.DailyNo,
		signal.Price,
		signal.Move,
		signal.Triggered.UTC(),
	)
	return err
}

func (s *PostgresStore) NextDailySignalNumber(ctx context.Context, ruleID domain.RuleID, symbol string, at time.Time) (int, error) {
	dayStart := time.Date(at.UTC().Year(), at.UTC().Month(), at.UTC().Day(), 0, 0, 0, 0, time.UTC)
	dayEnd := dayStart.Add(24 * time.Hour)

	var count int
	err := s.db.QueryRowContext(ctx, `
		select count(*)
		from signals
		where rule_id = $1
			and symbol = $2
			and triggered_at >= $3
			and triggered_at < $4`,
		ruleID,
		symbol,
		dayStart,
		dayEnd,
	).Scan(&count)
	return count + 1, err
}

func (s *PostgresStore) EnsureBlacklist(ctx context.Context, symbols []string) error {
	for _, symbol := range symbols {
		if err := s.AddBlacklist(ctx, symbol); err != nil {
			return err
		}
	}
	return nil
}

func (s *PostgresStore) migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		create table if not exists rules (
			id bigserial primary key,
			direction text not null,
			percent double precision not null,
			interval_seconds bigint not null,
			cooldown_seconds bigint not null,
			mode text not null default 'instant',
			hold_seconds bigint not null default 0,
			enabled boolean not null default true,
			created_at timestamptz not null default now()
		);

		create table if not exists blacklist (
			symbol text primary key
		);

		create table if not exists signals (
			id bigserial primary key,
			rule_id bigint not null,
			symbol text not null,
			daily_no integer not null,
			price double precision not null,
			move double precision not null,
			triggered_at timestamptz not null
		);

		create index if not exists signals_daily_idx
			on signals (rule_id, symbol, triggered_at);
	`)
	return err
}

type ruleScanner interface {
	Scan(dest ...any) error
}

func scanRule(scanner ruleScanner) (domain.Rule, error) {
	var stored storedRule
	var createdAt time.Time
	if err := scanner.Scan(
		&stored.ID,
		&stored.Direction,
		&stored.Percent,
		&stored.Interval,
		&stored.Cooldown,
		&stored.Mode,
		&stored.Hold,
		&stored.Enabled,
		&createdAt,
	); err != nil {
		return domain.Rule{}, err
	}
	stored.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	return stored.toDomain()
}

func requireAffected(result sql.Result, format string, args ...any) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf(format, args...)
	}
	return nil
}
