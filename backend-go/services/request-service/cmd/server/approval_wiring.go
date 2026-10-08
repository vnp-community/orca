package main

import (
	"fmt"
	"log/slog"

	"github.com/stablyai/orca-go/services/request-service/internal/config"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// buildApprovalRegistry creates the registry and fills it (see fillApprovalRegistry). A nil registry means
// ApprovalService must not be served.
func buildApprovalRegistry(cfg config.Config, log *slog.Logger, real map[domain.SubjectType]usecase.SubjectHandler) (*usecase.SubjectHandlerRegistry, error) {
	if !cfg.ApprovalEnabled {
		return nil, nil
	}
	registry := usecase.NewSubjectHandlerRegistry()
	if err := fillApprovalRegistry(cfg, log, registry, real); err != nil {
		return nil, err
	}
	return registry, nil
}

// fillApprovalRegistry keeps the safety rule (never approve through a silent no-op handler): every enabled subject
// needs a real handler unless REQUEST_ALLOW_NOOP_APPROVAL_HANDLERS is set, which is for local development only.
func fillApprovalRegistry(cfg config.Config, log *slog.Logger, registry *usecase.SubjectHandlerRegistry, real map[domain.SubjectType]usecase.SubjectHandler) error {
	subjects := domain.AllSubjectTypes
	if len(cfg.ApprovalSubjects) > 0 {
		subjects = make([]domain.SubjectType, 0, len(cfg.ApprovalSubjects))
		for _, s := range cfg.ApprovalSubjects {
			st := domain.SubjectType(s)
			if !st.Valid() {
				return fmt.Errorf("REQUEST_APPROVAL_SUBJECTS: unknown subject type %q", s)
			}
			subjects = append(subjects, st)
		}
	}
	for _, st := range subjects {
		if h, ok := real[st]; ok && h != nil {
			registry.Register(st, h)
			continue
		}
		if !cfg.AllowNoopApprovalHandlers {
			return fmt.Errorf("approval enabled for %s but no real handler is registered; disable REQUEST_APPROVAL_ENABLED or set REQUEST_ALLOW_NOOP_APPROVAL_HANDLERS=true for dev", st)
		}
		registry.Register(st, &usecase.NoopSubjectHandler{Reason: "no real handler registered", Logger: log})
	}
	return nil
}
