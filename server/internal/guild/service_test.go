package guild

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"

	dbgen "github.com/esuEdu/go-tauri-discord/internal/db/gen"
	"github.com/esuEdu/go-tauri-discord/internal/domain"
)

type countingRepo struct {
	Repository

	calls    []string
	channels []dbgen.Channel
	owner    uuid.UUID
	roles    string
	member   bool
}

func (r *countingRepo) record(name string) { r.calls = append(r.calls, name) }

func (r *countingRepo) ResolveChannelAccess(context.Context, dbgen.ResolveChannelAccessParams) (dbgen.ResolveChannelAccessRow, error) {
	r.record("ResolveChannelAccess")
	return dbgen.ResolveChannelAccessRow{
		Channel:    dbgen.Channel{ID: uuid.New(), Kind: domain.ChannelText},
		OwnerID:    r.owner,
		IsMember:   r.member,
		Roles:      r.roles,
		Overwrites: "[]",
	}, nil
}

func (r *countingRepo) ResolveGuildAccess(context.Context, dbgen.ResolveGuildAccessParams) (dbgen.ResolveGuildAccessRow, error) {
	r.record("ResolveGuildAccess")
	return dbgen.ResolveGuildAccessRow{OwnerID: r.owner, IsMember: r.member, Roles: r.roles}, nil
}

func (r *countingRepo) ListChannels(context.Context, uuid.UUID) ([]dbgen.Channel, error) {
	r.record("ListChannels")
	return r.channels, nil
}

func (r *countingRepo) ListGuildOverwrites(context.Context, uuid.UUID) ([]dbgen.ChannelOverwrite, error) {
	r.record("ListGuildOverwrites")
	return nil, nil
}

func viewerRepo(channels int) *countingRepo {
	rows := make([]dbgen.Channel, channels)
	for i := range rows {
		rows[i] = dbgen.Channel{ID: uuid.New(), Kind: domain.ChannelText}
	}
	return &countingRepo{
		channels: rows,
		owner:    uuid.New(),
		member:   true,
		roles:    fmt.Sprintf(`[{"id":%q,"permissions":%d}]`, uuid.New(), domain.PermAll),
	}
}

func TestPermissionsInAsksTheDatabaseOnce(t *testing.T) {
	repo := viewerRepo(0)
	svc := NewService(repo, nil, nil, nil)

	if _, _, err := svc.PermissionsIn(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatalf("permissions in: %v", err)
	}
	if got := strings.Join(repo.calls, ", "); got != "ResolveChannelAccess" {
		t.Errorf("a permission check ran [%s]; it is the hot path of every send, edit, "+
			"delete, typing indicator and history read, and it costs one query", got)
	}
}

func TestPermissionsInGuildAsksTheDatabaseOnce(t *testing.T) {
	repo := viewerRepo(0)
	svc := NewService(repo, nil, nil, nil)

	if _, err := svc.PermissionsInGuild(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatalf("permissions in guild: %v", err)
	}
	if got := strings.Join(repo.calls, ", "); got != "ResolveGuildAccess" {
		t.Errorf("a guild permission check ran [%s], want one query", got)
	}
}

func TestListChannelsDoesNotGrowWithTheGuild(t *testing.T) {
	small := viewerRepo(1)
	large := viewerRepo(40)

	for _, repo := range []*countingRepo{small, large} {
		svc := NewService(repo, nil, nil, nil)
		visible, err := svc.ListChannels(context.Background(), uuid.New(), uuid.New())
		if err != nil {
			t.Fatalf("list channels: %v", err)
		}
		if len(visible) != len(repo.channels) {
			t.Fatalf("saw %d of %d channels", len(visible), len(repo.channels))
		}
	}

	if len(small.calls) != len(large.calls) {
		t.Errorf("listing 1 channel took %d queries and 40 took %d; the cost has to be flat "+
			"in the number of channels, not one query per channel", len(small.calls), len(large.calls))
	}
}

func TestPermissionsSurviveTheTripThroughJSON(t *testing.T) {
	roleID, memberID := uuid.New(), uuid.New()
	repo := &countingRepo{
		owner:  uuid.New(),
		member: true,
		roles:  fmt.Sprintf(`[{"id":%q,"permissions":%d}]`, roleID, domain.PermViewChannel|domain.PermSendMessages),
	}
	svc := NewService(repo, nil, nil, nil)

	perms, _, err := svc.PermissionsIn(context.Background(), memberID, uuid.New())
	if err != nil {
		t.Fatalf("permissions in: %v", err)
	}
	if !perms.Has(domain.PermViewChannel) || !perms.Has(domain.PermSendMessages) {
		t.Errorf("granted permissions %d, want view and send to survive being aggregated "+
			"into JSON by Postgres and decoded back", perms)
	}
	if perms.Has(domain.PermManageChannels) {
		t.Error("a permission nobody granted came back set")
	}
}

func TestANonMemberIsNotToldTheChannelExists(t *testing.T) {
	repo := viewerRepo(0)
	repo.member = false
	svc := NewService(repo, nil, nil, nil)

	_, _, err := svc.PermissionsIn(context.Background(), uuid.New(), uuid.New())
	var domainErr *domain.Error
	if !errors.As(err, &domainErr) || domainErr.Kind != domain.KindNotFound {
		t.Fatalf("error = %v, want not-found so membership is not leaked by a forbidden", err)
	}
}
