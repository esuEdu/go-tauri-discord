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
	"github.com/esuEdu/go-tauri-discord/internal/platform/bus"
	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

type Repository interface {
	CreateGuild(ctx context.Context, arg dbgen.CreateGuildParams) (dbgen.Guild, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (dbgen.User, error)
	GetGuild(ctx context.Context, id uuid.UUID) (dbgen.Guild, error)
	SetGuildIcon(ctx context.Context, arg dbgen.SetGuildIconParams) (dbgen.Guild, error)
	ListGuildsForUser(ctx context.Context, userID uuid.UUID) ([]dbgen.Guild, error)
	CountGuildsForUser(ctx context.Context, userID uuid.UUID) (int64, error)
	CreateChannel(ctx context.Context, arg dbgen.CreateChannelParams) (dbgen.Channel, error)
	GetChannel(ctx context.Context, id uuid.UUID) (dbgen.Channel, error)
	ListChannels(ctx context.Context, guildID uuid.UUID) ([]dbgen.Channel, error)
	SetChannelPosition(ctx context.Context, arg dbgen.SetChannelPositionParams) error
	SetChannelParent(ctx context.Context, arg dbgen.SetChannelParentParams) error
	AddGuildMember(ctx context.Context, arg dbgen.AddGuildMemberParams) (dbgen.GuildMember, error)
	GetGuildMember(ctx context.Context, arg dbgen.GetGuildMemberParams) (dbgen.GuildMember, error)
	RemoveGuildMember(ctx context.Context, arg dbgen.RemoveGuildMemberParams) error
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
	GetGuildBan(ctx context.Context, arg dbgen.GetGuildBanParams) (dbgen.GuildBan, error)
	ListGuildBans(ctx context.Context, guildID uuid.UUID) ([]dbgen.ListGuildBansRow, error)
	DeleteGuildBan(ctx context.Context, arg dbgen.DeleteGuildBanParams) (int64, error)
}

type TxRunner interface {
	InTx(ctx context.Context, fn func(q *dbgen.Queries) error) error
}

type Service struct {
	repo    Repository
	invites InviteRepository
	tx      TxRunner
	pub     *bus.Publisher
}

func NewService(repo Repository, invites InviteRepository, tx TxRunner, pub *bus.Publisher) *Service {
	return &Service{repo: repo, invites: invites, tx: tx, pub: pub}
}

func (s *Service) permissionsChanged(ctx context.Context, guildID uuid.UUID) {
	if s.pub != nil {
		s.pub.PermissionsChanged(ctx, guildID)
	}
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

func (s *Service) Reorder(ctx context.Context, userID uuid.UUID, order []uuid.UUID) error {
	held, err := s.repo.CountGuildsForUser(ctx, userID)
	if err != nil {
		return domain.Internal(err)
	}
	if int64(len(order)) != held {
		return domain.Invalid("the order must name every server you are in, exactly once")
	}

	seen := make(map[uuid.UUID]bool, len(order))
	for _, id := range order {
		if seen[id] {
			return domain.Invalid("the order names %s twice", id)
		}
		seen[id] = true
	}

	err = s.tx.InTx(ctx, func(q *dbgen.Queries) error {
		for at, id := range order {
			if _, err := q.GetGuildMember(ctx, dbgen.GetGuildMemberParams{
				GuildID: id, UserID: userID,
			}); err != nil {
				if db.IsNoRows(err) {
					return domain.NotFound("guild")
				}
				return err
			}
			if err := q.SetGuildPosition(ctx, dbgen.SetGuildPositionParams{
				UserID: userID, GuildID: id, Position: int32(at),
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		if domain.KindOf(err) == domain.KindNotFound {
			return err
		}
		return domain.Internal(err)
	}
	return nil
}

func (s *Service) ListChannels(ctx context.Context, userID, guildID uuid.UUID) ([]dbgen.Channel, error) {
	visible, _, err := s.PartitionChannels(ctx, userID, guildID)
	return visible, err
}

type Access struct {
	Visible   []dbgen.Channel
	Hidden    []uuid.UUID
	Guild     domain.Permission
	ByChannel map[uuid.UUID]domain.Permission
}

func (s *Service) PartitionChannels(ctx context.Context, userID, guildID uuid.UUID) (visible []dbgen.Channel, hidden []uuid.UUID, err error) {
	access, err := s.ResolveAccess(ctx, userID, guildID)
	if err != nil {
		return nil, nil, err
	}
	return access.Visible, access.Hidden, nil
}

func (s *Service) ResolveAccess(ctx context.Context, userID, guildID uuid.UUID) (Access, error) {
	access, err := s.repo.ResolveGuildAccess(ctx, dbgen.ResolveGuildAccessParams{
		GuildID: guildID, UserID: userID,
	})
	if err != nil {
		if db.IsNoRows(err) {
			return Access{}, domain.NotFound("guild")
		}
		return Access{}, domain.Internal(err)
	}
	if !access.IsMember {
		return Access{}, domain.NotFound("guild")
	}
	roles, err := decodeRoles(access.Roles)
	if err != nil {
		return Access{}, err
	}

	channels, err := s.repo.ListChannels(ctx, guildID)
	if err != nil {
		return Access{}, domain.Internal(err)
	}
	rows, err := s.repo.ListGuildOverwrites(ctx, guildID)
	if err != nil {
		return Access{}, domain.Internal(err)
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

	out := Access{
		Visible:   make([]dbgen.Channel, 0, len(channels)),
		Hidden:    make([]uuid.UUID, 0),
		Guild:     domain.ResolvePermissions(access.OwnerID, userID, roles, nil),
		ByChannel: make(map[uuid.UUID]domain.Permission, len(channels)),
	}
	for _, ch := range channels {
		perms := domain.ResolvePermissions(access.OwnerID, userID, roles, byChannel[ch.ID])
		out.ByChannel[ch.ID] = perms
		if perms.Has(domain.PermViewChannel) {
			out.Visible = append(out.Visible, ch)
		} else {
			out.Hidden = append(out.Hidden, ch.ID)
		}
	}
	return out, nil
}

func (s *Service) CreateChannel(ctx context.Context, userID, guildID uuid.UUID, name, kind string, position int32, parentID *uuid.UUID) (dbgen.Channel, error) {
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

	if parentID != nil {
		if kind == domain.ChannelCategory {
			return dbgen.Channel{}, domain.Invalid("a category cannot sit inside another")
		}
		parent, err := s.repo.GetChannel(ctx, *parentID)
		if err != nil {
			return dbgen.Channel{}, domain.Invalid("no such category")
		}
		if parent.GuildID != guildID || parent.Kind != domain.ChannelCategory {
			return dbgen.Channel{}, domain.Invalid("that is not a category in this server")
		}
	}

	ch, err := s.repo.CreateChannel(ctx, dbgen.CreateChannelParams{
		ID:       uuid.Must(uuid.NewV7()),
		GuildID:  guildID,
		ParentID: parentID,
		Kind:     kind,
		Name:     name,
		Position: position,
	})
	if err != nil {
		return dbgen.Channel{}, domain.Internal(err)
	}
	s.permissionsChanged(ctx, guildID)
	return ch, nil
}

func (s *Service) MoveChannel(
	ctx context.Context,
	userID, channelID uuid.UUID,
	to int,
	parent *uuid.UUID,
	reparent bool,
) ([]dbgen.Channel, error) {
	moving, err := s.Channel(ctx, channelID)
	if err != nil {
		return nil, err
	}

	perms, err := s.PermissionsInGuild(ctx, userID, moving.GuildID)
	if err != nil {
		return nil, err
	}
	if !perms.Has(domain.PermManageChannels) {
		return nil, domain.Forbidden("missing ManageChannels permission")
	}

	all, err := s.repo.ListChannels(ctx, moving.GuildID)
	if err != nil {
		return nil, domain.Internal(err)
	}

	if reparent {
		if err := s.canParent(moving, parent, all); err != nil {
			return nil, err
		}
	} else {
		parent = moving.ParentID
	}

	siblings := make([]dbgen.Channel, 0, len(all))
	for _, ch := range all {
		if ch.ID == moving.ID {
			continue
		}
		if under(ch, parent) {
			siblings = append(siblings, ch)
		}
	}

	if to < 0 {
		to = 0
	}
	if to > len(siblings) {
		to = len(siblings)
	}
	ordered := make([]dbgen.Channel, 0, len(siblings)+1)
	ordered = append(ordered, siblings[:to]...)
	ordered = append(ordered, moving)
	ordered = append(ordered, siblings[to:]...)

	changed := make([]dbgen.Channel, 0, len(ordered))
	err = s.tx.InTx(ctx, func(q *dbgen.Queries) error {
		if reparent && !samePointer(moving.ParentID, parent) {
			if err := q.SetChannelParent(ctx, dbgen.SetChannelParentParams{
				ID: moving.ID, GuildID: moving.GuildID, ParentID: parent,
			}); err != nil {
				return err
			}
			moving.ParentID = parent
			for at := range ordered {
				if ordered[at].ID == moving.ID {
					ordered[at].ParentID = parent
				}
			}
		}
		for at, ch := range ordered {
			if ch.Position == int32(at) {
				continue
			}
			if err := q.SetChannelPosition(ctx, dbgen.SetChannelPositionParams{
				ID:       ch.ID,
				GuildID:  moving.GuildID,
				Position: int32(at),
			}); err != nil {
				return err
			}
			ch.Position = int32(at)
			changed = append(changed, ch)
		}
		return nil
	})
	if err != nil {
		return nil, domain.Internal(err)
	}
	return changed, nil
}

func under(ch dbgen.Channel, parent *uuid.UUID) bool {
	if parent == nil {
		return ch.ParentID == nil
	}
	return ch.ParentID != nil && *ch.ParentID == *parent
}

func samePointer(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func (s *Service) canParent(moving dbgen.Channel, parent *uuid.UUID, all []dbgen.Channel) error {
	if parent == nil {
		return nil
	}
	if moving.Kind == "category" {
		return domain.Invalid("a category cannot sit inside another category")
	}
	for _, ch := range all {
		if ch.ID != *parent {
			continue
		}
		if ch.Kind != "category" {
			return domain.Invalid("a channel can only be put inside a category")
		}
		return nil
	}
	return domain.NotFound("category")
}

func sameParent(a, b dbgen.Channel) bool {
	if a.ParentID == nil || b.ParentID == nil {
		return a.ParentID == nil && b.ParentID == nil
	}
	return *a.ParentID == *b.ParentID
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

func (s *Service) NewMember(ctx context.Context, guildID, userID uuid.UUID) (events.Member, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return events.Member{}, domain.Internal(err)
	}
	return events.Member{
		GuildID: guildID,
		User: events.User{
			ID: user.ID, Username: user.Username,
			Discriminator: user.Discriminator, AvatarKey: user.AvatarKey,
		},
	}, nil
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

func (s *Service) Icon(ctx context.Context, guildID uuid.UUID) (*string, error) {
	guild, err := s.repo.GetGuild(ctx, guildID)
	if err != nil {
		return nil, domain.NotFound("guild")
	}
	return guild.IconKey, nil
}

func (s *Service) SetIcon(ctx context.Context, actorID, guildID uuid.UUID, key *string) error {
	perms, err := s.PermissionsInGuild(ctx, actorID, guildID)
	if err != nil {
		return err
	}
	if !perms.Has(domain.PermManageGuild) {
		return domain.Forbidden("missing ManageGuild permission")
	}

	if _, err := s.repo.SetGuildIcon(ctx, dbgen.SetGuildIconParams{
		ID: guildID, IconKey: key,
	}); err != nil {
		return domain.Internal(err)
	}
	return nil
}
