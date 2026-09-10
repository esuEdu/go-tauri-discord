package identity

import (
	"context"
	"sync"

	"github.com/google/uuid"

	"github.com/esuEdu/go-tauri-discord/internal/db"
	dbgen "github.com/esuEdu/go-tauri-discord/internal/db/gen"
	"github.com/esuEdu/go-tauri-discord/internal/domain"
	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

type Repository interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (dbgen.User, error)
	GetUserByPublicID(ctx context.Context, publicID string) (dbgen.User, error)
}

type Directory struct {
	repo Repository

	mu      sync.RWMutex
	public  map[uuid.UUID]events.UserID
	private map[events.UserID]uuid.UUID
}

func NewDirectory(repo Repository) *Directory {
	return &Directory{
		repo:    repo,
		public:  make(map[uuid.UUID]events.UserID),
		private: make(map[events.UserID]uuid.UUID),
	}
}

func (d *Directory) Remember(user dbgen.User) {
	d.store(user.ID, events.UserID(user.PublicID))
}

func (d *Directory) Public(ctx context.Context, id uuid.UUID) (events.UserID, error) {
	d.mu.RLock()
	known, ok := d.public[id]
	d.mu.RUnlock()
	if ok {
		return known, nil
	}

	user, err := d.repo.GetUserByID(ctx, id)
	if err != nil {
		if db.IsNoRows(err) {
			return "", domain.NotFound("user")
		}
		return "", domain.Internal(err)
	}
	d.Remember(user)
	return events.UserID(user.PublicID), nil
}

func (d *Directory) Resolve(ctx context.Context, public events.UserID) (uuid.UUID, error) {
	d.mu.RLock()
	known, ok := d.private[public]
	d.mu.RUnlock()
	if ok {
		return known, nil
	}

	user, err := d.repo.GetUserByPublicID(ctx, string(public))
	if err != nil {
		if db.IsNoRows(err) {
			return uuid.Nil, domain.NotFound("user")
		}
		return uuid.Nil, domain.Internal(err)
	}
	d.Remember(user)
	return user.ID, nil
}

func (d *Directory) store(id uuid.UUID, public events.UserID) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.public[id] = public
	d.private[public] = id
}
