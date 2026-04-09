package trigger

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Sejutacita/cs-agent-bot/internal/entity"
	"github.com/Sejutacita/cs-agent-bot/internal/repository"
	"github.com/rs/zerolog"
)

// RuleEngine evaluates dynamic trigger rules from the database.
// It caches rules in memory with a TTL to avoid per-client DB queries.
type RuleEngine struct {
	ruleRepo repository.TriggerRuleRepository
	executor *ActionExecutor
	logger   zerolog.Logger

	mu        sync.RWMutex
	cache     []entity.TriggerRule
	cacheTime time.Time
	cacheTTL  time.Duration
}

// NewRuleEngine creates a new rule engine with the given dependencies.
func NewRuleEngine(
	ruleRepo repository.TriggerRuleRepository,
	executor *ActionExecutor,
	logger zerolog.Logger,
) *RuleEngine {
	return &RuleEngine{
		ruleRepo: ruleRepo,
		executor: executor,
		logger:   logger,
		cacheTTL: 5 * time.Minute,
	}
}

// InvalidateCache forces the next EvaluateAll call to reload rules from DB.
func (e *RuleEngine) InvalidateCache() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cache = nil
	e.cacheTime = time.Time{}
}

// loadRules returns cached rules or reloads from DB if expired.
func (e *RuleEngine) loadRules(ctx context.Context) ([]entity.TriggerRule, error) {
	e.mu.RLock()
	if e.cache != nil && time.Since(e.cacheTime) < e.cacheTTL {
		rules := e.cache
		e.mu.RUnlock()
		return rules, nil
	}
	e.mu.RUnlock()

	e.mu.Lock()
	defer e.mu.Unlock()

	// Double-check after acquiring write lock
	if e.cache != nil && time.Since(e.cacheTime) < e.cacheTTL {
		return e.cache, nil
	}

	rules, err := e.ruleRepo.GetActiveRulesOrdered(ctx)
	if err != nil {
		return nil, fmt.Errorf("load trigger rules: %w", err)
	}

	e.cache = rules
	e.cacheTime = time.Now()
	e.logger.Info().Int("count", len(rules)).Msg("Trigger rules cache refreshed")

	return rules, nil
}

// EvaluateAll evaluates all active trigger rules against the given client context.
// Rules are evaluated in priority order. The first matching rule fires its action.
// Returns (true, nil) if a rule fired, (false, nil) if no rule matched.
func (e *RuleEngine) EvaluateAll(ctx context.Context, clientCtx *ClientContext) (bool, error) {
	rules, err := e.loadRules(ctx)
	if err != nil {
		return false, err
	}

	for _, rule := range rules {
		match, err := EvaluateCondition(rule.Condition, clientCtx)
		if err != nil {
			e.logger.Error().
				Err(err).
				Str("rule_id", rule.RuleID).
				Str("company_id", clientCtx.Client.CompanyID).
				Msg("Rule condition evaluation error, skipping rule")
			continue
		}

		if !match {
			continue
		}

		e.logger.Info().
			Str("rule_id", rule.RuleID).
			Str("company_id", clientCtx.Client.CompanyID).
			Str("action_type", rule.ActionType).
			Msg("Trigger rule matched")

		if err := e.executor.Execute(ctx, rule, clientCtx); err != nil {
			return false, fmt.Errorf("execute rule %s: %w", rule.RuleID, err)
		}

		if rule.StopOnFire {
			return true, nil
		}
	}

	return false, nil
}
