package db

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// CreateOrgWithOwner creates an organization and adds the user as its owner,
// atomically.
func (s *Store) CreateOrgWithOwner(ctx context.Context, name, slug, userID string) (Organization, error) {
	var o Organization
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return o, err
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx,
		`INSERT INTO organizations (name, slug) VALUES ($1, $2)
		 RETURNING id, name, slug, created_at`,
		name, slug,
	).Scan(&o.ID, &o.Name, &o.Slug, &o.CreatedAt)
	if err != nil {
		return o, err
	}
	if _, err = tx.Exec(ctx,
		`INSERT INTO organization_users (organization_id, user_id, role) VALUES ($1, $2, 'owner')`,
		o.ID, userID,
	); err != nil {
		return o, err
	}
	o.Role = "owner"
	return o, tx.Commit(ctx)
}

// ListOrgsForUser returns every org the user belongs to, with their role.
func (s *Store) ListOrgsForUser(ctx context.Context, userID string) ([]Organization, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT o.id, o.name, o.slug, o.created_at, ou.role
		   FROM organizations o
		   JOIN organization_users ou ON ou.organization_id = o.id
		  WHERE ou.user_id = $1
		  ORDER BY o.created_at`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orgs := []Organization{}
	for rows.Next() {
		var o Organization
		if err := rows.Scan(&o.ID, &o.Name, &o.Slug, &o.CreatedAt, &o.Role); err != nil {
			return nil, err
		}
		orgs = append(orgs, o)
	}
	return orgs, rows.Err()
}

// OrgMembershipBySlug returns the org plus the caller's role, or ErrNotFound if
// the org does not exist or the user is not a member (we don't distinguish, to
// avoid leaking org existence).
func (s *Store) OrgMembershipBySlug(ctx context.Context, slug, userID string) (Organization, error) {
	var o Organization
	err := s.Pool.QueryRow(ctx,
		`SELECT o.id, o.name, o.slug, o.created_at, ou.role
		   FROM organizations o
		   JOIN organization_users ou ON ou.organization_id = o.id
		  WHERE o.slug = $1 AND ou.user_id = $2`,
		slug, userID,
	).Scan(&o.ID, &o.Name, &o.Slug, &o.CreatedAt, &o.Role)
	if errors.Is(err, pgx.ErrNoRows) {
		return o, ErrNotFound
	}
	return o, err
}

func (s *Store) ListMembers(ctx context.Context, orgID string) ([]Member, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT u.id, u.username, ou.role, ou.created_at
		   FROM organization_users ou JOIN users u ON u.id = ou.user_id
		  WHERE ou.organization_id = $1
		  ORDER BY ou.created_at`,
		orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := []Member{}
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.UserID, &m.Username, &m.Role, &m.JoinedAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

// AddMemberByUsername adds an existing user to the org. Idempotent on conflict.
func (s *Store) AddMemberByUsername(ctx context.Context, orgID, username, role string) (Member, error) {
	u, err := s.GetUserByUsername(ctx, username)
	if err != nil {
		return Member{}, err
	}
	_, err = s.Pool.Exec(ctx,
		`INSERT INTO organization_users (organization_id, user_id, role) VALUES ($1, $2, $3)
		 ON CONFLICT (organization_id, user_id) DO UPDATE SET role = EXCLUDED.role`,
		orgID, u.ID, role,
	)
	if err != nil {
		return Member{}, err
	}
	return Member{UserID: u.ID, Username: u.Username, Role: role}, nil
}

func (s *Store) RemoveMember(ctx context.Context, orgID, userID string) error {
	_, err := s.Pool.Exec(ctx,
		`DELETE FROM organization_users WHERE organization_id = $1 AND user_id = $2`,
		orgID, userID,
	)
	return err
}

// --- notification channels ---

func (s *Store) ListChannels(ctx context.Context, orgID string) ([]NotificationChannel, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, organization_id, type, name, webhook_url, enabled, created_at
		   FROM notification_channels WHERE organization_id = $1 ORDER BY created_at`,
		orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	chans := []NotificationChannel{}
	for rows.Next() {
		var c NotificationChannel
		if err := rows.Scan(&c.ID, &c.OrganizationID, &c.Type, &c.Name, &c.WebhookURL, &c.Enabled, &c.CreatedAt); err != nil {
			return nil, err
		}
		chans = append(chans, c)
	}
	return chans, rows.Err()
}

func (s *Store) CreateChannel(ctx context.Context, orgID, name, webhookURL string) (NotificationChannel, error) {
	var c NotificationChannel
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO notification_channels (organization_id, type, name, webhook_url)
		 VALUES ($1, 'discord', $2, $3)
		 RETURNING id, organization_id, type, name, webhook_url, enabled, created_at`,
		orgID, name, webhookURL,
	).Scan(&c.ID, &c.OrganizationID, &c.Type, &c.Name, &c.WebhookURL, &c.Enabled, &c.CreatedAt)
	return c, err
}

func (s *Store) DeleteChannel(ctx context.Context, orgID, channelID string) error {
	_, err := s.Pool.Exec(ctx,
		`DELETE FROM notification_channels WHERE id = $1 AND organization_id = $2`,
		channelID, orgID,
	)
	return err
}

// EnabledWebhooksForOrg returns the webhook URLs of all enabled discord channels.
func (s *Store) EnabledWebhooksForOrg(ctx context.Context, orgID string) ([]string, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT webhook_url FROM notification_channels
		  WHERE organization_id = $1 AND enabled AND type = 'discord'`,
		orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	urls := []string{}
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return nil, err
		}
		urls = append(urls, u)
	}
	return urls, rows.Err()
}
