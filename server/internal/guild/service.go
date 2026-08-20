package guild

import (
	"context"
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/esuEdu/go-tauri-discord/internal/db"
	dbgen "github.com/esuEdu/go-tauri-discord/internal/db/gen"
	"github.com/esuEdu/go-tauri-discord/internal/domain"
	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

type Repository interface {
	CreateGuild(ctx context.Context, arg dbgen.CreateGuildParams) (dbgen.Guild, error)
	GetGuild(ctx context.Context, id uuid.UUID) (dbgen.Guild, error)
	ListGuildsForUser(ctx context.Context, userID uuid.UUID) ([]dbgen.Guild, error)
	CreateChannel(ctx context.Context, arg dbgen.CreateChannelParams) (dbgen.Channel, error)
	GetChannel(ctx context.Context, id uuid.UUID) (dbgen.Channel, error)
	ListChannels(ctx context.Context, guildID uuid.UUID) ([]dbgen.Channel, error)
	AddGuildMember(ctx context.Context, arg dbgen.AddGuildMemberParams) (dbgen.GuildMember, error)
	GetGuildMember(ctx context.Context, arg dbgen.GetGuildMemberParams) (dbgen.GuildMember, error)
	ListGuildMembers(ctx context.Context, guildID uuid.UUID) ([]dbgen.ListGuildMembersRow, error)
	ListGuildMemberIDs(ctx context.Context, guildID uuid.UUID) ([]uuid.UUID, error)
	ListGuildOverwrites(ctx context.Context, guildID uuid.UUID) ([]dbgen.ChannelOverwrite, error)
	ResolveChannelAccess(ctx context.Context, arg dbgen.ResolveChannelAccessParams) (dbgen.ResolveChannelAccessRow, error)
	ResolveGuildAccess(ctx context.Context, arg dbgen.ResolveGuildAccessParams) (dbgen.ResolveGuildAccessRow, error)
	CreateRole(ctx context.Context, arg dbgen.CreateRoleParams) (dbgen.Role, error)
	ListRoles(ctx context.Context, guildID uuid.UUID) ([]dbgen.Role, error)
	GetRole(ctx context.Context, id uuid.UUID) (dbgen.Role, error)
	UpdateRole(ctx context.Context, arg dbgen.UpdateRoleParams) (dbgen.Role, error)
	AssignRole(ctx context.Context, arg dbgen.AssignRoleParams) error
	UnassignRole(ctx context.Context, arg dbgen.UnassignRoleParams) error
	ListMemberRoles(ctx context.Context, arg dbgen.ListMemberRolesParams) ([]dbgen.Role, error)
	UpsertChannelOverwrite(ctx context.Context, arg dbgen.UpsertChannelOverwriteParams) error
	DeleteChannelOverwrite(ctx context.Context, arg dbgen.DeleteChannelOverwriteParams) error
	ListChannelOverwrites(ctx context.Context, channelID uuid.UUID) ([]dbgen.ChannelOverwrite, error)
}

type TxRunner interface {
	InTx(ctx context.Context, fn func(q *dbgen.Queries) error) error
}

type Service struct {
	repo    Repository
	invites InviteRepository
	tx      TxRunner
}

func NewService(repo Repository, invites InviteRepository, tx TxRunner) *Service {
	return &Service{repo: repo, invites: invites, tx: tx}
}

const maxGuildNameLen = 100

func (s *Service) Create(ctx context.Context, ownerID uuid.UUID, name string) (dbgen.Guild, error) {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n < 1 || n > maxGuildNameLen {
		return dbgen.Guild{}, domain.Invalid("guild name must be 1-%d characters", maxGuildNameLen)
	}

	var created dbgen.Guild
	err := s.tx.InTx(ctx, func(q *dbgen.Queries) error {
		guildID := uuid.Must(uuid.NewV7())

		g, err := q.CreateGuild(ctx, dbgen.CreateGuildParams{
			ID: guildID, Name: name, OwnerID: ownerID,
		})
		if err != nil {
			return err
		}

		if _, err := q.CreateRole(ctx, dbgen.CreateRoleParams{
			ID:          uuid.Must(uuid.NewV7()),
			GuildID:     guildID,
			Name:        "@everyone",
			Permissions: int64(domain.DefaultEveryonePermissions),
			Position:    0,
			IsDefault:   true,
		}); err != nil {
			return err
		}

		if _, err := q.AddGuildMember(ctx, dbgen.AddGuildMemberParams{
			GuildID: guildID, UserID: ownerID,
		}); err != nil {
			return err
		}

		for i, ch := range []struct {
			name string
			kind string
		}{
			{"general", domain.ChannelText},
			{"General", domain.ChannelVoice},
		} {
			if _, err := q.CreateChannel(ctx, dbgen.CreateChannelParams{
				ID:       uuid.Must(uuid.NewV7()),
				GuildID:  guildID,
				Kind:     ch.kind,
				Name:     ch.name,
				Position: int32(i),
			}); err != nil {
				return err
			}
		}

		created = g
		return nil
	})
	if err != nil {
		return dbgen.Guild{}, domain.Internal(err)
	}
	return created, nil
}

func (s *Service) ListForUser(ctx context.Context, userID uuid.UUID) ([]dbgen.Guild, error) {
	guilds, err := s.repo.ListGuildsForUser(ctx, userID)
	if err != nil {
		return nil, domain.Internal(err)
	}
	return guilds, nil
}

func (s *Service) ListChannels(ctx context.Context, userID, guildID uuid.UUID) ([]dbgen.Channel, error) {
	access, err := s.repo.ResolveGuildAccess(ctx, dbgen.ResolveGuildAccessParams{
		GuildID: guildID, UserID: userID,
	})
	if err != nil {
		if db.IsNoRows(err) {
			return nil, domain.NotFound("guild")
		}
		return nil, domain.Internal(err)
	}
	if !access.IsMember {
		return nil, domain.NotFound("guild")
	}
	roles, err := decodeRoles(access.Roles)
	if err != nil {
		return nil, err
	}

	channels, err := s.repo.ListChannels(ctx, guildID)
	if err != nil {
		return nil, domain.Internal(err)
	}
	rows, err := s.repo.ListGuildOverwrites(ctx, guildID)
	if err != nil {
		return nil, domain.Internal(err)
	}

	byChannel := make(map[uuid.UUID][]domain.Overwrite, len(rows))
	for _, o := range rows {
		byChannel[o.ChannelID] = append(byChannel[o.ChannelID], domain.Overwrite{
			TargetID:   o.TargetID,
			TargetType: o.TargetType,
			Allow:      domain.Permission(o.Allow),
			Deny:       domain.Permission(o.Deny),
		})
	}

	visible := make([]dbgen.Channel, 0, len(channels))
	for _, ch := range channels {
		if domain.ResolvePermissions(access.OwnerID, userID, roles, byChannel[ch.ID]).Has(domain.PermViewChannel) {
			visible = append(visible, ch)
		}
	}
	return visible, nil
}

func (s *Service) CreateChannel(ctx context.Context, userID, guildID uuid.UUID, name, kind string, position int32) (dbgen.Channel, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return dbgen.Channel{}, domain.Invalid("channel name is required")
	}
	if !domain.ValidChannelKind(kind) {
		return dbgen.Channel{}, domain.Invalid("kind must be one of text, voice, category")
	}

	perms, err := s.PermissionsInGuild(ctx, userID, guildID)
	if err != nil {
		return dbgen.Channel{}, err
	}
	if !perms.Has(domain.PermManageChannels) {
		return dbgen.Channel{}, domain.Forbidden("missing ManageChannels permission")
	}

	ch, err := s.repo.CreateChannel(ctx, dbgen.CreateChannelParams{
		ID:       uuid.Must(uuid.NewV7()),
		GuildID:  guildID,
		Kind:     kind,
		Name:     name,
		Position: position,
	})
	if err != nil {
		return dbgen.Channel{}, domain.Internal(err)
	}
	return ch, nil
}

func (s *Service) Channel(ctx context.Context, channelID uuid.UUID) (dbgen.Channel, error) {
	channel, err := s.repo.GetChannel(ctx, channelID)
	if err != nil {
		if db.IsNoRows(err) {
			return dbgen.Channel{}, domain.NotFound("channel")
		}
		return dbgen.Channel{}, domain.Internal(err)
	}
	return channel, nil
}

func (s *Service) MemberIDs(ctx context.Context, guildID uuid.UUID) ([]uuid.UUID, error) {
	ids, err := s.repo.ListGuildMemberIDs(ctx, guildID)
	if err != nil {
		return nil, domain.Internal(err)
	}
	return ids, nil
}

func (s *Service) Members(ctx context.Context, userID, guildID uuid.UUID) ([]dbgen.ListGuildMembersRow, error) {
	if _, err := s.requireMember(ctx, guildID, userID); err != nil {
		return nil, err
	}
	members, err := s.repo.ListGuildMembers(ctx, guildID)
	if err != nil {
		return nil, domain.Internal(err)
	}
	return members, nil
}

func (s *Service) actorInChannel(ctx context.Context, userID, channelID uuid.UUID) (domain.Actor, dbgen.Channel, error) {
	access, err := s.repo.ResolveChannelAccess(ctx, dbgen.ResolveChannelAccessParams{
		ChannelID: channelID, UserID: userID,
	})
	if err != nil {
		if db.IsNoRows(err) {
			return domain.Actor{}, dbgen.Channel{}, domain.NotFound("channel")
		}
		return domain.Actor{}, dbgen.Channel{}, domain.Internal(err)
	}
	if !access.IsMember {
		return domain.Actor{}, dbgen.Channel{}, domain.NotFound("guild")
	}

	roles, err := decodeRoles(access.Roles)
	if err != nil {
		return domain.Actor{}, dbgen.Channel{}, err
	}
	overwrites, err := decodeOverwrites(access.Overwrites)
	if err != nil {
		return domain.Actor{}, dbgen.Channel{}, err
	}

	return domain.NewActor(access.OwnerID, userID, roles, overwrites), access.Channel, nil
}

func (s *Service) actorInGuild(ctx context.Context, userID, guildID uuid.UUID) (domain.Actor, error) {
	access, err := s.repo.ResolveGuildAccess(ctx, dbgen.ResolveGuildAccessParams{
		GuildID: guildID, UserID: userID,
	})
	if err != nil {
		if db.IsNoRows(err) {
			return domain.Actor{}, domain.NotFound("guild")
		}
		return domain.Actor{}, domain.Internal(err)
	}
	if !access.IsMember {
		return domain.Actor{}, domain.NotFound("guild")
	}

	roles, err := decodeRoles(access.Roles)
	if err != nil {
		return domain.Actor{}, err
	}
	return domain.NewActor(access.OwnerID, userID, roles, nil), nil
}

func (s *Service) PermissionsIn(ctx context.Context, userID, channelID uuid.UUID) (domain.Permission, dbgen.Channel, error) {
	actor, channel, err := s.actorInChannel(ctx, userID, channelID)
	if err != nil {
		return 0, dbgen.Channel{}, err
	}
	return actor.Permissions, channel, nil
}

func (s *Service) PermissionsInGuild(ctx context.Context, userID, guildID uuid.UUID) (domain.Permission, error) {
	actor, err := s.actorInGuild(ctx, userID, guildID)
	if err != nil {
		return 0, err
	}
	return actor.Permissions, nil
}

func decodeRoles(raw string) ([]domain.RoleGrant, error) {
	var rows []struct {
		ID          uuid.UUID `json:"id"`
		Permissions int64     `json:"permissions"`
		Position    int32     `json:"position"`
	}
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		return nil, domain.Internal(err)
	}
	grants := make([]domain.RoleGrant, len(rows))
	for i, r := range rows {
		grants[i] = domain.RoleGrant{
			ID:          r.ID,
			Permissions: domain.Permission(r.Permissions),
			Position:    r.Position,
		}
	}
	return grants, nil
}

func decodeOverwrites(raw string) ([]domain.Overwrite, error) {
	var rows []struct {
		TargetID   uuid.UUID `json:"target_id"`
		TargetType string    `json:"target_type"`
		Allow      int64     `json:"allow"`
		Deny       int64     `json:"deny"`
	}
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		return nil, domain.Internal(err)
	}
	out := make([]domain.Overwrite, len(rows))
	for i, o := range rows {
		out[i] = domain.Overwrite{
			TargetID:   o.TargetID,
			TargetType: o.TargetType,
			Allow:      domain.Permission(o.Allow),
			Deny:       domain.Permission(o.Deny),
		}
	}
	return out, nil
}

func (s *Service) requireMember(ctx context.Context, guildID, userID uuid.UUID) (dbgen.GuildMember, error) {
	member, err := s.repo.GetGuildMember(ctx, dbgen.GetGuildMemberParams{
		GuildID: guildID, UserID: userID,
	})
	if err != nil {
		if db.IsNoRows(err) {
			return dbgen.GuildMember{}, domain.NotFound("guild")
		}
		return dbgen.GuildMember{}, domain.Internal(err)
	}
	return member, nil
}

func PublicGuild(g dbgen.Guild) events.Guild {
	return events.Guild{ID: g.ID, Name: g.Name, OwnerID: g.OwnerID, IconKey: g.IconKey}
}

func PublicRole(r dbgen.Role) events.Role {
	return events.Role{
		ID: r.ID, GuildID: r.GuildID, Name: r.Name,
		Permissions: r.Permissions, Position: r.Position, IsDefault: r.IsDefault,
	}
}

func PublicOverwrite(o dbgen.ChannelOverwrite) events.Overwrite {
	return events.Overwrite{
		ChannelID: o.ChannelID, TargetID: o.TargetID,
		TargetType: o.TargetType, Allow: o.Allow, Deny: o.Deny,
	}
}

func PublicChannel(c dbgen.Channel) events.Channel {
	return events.Channel{
		ID: c.ID, GuildID: c.GuildID, ParentID: c.ParentID,
		Kind: c.Kind, Name: c.Name, Topic: c.Topic, Position: c.Position,
	}
}
