package guild

import (
	"context"
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
	ListEffectiveRoles(ctx context.Context, arg dbgen.ListEffectiveRolesParams) ([]dbgen.Role, error)
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
	if _, err := s.requireMember(ctx, guildID, userID); err != nil {
		return nil, err
	}
	channels, err := s.repo.ListChannels(ctx, guildID)
	if err != nil {
		return nil, domain.Internal(err)
	}

	guild, err := s.repo.GetGuild(ctx, guildID)
	if err != nil {
		return nil, domain.Internal(err)
	}
	roles, err := s.effectiveRoles(ctx, guildID, userID)
	if err != nil {
		return nil, err
	}

	visible := make([]dbgen.Channel, 0, len(channels))
	for _, ch := range channels {
		overwrites, err := s.overwrites(ctx, ch.ID)
		if err != nil {
			return nil, err
		}
		if domain.ResolvePermissions(guild.OwnerID, userID, roles, overwrites).Has(domain.PermViewChannel) {
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

func (s *Service) PermissionsIn(ctx context.Context, userID, channelID uuid.UUID) (domain.Permission, dbgen.Channel, error) {
	channel, err := s.repo.GetChannel(ctx, channelID)
	if err != nil {
		if db.IsNoRows(err) {
			return 0, dbgen.Channel{}, domain.NotFound("channel")
		}
		return 0, dbgen.Channel{}, domain.Internal(err)
	}

	guild, err := s.repo.GetGuild(ctx, channel.GuildID)
	if err != nil {
		return 0, dbgen.Channel{}, domain.Internal(err)
	}
	if _, err := s.requireMember(ctx, channel.GuildID, userID); err != nil {
		return 0, dbgen.Channel{}, err
	}

	roles, err := s.effectiveRoles(ctx, channel.GuildID, userID)
	if err != nil {
		return 0, dbgen.Channel{}, err
	}
	overwrites, err := s.overwrites(ctx, channelID)
	if err != nil {
		return 0, dbgen.Channel{}, err
	}

	return domain.ResolvePermissions(guild.OwnerID, userID, roles, overwrites), channel, nil
}

func (s *Service) PermissionsInGuild(ctx context.Context, userID, guildID uuid.UUID) (domain.Permission, error) {
	guild, err := s.repo.GetGuild(ctx, guildID)
	if err != nil {
		if db.IsNoRows(err) {
			return 0, domain.NotFound("guild")
		}
		return 0, domain.Internal(err)
	}
	if _, err := s.requireMember(ctx, guildID, userID); err != nil {
		return 0, err
	}
	roles, err := s.effectiveRoles(ctx, guildID, userID)
	if err != nil {
		return 0, err
	}
	return domain.ResolvePermissions(guild.OwnerID, userID, roles, nil), nil
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

func (s *Service) effectiveRoles(ctx context.Context, guildID, userID uuid.UUID) ([]domain.RoleGrant, error) {
	rows, err := s.repo.ListEffectiveRoles(ctx, dbgen.ListEffectiveRolesParams{
		GuildID: guildID, UserID: userID,
	})
	if err != nil {
		return nil, domain.Internal(err)
	}
	grants := make([]domain.RoleGrant, len(rows))
	for i, r := range rows {
		grants[i] = domain.RoleGrant{ID: r.ID, Permissions: domain.Permission(r.Permissions)}
	}
	return grants, nil
}

func (s *Service) overwrites(ctx context.Context, channelID uuid.UUID) ([]domain.Overwrite, error) {
	rows, err := s.repo.ListChannelOverwrites(ctx, channelID)
	if err != nil {
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

func PublicGuild(g dbgen.Guild) events.Guild {
	return events.Guild{ID: g.ID, Name: g.Name, OwnerID: g.OwnerID, IconKey: g.IconKey}
}

func PublicChannel(c dbgen.Channel) events.Channel {
	return events.Channel{
		ID: c.ID, GuildID: c.GuildID, ParentID: c.ParentID,
		Kind: c.Kind, Name: c.Name, Topic: c.Topic, Position: c.Position,
	}
}
