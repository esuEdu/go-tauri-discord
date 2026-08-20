package domain

import "github.com/google/uuid"

type Permission int64

const (
	PermViewChannel Permission = 1 << iota
	PermSendMessages
	PermManageMessages
	PermManageChannels
	PermManageRoles
	PermKickMembers
	PermBanMembers
	PermConnect
	PermSpeak
	PermStream
	PermAdministrator
	PermCreateInvite
	PermManageGuild
)

const PermAll = PermViewChannel | PermSendMessages | PermManageMessages |
	PermManageChannels | PermManageRoles | PermKickMembers | PermBanMembers |
	PermConnect | PermSpeak | PermStream | PermAdministrator |
	PermCreateInvite | PermManageGuild

const DefaultEveryonePermissions = PermViewChannel | PermSendMessages |
	PermConnect | PermSpeak | PermStream | PermCreateInvite

func (p Permission) Has(want Permission) bool {
	if p&PermAdministrator != 0 {
		return true
	}
	return p&want == want
}

type RoleGrant struct {
	ID          uuid.UUID
	Permissions Permission
	Position    int32
}

type Overwrite struct {
	TargetID   uuid.UUID
	TargetType string
	Allow      Permission
	Deny       Permission
}

const (
	OverwriteRole   = "role"
	OverwriteMember = "member"
)

func ResolvePermissions(ownerID, userID uuid.UUID, roles []RoleGrant, overwrites []Overwrite) Permission {
	if ownerID == userID {
		return PermAll
	}

	var base Permission
	for _, r := range roles {
		base |= r.Permissions
	}
	if base&PermAdministrator != 0 {
		return PermAll
	}

	roleIDs := make(map[uuid.UUID]struct{}, len(roles))
	for _, r := range roles {
		roleIDs[r.ID] = struct{}{}
	}

	var roleAllow, roleDeny Permission
	var memberOverwrite *Overwrite
	for i := range overwrites {
		ov := overwrites[i]
		switch ov.TargetType {
		case OverwriteRole:
			if _, ok := roleIDs[ov.TargetID]; ok {
				roleAllow |= ov.Allow
				roleDeny |= ov.Deny
			}
		case OverwriteMember:
			if ov.TargetID == userID {
				memberOverwrite = &overwrites[i]
			}
		}
	}

	perms := base &^ roleDeny
	perms |= roleAllow

	if memberOverwrite != nil {
		perms &^= memberOverwrite.Deny
		perms |= memberOverwrite.Allow
	}
	return perms
}
