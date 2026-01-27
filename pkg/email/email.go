package email

import (
	"context"
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"

	"github.com/bacchus-snu/sgs/model"
)

// Service sends email notifications.
type Service interface {
	// SendWorkspaceRequestNotification notifies subscribed admins about a new workspace request.
	SendWorkspaceRequestNotification(ctx context.Context, ws *model.Workspace, subscribers []model.Subscriber) error
	// SendWorkspaceApprovalNotification notifies workspace users about approval/denial.
	SendWorkspaceApprovalNotification(ctx context.Context, ws *model.Workspace, approved bool) error
}

type smtpService struct {
	cfg  Config
	auth smtp.Auth
}

// NewSMTPService creates a new SMTP email service.
func NewSMTPService(cfg Config) Service {
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	return &smtpService{cfg: cfg, auth: auth}
}

func (s *smtpService) sendEmail(to []string, subject, body string) error {
	if len(to) == 0 {
		return nil
	}

	msg := fmt.Sprintf("From: %s\r\n"+
		"To: %s\r\n"+
		"Subject: %s\r\n"+
		"MIME-Version: 1.0\r\n"+
		"Content-Type: text/plain; charset=\"UTF-8\"\r\n"+
		"\r\n"+
		"%s",
		s.cfg.From,
		strings.Join(to, ", "),
		subject,
		body,
	)

	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	return smtp.SendMail(addr, s.auth, s.cfg.From, to, []byte(msg))
}

func (s *smtpService) SendWorkspaceRequestNotification(ctx context.Context, ws *model.Workspace, subscribers []model.Subscriber) error {
	if len(subscribers) == 0 {
		return nil
	}

	to := make([]string, len(subscribers))
	for i, sub := range subscribers {
		to[i] = sub.Email
	}

	// Find the requester (first user with email set)
	requester := "unknown"
	for _, u := range ws.Users {
		if u.Email != "" {
			requester = u.Username
			break
		}
	}

	subject := fmt.Sprintf("[SGS] New Workspace Request from %s", requester)
	body := fmt.Sprintf(`A new workspace has been requested:

Workspace ID: %d
Requested by: %s
Nodegroup: %s
GPUs: %d

Review at: https://sgs.snucse.org/ws/%s
`,
		ws.ID,
		requester,
		ws.Nodegroup,
		ws.Quotas[model.ResGPURequest],
		ws.ID.Hash(),
	)

	if err := s.sendEmail(to, subject, body); err != nil {
		slog.Error("failed to send workspace request notification", "error", err, "workspace_id", ws.ID)
		return err
	}
	return nil
}

func (s *smtpService) SendWorkspaceApprovalNotification(ctx context.Context, ws *model.Workspace, approved bool) error {
	// Collect user emails
	var to []string
	for _, u := range ws.Users {
		if u.Email != "" {
			to = append(to, u.Email)
		}
	}

	if len(to) == 0 {
		return nil
	}

	var subject, body string
	if approved {
		subject = "[SGS] Your Workspace Request Has Been Approved"
		body = fmt.Sprintf(`Your workspace request has been approved and is now active.

Workspace ID: %d
Namespace: ws-%s

Access your workspace: https://sgs.snucse.org/ws/%s
Documentation: https://sgs-docs.snucse.org
`,
			ws.ID,
			ws.ID.Hash(),
			ws.ID.Hash(),
		)
	} else {
		subject = "[SGS] Your Workspace Request Has Been Denied"
		body = fmt.Sprintf(`Your workspace request has been reviewed and was not approved.

Workspace ID: %d

Please contact the administrators if you have questions.
`,
			ws.ID,
		)
	}

	if err := s.sendEmail(to, subject, body); err != nil {
		slog.Error("failed to send workspace approval notification", "error", err, "workspace_id", ws.ID, "approved", approved)
		return err
	}
	return nil
}
