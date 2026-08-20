package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestActorHighestIsTheTopOfItsRoles(t *testing.T) {
	owner := uuid.New()
	user := uuid.New()

	actor := NewActor(owner, user, []RoleGrant{
		{ID: uuid.New(), Permissions: DefaultEveryonePermissions, Position: 0},
		{ID: uuid.New(), Permissions: PermManageRoles, Position: 7},
		{ID: uuid.New(), Permissions: PermKickMembers, Position: 3},
	}, nil)

	if actor.Highest != 7 {
		t.Errorf("Highest = %d, want 7", actor.Highest)
	}
	if actor.IsOwner {
		t.Error("a member is not the owner")
	}
	if !actor.Permissions.Has(PermManageRoles | PermKickMembers) {
		t.Error("an actor's permissions are the union of its roles")
	}
}

func TestMemberWithOnlyEveryoneOutranksNothing(t *testing.T) {
	owner := uuid.New()
	user := uuid.New()

	actor := NewActor(owner, user, []RoleGrant{
		{ID: uuid.New(), Permissions: DefaultEveryonePermissions, Position: EveryonePosition},
	}, nil)

	if actor.Outranks(EveryonePosition) {
		t.Error("@everyone must not outrank itself")
	}
	if actor.Outranks(1) {
		t.Error("a plain member must not outrank a role above it")
	}
}

func TestOutranksIsStrict(t *testing.T) {
	owner := uuid.New()
	user := uuid.New()

	actor := NewActor(owner, user, []RoleGrant{
		{ID: uuid.New(), Permissions: PermManageRoles, Position: 5},
	}, nil)

	if !actor.Outranks(4) {
		t.Error("position 5 must outrank position 4")
	}
	if actor.Outranks(5) {
		t.Error("a role at your own position is not outranked")
	}
	if actor.Outranks(6) {
		t.Error("a role above you is not outranked")
	}
}

func TestOwnerOutranksAndGrantsEverything(t *testing.T) {
	owner := uuid.New()

	actor := NewActor(owner, owner, nil, nil)

	if !actor.Outranks(1 << 20) {
		t.Error("the owner outranks any position")
	}
	if !actor.CanGrant(PermAdministrator) {
		t.Error("the owner can grant anything")
	}
}

func TestCanGrantRefusesPermissionsTheActorLacks(t *testing.T) {
	owner := uuid.New()
	user := uuid.New()

	actor := NewActor(owner, user, []RoleGrant{
		{ID: uuid.New(), Permissions: PermManageRoles | PermKickMembers, Position: 5},
	}, nil)

	if !actor.CanGrant(PermKickMembers) {
		t.Error("an actor can grant a permission it holds")
	}
	if actor.CanGrant(PermAdministrator) {
		t.Error("ManageRoles must not be a path to granting Administrator")
	}
	if actor.CanGrant(PermKickMembers | PermBanMembers) {
		t.Error("CanGrant must require every requested bit")
	}
}

func TestChannelOverwritesNarrowWhatAnActorCanGrant(t *testing.T) {
	owner := uuid.New()
	user := uuid.New()
	moderator := uuid.New()

	roles := []RoleGrant{{ID: moderator, Permissions: PermManageRoles | PermSendMessages, Position: 5}}
	denied := NewActor(owner, user, roles, []Overwrite{
		{TargetID: moderator, TargetType: OverwriteRole, Deny: PermSendMessages},
	})

	if denied.CanGrant(PermSendMessages) {
		t.Error("a permission denied in this channel must not be grantable in it")
	}
	if denied.Highest != 5 {
		t.Errorf("Highest = %d, want 5: an overwrite does not move a role", denied.Highest)
	}
}

func TestAdministratorCanGrantAnything(t *testing.T) {
	owner := uuid.New()
	user := uuid.New()

	actor := NewActor(owner, user, []RoleGrant{
		{ID: uuid.New(), Permissions: PermAdministrator, Position: 5},
	}, nil)

	if !actor.CanGrant(PermBanMembers) {
		t.Error("an administrator can grant anything it could exercise")
	}
	if actor.Outranks(5) {
		t.Error("Administrator does not lift the position rule")
	}
}
