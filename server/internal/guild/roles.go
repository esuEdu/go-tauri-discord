package guild

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/esuEdu/go-tauri-discord/internal/db"
	dbgen "github.com/esuEdu/go-tauri-discord/internal/db/gen"
	"github.com/esuEdu/go-tauri-discord/internal/domain"
)

const (
	maxRoleNameLen  = 100
	maxRolePosition = 1 << 16
)

func (s *Service) ListRoles(ctx context.Context, userID, guildID uuid.UUID) ([]dbgen.Role, error) {
	if _, err := s.actorInGuild(ctx, userID, guildID); err != nil {
		return nil, err
	}
	roles, err := s.repo.ListRoles(ctx, guildID)
	if err != nil {
		return nil, domain.Internal(err)
	}
	return roles, nil
}

func (s *Service) MemberRoles(ctx context.Context, userID, guildID, memberID uuid.UUID) ([]dbgen.Role, error) {
	if _, err := s.actorInGuild(ctx, userID, guildID); err != nil {
		return nil, err
	}
	if _, err := s.requireMember(ctx, guildID, memberID); err != nil {
		return nil, domain.NotFound("member")
	}
	roles, err := s.repo.ListMemberRoles(ctx, dbgen.ListMemberRolesParams{
		GuildID: guildID, UserID: memberID,
	})
	if err != nil {
		return nil, domain.Internal(err)
	}
	return roles, nil
}

func (s *Service) CreateRole(ctx context.Context, userID, guildID uuid.UUID, name string, perms domain.Permission, position *int32) (dbgen.Role, error) {
	name, err := validRoleName(name)
	if err != nil {
		return dbgen.Role{}, err
	}
	if err := knownPermissions(perms); err != nil {
		return dbgen.Role{}, err
	}
	at := domain.EveryonePosition + 1
	if position != nil {
		at = *position
	}
	if err := validPosition(at); err != nil {
		return dbgen.Role{}, err
	}

	actor, err := s.roleManagerIn(ctx, userID, guildID)
	if err != nil {
		return dbgen.Role{}, err
	}
	if !actor.Outranks(at) {
		return dbgen.Role{}, domain.Forbidden("cannot create a role at or above your highest role")
	}
	if !actor.CanGrant(perms) {
		return dbgen.Role{}, domain.Forbidden("cannot grant a permission you do not have")
	}

	role, err := s.repo.CreateRole(ctx, dbgen.CreateRoleParams{
		ID:          uuid.Must(uuid.NewV7()),
		GuildID:     guildID,
		Name:        name,
		Permissions: int64(perms),
		Position:    at,
	})
	if err != nil {
		return dbgen.Role{}, domain.Internal(err)
	}
	s.permissionsChanged(ctx, guildID)
	return role, nil
}

func (s *Service) UpdateRole(ctx context.Context, userID, roleID uuid.UUID, name *string, perms *domain.Permission, position *int32) (dbgen.Role, error) {
	role, actor, err := s.roleUnderActor(ctx, userID, roleID)
	if err != nil {
		return dbgen.Role{}, err
	}

	arg := dbgen.UpdateRoleParams{ID: roleID}

	if name != nil {
		if role.IsDefault {
			return dbgen.Role{}, domain.Invalid("the @everyone role cannot be renamed")
		}
		clean, err := validRoleName(*name)
		if err != nil {
			return dbgen.Role{}, err
		}
		arg.Name = &clean
	}

	if position != nil {
		if role.IsDefault {
			return dbgen.Role{}, domain.Invalid("the @everyone role cannot be moved")
		}
		if err := validPosition(*position); err != nil {
			return dbgen.Role{}, err
		}
		if !actor.Outranks(*position) {
			return dbgen.Role{}, domain.Forbidden("cannot move a role to or above your highest role")
		}
		arg.Position = position
	}

	if perms != nil {
		if err := knownPermissions(*perms); err != nil {
			return dbgen.Role{}, err
		}
		changed := domain.Permission(role.Permissions) ^ *perms
		if !actor.CanGrant(changed) {
			return dbgen.Role{}, domain.Forbidden("cannot change a permission you do not have")
		}
		raw := int64(*perms)
		arg.Permissions = &raw
	}

	updated, err := s.repo.UpdateRole(ctx, arg)
	if err != nil {
		return dbgen.Role{}, domain.Internal(err)
	}
	s.permissionsChanged(ctx, role.GuildID)
	return updated, nil
}

func (s *Service) DeleteRole(ctx context.Context, userID, roleID uuid.UUID) error {
	role, _, err := s.roleUnderActor(ctx, userID, roleID)
	if err != nil {
		return err
	}
	if role.IsDefault {
		return domain.Invalid("the @everyone role cannot be deleted")
	}

	err = s.tx.InTx(ctx, func(q *dbgen.Queries) error {
		if err := q.DeleteOverwritesForTarget(ctx, dbgen.DeleteOverwritesForTargetParams{
			GuildID: role.GuildID, TargetID: roleID,
		}); err != nil {
			return err
		}
		return q.DeleteRole(ctx, roleID)
	})
	if err != nil {
		return domain.Internal(err)
	}
	s.permissionsChanged(ctx, role.GuildID)
	return nil
}

func (s *Service) AssignRole(ctx context.Context, userID, guildID, memberID, roleID uuid.UUID) error {
	role, _, err := s.assignableRole(ctx, userID, guildID, memberID, roleID)
	if err != nil {
		return err
	}
	if err := s.repo.AssignRole(ctx, dbgen.AssignRoleParams{
		GuildID: guildID, UserID: memberID, RoleID: role.ID,
	}); err != nil {
		return domain.Internal(err)
	}
	s.permissionsChanged(ctx, guildID)
	return nil
}

func (s *Service) UnassignRole(ctx context.Context, userID, guildID, memberID, roleID uuid.UUID) error {
	role, _, err := s.assignableRole(ctx, userID, guildID, memberID, roleID)
	if err != nil {
		return err
	}
	if err := s.repo.UnassignRole(ctx, dbgen.UnassignRoleParams{
		GuildID: guildID, UserID: memberID, RoleID: role.ID,
	}); err != nil {
		return domain.Internal(err)
	}
	s.permissionsChanged(ctx, guildID)
	return nil
}

func (s *Service) ChannelOverwrites(ctx context.Context, userID, channelID uuid.UUID) ([]dbgen.ChannelOverwrite, error) {
	if _, _, err := s.actorInChannel(ctx, userID, channelID); err != nil {
		return nil, err
	}
	rows, err := s.repo.ListChannelOverwrites(ctx, channelID)
	if err != nil {
		return nil, domain.Internal(err)
	}
	return rows, nil
}

func (s *Service) SetChannelOverwrite(ctx context.Context, userID, channelID, targetID uuid.UUID, targetType string, allow, deny domain.Permission) error {
	if targetType != domain.OverwriteRole && targetType != domain.OverwriteMember {
		return domain.Invalid("target_type must be one of role, member")
	}
	if err := knownPermissions(allow | deny); err != nil {
		return err
	}
	if allow&deny != 0 {
		return domain.Invalid("a permission cannot be both allowed and denied")
	}
	if (allow|deny)&domain.PermAdministrator != 0 {
		return domain.Invalid("Administrator cannot be set on a channel overwrite")
	}

	actor, channel, err := s.actorInChannel(ctx, userID, channelID)
	if err != nil {
		return err
	}
	if !actor.Permissions.Has(domain.PermManageRoles) {
		return domain.Forbidden("missing ManageRoles permission")
	}
	if !actor.CanGrant(allow | deny) {
		return domain.Forbidden("cannot change a permission you do not have")
	}

	switch targetType {
	case domain.OverwriteRole:
		role, err := s.guildRole(ctx, channel.GuildID, targetID)
		if err != nil {
			return err
		}
		if !actor.Outranks(role.Position) {
			return domain.Forbidden("cannot overwrite a role at or above your highest role")
		}
	case domain.OverwriteMember:
		if _, err := s.requireMember(ctx, channel.GuildID, targetID); err != nil {
			return domain.NotFound("member")
		}
	}

	if err := s.repo.UpsertChannelOverwrite(ctx, dbgen.UpsertChannelOverwriteParams{
		ChannelID:  channelID,
		TargetID:   targetID,
		TargetType: targetType,
		Allow:      int64(allow),
		Deny:       int64(deny),
	}); err != nil {
		return domain.Internal(err)
	}
	s.permissionsChanged(ctx, channel.GuildID)
	return nil
}

func (s *Service) ClearChannelOverwrite(ctx context.Context, userID, channelID, targetID uuid.UUID) error {
	actor, channel, err := s.actorInChannel(ctx, userID, channelID)
	if err != nil {
		return err
	}
	if !actor.Permissions.Has(domain.PermManageRoles) {
		return domain.Forbidden("missing ManageRoles permission")
	}

	if role, err := s.guildRole(ctx, channel.GuildID, targetID); err == nil {
		if !actor.Outranks(role.Position) {
			return domain.Forbidden("cannot overwrite a role at or above your highest role")
		}
	} else if domain.KindOf(err) != domain.KindNotFound {
		return err
	}

	if err := s.repo.DeleteChannelOverwrite(ctx, dbgen.DeleteChannelOverwriteParams{
		ChannelID: channelID, TargetID: targetID,
	}); err != nil {
		return domain.Internal(err)
	}
	s.permissionsChanged(ctx, channel.GuildID)
	return nil
}

func (s *Service) roleManagerIn(ctx context.Context, userID, guildID uuid.UUID) (domain.Actor, error) {
	actor, err := s.actorInGuild(ctx, userID, guildID)
	if err != nil {
		return domain.Actor{}, err
	}
	if !actor.Permissions.Has(domain.PermManageRoles) {
		return domain.Actor{}, domain.Forbidden("missing ManageRoles permission")
	}
	return actor, nil
}

func (s *Service) roleUnderActor(ctx context.Context, userID, roleID uuid.UUID) (dbgen.Role, domain.Actor, error) {
	role, err := s.repo.GetRole(ctx, roleID)
	if err != nil {
		if db.IsNoRows(err) {
			return dbgen.Role{}, domain.Actor{}, domain.NotFound("role")
		}
		return dbgen.Role{}, domain.Actor{}, domain.Internal(err)
	}

	actor, err := s.roleManagerIn(ctx, userID, role.GuildID)
	if err != nil {
		return dbgen.Role{}, domain.Actor{}, err
	}
	if !actor.Outranks(role.Position) {
		return dbgen.Role{}, domain.Actor{}, domain.Forbidden("cannot manage a role at or above your highest role")
	}
	return role, actor, nil
}

func (s *Service) assignableRole(ctx context.Context, userID, guildID, memberID, roleID uuid.UUID) (dbgen.Role, domain.Actor, error) {
	role, actor, err := s.roleUnderActor(ctx, userID, roleID)
	if err != nil {
		return dbgen.Role{}, domain.Actor{}, err
	}
	if role.GuildID != guildID {
		return dbgen.Role{}, domain.Actor{}, domain.NotFound("role")
	}
	if role.IsDefault {
		return dbgen.Role{}, domain.Actor{}, domain.Invalid("every member already holds @everyone")
	}
	if _, err := s.requireMember(ctx, guildID, memberID); err != nil {
		return dbgen.Role{}, domain.Actor{}, domain.NotFound("member")
	}
	return role, actor, nil
}

func (s *Service) guildRole(ctx context.Context, guildID, roleID uuid.UUID) (dbgen.Role, error) {
	role, err := s.repo.GetRole(ctx, roleID)
	if err != nil {
		if db.IsNoRows(err) {
			return dbgen.Role{}, domain.NotFound("role")
		}
		return dbgen.Role{}, domain.Internal(err)
	}
	if role.GuildID != guildID {
		return dbgen.Role{}, domain.NotFound("role")
	}
	return role, nil
}

func validRoleName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if n := utf8.RuneCountInString(name); n < 1 || n > maxRoleNameLen {
		return "", domain.Invalid("role name must be 1-%d characters", maxRoleNameLen)
	}
	return name, nil
}

func validPosition(position int32) error {
	if position <= domain.EveryonePosition || position > maxRolePosition {
		return domain.Invalid("position must be between %d and %d", domain.EveryonePosition+1, maxRolePosition)
	}
	return nil
}

func knownPermissions(perms domain.Permission) error {
	if perms&^domain.PermAll != 0 {
		return domain.Invalid("permissions contain bits that do not exist")
	}
	return nil
}
