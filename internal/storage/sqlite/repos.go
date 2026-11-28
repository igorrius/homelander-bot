package sqlite

import "github.com/igorrius/homelander-bot/internal/domain"

// NewRepos returns every repository backed by db.
func NewRepos(db DB) domain.Repos {
	return domain.Repos{
		Tx: db, Users: Users{DB: db}, Channels: Channels{DB: db}, Houses: Houses{DB: db},
		Statuses: Statuses{DB: db}, Residencies: Residencies{DB: db}, Codes: Codes{DB: db},
		Sessions: Sessions{DB: db}, Site: Site{DB: db}, Audit: Audit{DB: db},
		Chats: Chats{DB: db}, Topics: Topics{DB: db}, Labels: Labels{DB: db},
		Outbox: Outbox{DB: db}, RegTokens: RegTokens{DB: db}, Notifications: Notifications{DB: db},
		Data: Data{DB: db}, Pages: Pages{DB: db}, MCP: MCP{DB: db}, LoginLinks: LoginLinks{DB: db},
	}
}
