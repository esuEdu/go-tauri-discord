package guild

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/esuEdu/go-tauri-discord/internal/db"
	dbgen "github.com/esuEdu/go-tauri-discord/internal/db/gen"
	"github.com/esuEdu/go-tauri-discord/internal/domain"
	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

type Profile struct {
	User     events.User   `json:"user"`
	GuildID  uuid.UUID     `json:"guild_id"`
	Nickname *string       `json:"nickname"`
	JoinedAt time.Time     `json:"joined_at"`
	Bio      *string       `json:"bio"`
	Roles    []events.Role `json:"roles"`
}

func (s *Service) MemberProfile(ctx context.Context, viewerID, guildID, memberID uuid.UUID) (Profile, error) {
	if _, err := s.actorInGuild(ctx, viewerID, guildID); err != nil {
		return Profile{}, err
	}

	row, err := s.repo.GetGuildMemberProfile(ctx, dbgen.GetGuildMemberProfileParams{
		GuildID: guildID, UserID: memberID,
	})
	if err != nil {
		if db.IsNoRows(err) {
			return Profile{}, domain.NotFound("member")
		}
		return Profile{}, domain.Internal(err)
	}

	roles, err := s.repo.ListMemberRoles(ctx, dbgen.ListMemberRolesParams{
		GuildID: guildID, UserID: memberID,
	})
	if err != nil {
		return Profile{}, domain.Internal(err)
	}

	return Profile{
		User: events.User{
			ID:            events.UserID(row.PublicID),
			Username:      row.Username,
			Discriminator: row.Discriminator,
			AvatarKey:     row.AvatarKey,
		},
		GuildID:  guildID,
		Nickname: row.Nickname,
		JoinedAt: row.JoinedAt,
		Bio:      row.Bio,
		Roles:    mapSlice(roles, PublicRole),
	}, nil
}
