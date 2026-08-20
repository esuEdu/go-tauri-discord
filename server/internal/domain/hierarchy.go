package domain

import "github.com/google/uuid"

const EveryonePosition int32 = 0

type Actor struct {
	IsOwner     bool
	Permissions Permission
	Highest     int32
}

func NewActor(ownerID, userID uuid.UUID, roles []RoleGrant, overwrites []Overwrite) Actor {
	actor := Actor{
		IsOwner:     ownerID == userID,
		Permissions: ResolvePermissions(ownerID, userID, roles, overwrites),
		Highest:     EveryonePosition,
	}
	for _, r := range roles {
		if r.Position > actor.Highest {
			actor.Highest = r.Position
		}
	}
	return actor
}

func (a Actor) Outranks(position int32) bool {
	if a.IsOwner {
		return true
	}
	return position < a.Highest
}

func (a Actor) CanGrant(perms Permission) bool {
	if a.IsOwner {
		return true
	}
	return a.Permissions.Has(perms)
}
