package guild

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/esuEdu/go-tauri-discord/internal/db"
	dbgen "github.com/esuEdu/go-tauri-discord/internal/db/gen"
	"github.com/esuEdu/go-tauri-discord/internal/domain"
)

const maxBanReasonLen = 500

type BanView struct {
	GuildID   uuid.UUID  `json:"guild_id"`
	UserID    uuid.UUID  `json:"user_id"`
	Username  string     `json:"username"`
	BannedBy  *uuid.UUID `json:"banned_by"`
	Reason    *string    `json:"reason"`
	CreatedAt time.Time  `json:"created_at"`
}

func (s *Service) Kick(ctx context.Context, actorID, guildID, targetID uuid.UUID) error {
	member, err := s.removes(ctx, actorID, guildID, targetID, domain.PermKickMembers, "KickMembers")
	if err != nil {
		return err
	}
	if !member {
		return domain.NotFound("member")
	}
	if err := s.repo.RemoveGuildMember(ctx, dbgen.RemoveGuildMemberParams{
		GuildID: guildID, UserID: targetID,
	}); err != nil {
		return domain.Internal(err)
	}
	return nil
}

func (s *Service) Leave(ctx context.Context, userID, guildID uuid.UUID) error {
	guild, err := s.repo.GetGuild(ctx, guildID)
	if err != nil {
		if db.IsNoRows(err) {
			return domain.NotFound("guild")
		}
		return domain.Internal(err)
	}
	if guild.OwnerID == userID {
		return domain.Forbidden("the owner of a server cannot leave it")
	}

	if _, err := s.actorInGuild(ctx, userID, guildID); err != nil {
		return err
	}

	if err := s.repo.RemoveGuildMember(ctx, dbgen.RemoveGuildMemberParams{
		GuildID: guildID, UserID: userID,
	}); err != nil {
		return domain.Internal(err)
	}
	return nil
}

func (s *Service) Ban(ctx context.Context, actorID, guildID, targetID uuid.UUID, reason *string) (BanView, error) {
	clean, err := validBanReason(reason)
	if err != nil {
		return BanView{}, err
	}
	if _, err := s.removes(ctx, actorID, guildID, targetID, domain.PermBanMembers, "BanMembers"); err != nil {
		return BanView{}, err
	}

	user, err := s.repo.GetUserByID(ctx, targetID)
	if err != nil {
		if db.IsNoRows(err) {
			return BanView{}, domain.NotFound("user")
		}
		return BanView{}, domain.Internal(err)
	}

	var banned dbgen.GuildBan
	err = s.tx.InTx(ctx, func(q *dbgen.Queries) error {
		if err := q.RemoveGuildMember(ctx, dbgen.RemoveGuildMemberParams{
			GuildID: guildID, UserID: targetID,
		}); err != nil {
			return err
		}
		var err error
		banned, err = q.BanUser(ctx, dbgen.BanUserParams{
			GuildID: guildID, UserID: targetID, BannedBy: &actorID, Reason: clean,
		})
		return err
	})
	if err != nil {
		return BanView{}, domain.Internal(err)
	}
	return toBanView(banned, user.Username), nil
}

func (s *Service) Unban(ctx context.Context, actorID, guildID, targetID uuid.UUID) error {
	if err := s.banManagerIn(ctx, actorID, guildID); err != nil {
		return err
	}
	lifted, err := s.repo.DeleteGuildBan(ctx, dbgen.DeleteGuildBanParams{
		GuildID: guildID, UserID: targetID,
	})
	if err != nil {
		return domain.Internal(err)
	}
	if lifted == 0 {
		return domain.NotFound("ban")
	}
	return nil
}

func (s *Service) ListBans(ctx context.Context, actorID, guildID uuid.UUID) ([]BanView, error) {
	if err := s.banManagerIn(ctx, actorID, guildID); err != nil {
		return nil, err
	}
	rows, err := s.repo.ListGuildBans(ctx, guildID)
	if err != nil {
		return nil, domain.Internal(err)
	}

	out := make([]BanView, len(rows))
	for i, b := range rows {
		out[i] = BanView{
			GuildID:   b.GuildID,
			UserID:    b.UserID,
			Username:  b.Username,
			BannedBy:  b.BannedBy,
			Reason:    b.Reason,
			CreatedAt: b.CreatedAt,
		}
	}
	return out, nil
}

func (s *Service) Banned(ctx context.Context, guildID, userID uuid.UUID) (bool, error) {
	if _, err := s.repo.GetGuildBan(ctx, dbgen.GetGuildBanParams{
		GuildID: guildID, UserID: userID,
	}); err != nil {
		if db.IsNoRows(err) {
			return false, nil
		}
		return false, domain.Internal(err)
	}
	return true, nil
}

func (s *Service) removes(ctx context.Context, actorID, guildID, targetID uuid.UUID, needs domain.Permission, missing string) (bool, error) {
	if targetID == actorID {
		return false, domain.Invalid("you cannot remove yourself from a server")
	}

	actor, err := s.actorInGuild(ctx, actorID, guildID)
	if err != nil {
		return false, err
	}
	if !actor.Permissions.Has(needs) {
		return false, domain.Forbidden("missing " + missing + " permission")
	}

	guild, err := s.repo.GetGuild(ctx, guildID)
	if err != nil {
		if db.IsNoRows(err) {
			return false, domain.NotFound("guild")
		}
		return false, domain.Internal(err)
	}
	if guild.OwnerID == targetID {
		return false, domain.Forbidden("the owner of a server cannot be removed from it")
	}

	target, err := s.actorInGuild(ctx, targetID, guildID)
	if err != nil {
		if domain.KindOf(err) == domain.KindNotFound {
			return false, nil
		}
		return false, err
	}
	if !actor.Outranks(target.Highest) {
		return false, domain.Forbidden("cannot remove somebody at or above your highest role")
	}
	return true, nil
}

func (s *Service) banManagerIn(ctx context.Context, actorID, guildID uuid.UUID) error {
	actor, err := s.actorInGuild(ctx, actorID, guildID)
	if err != nil {
		return err
	}
	if !actor.Permissions.Has(domain.PermBanMembers) {
		return domain.Forbidden("missing BanMembers permission")
	}
	return nil
}

func validBanReason(reason *string) (*string, error) {
	if reason == nil {
		return nil, nil
	}
	clean := strings.TrimSpace(*reason)
	if clean == "" {
		return nil, nil
	}
	if utf8.RuneCountInString(clean) > maxBanReasonLen {
		return nil, domain.Invalid("a reason must be at most %d characters", maxBanReasonLen)
	}
	return &clean, nil
}

func toBanView(ban dbgen.GuildBan, username string) BanView {
	return BanView{
		GuildID:   ban.GuildID,
		UserID:    ban.UserID,
		Username:  username,
		BannedBy:  ban.BannedBy,
		Reason:    ban.Reason,
		CreatedAt: ban.CreatedAt,
	}
}
