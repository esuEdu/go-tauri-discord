package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestResolvePermissions(t *testing.T) {
	owner := uuid.New()
	user := uuid.New()
	everyone := uuid.New()
	moderator := uuid.New()

	tests := []struct {
		name       string
		userID     uuid.UUID
		roles      []RoleGrant
		overwrites []Overwrite
		want       Permission
		wantNot    Permission
	}{
		{
			name:   "owner gets everything regardless of roles",
			userID: owner,
			roles:  nil,
			want:   PermManageRoles | PermBanMembers,
		},
		{
			name:   "administrator implies every other permission",
			userID: user,
			roles:  []RoleGrant{{ID: moderator, Permissions: PermAdministrator}},
			want:   PermBanMembers | PermManageChannels,
		},
		{
			name:   "role permissions are unioned",
			userID: user,
			roles: []RoleGrant{
				{ID: everyone, Permissions: PermViewChannel},
				{ID: moderator, Permissions: PermManageMessages},
			},
			want: PermViewChannel | PermManageMessages,
		},
		{
			name:   "a channel deny removes a role permission",
			userID: user,
			roles:  []RoleGrant{{ID: everyone, Permissions: PermViewChannel | PermSendMessages}},
			overwrites: []Overwrite{
				{TargetID: everyone, TargetType: OverwriteRole, Deny: PermSendMessages},
			},
			want:    PermViewChannel,
			wantNot: PermSendMessages,
		},
		{
			name:   "an allow on a second role lifts another role's deny",
			userID: user,
			roles: []RoleGrant{
				{ID: everyone, Permissions: PermViewChannel | PermSendMessages},
				{ID: moderator, Permissions: PermViewChannel},
			},
			overwrites: []Overwrite{
				{TargetID: everyone, TargetType: OverwriteRole, Deny: PermSendMessages},
				{TargetID: moderator, TargetType: OverwriteRole, Allow: PermSendMessages},
			},
			want: PermSendMessages,
		},
		{
			name:   "a member overwrite beats the role overwrites",
			userID: user,
			roles:  []RoleGrant{{ID: everyone, Permissions: PermViewChannel | PermSendMessages}},
			overwrites: []Overwrite{
				{TargetID: everyone, TargetType: OverwriteRole, Allow: PermSendMessages},
				{TargetID: user, TargetType: OverwriteMember, Deny: PermSendMessages},
			},
			want:    PermViewChannel,
			wantNot: PermSendMessages,
		},
		{
			name:   "overwrites aimed at other members are ignored",
			userID: user,
			roles:  []RoleGrant{{ID: everyone, Permissions: PermSendMessages}},
			overwrites: []Overwrite{
				{TargetID: uuid.New(), TargetType: OverwriteMember, Deny: PermSendMessages},
			},
			want: PermSendMessages,
		},
		{
			name:   "overwrites for roles the member lacks are ignored",
			userID: user,
			roles:  []RoleGrant{{ID: everyone, Permissions: PermSendMessages}},
			overwrites: []Overwrite{
				{TargetID: moderator, TargetType: OverwriteRole, Deny: PermSendMessages},
			},
			want: PermSendMessages,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolvePermissions(owner, tt.userID, tt.roles, tt.overwrites)
			if tt.want != 0 && !got.Has(tt.want) {
				t.Errorf("missing permission: got %b, want %b set", got, tt.want)
			}
			if tt.wantNot != 0 && got&tt.wantNot != 0 {
				t.Errorf("permission should have been denied: got %b, want %b clear", got, tt.wantNot)
			}
		})
	}
}

func TestHasRequiresEveryBit(t *testing.T) {
	p := PermViewChannel
	if p.Has(PermViewChannel | PermSendMessages) {
		t.Error("Has must require all requested bits, not any")
	}
}

func TestAdministratorSatisfiesHas(t *testing.T) {
	if !PermAdministrator.Has(PermBanMembers) {
		t.Error("administrator must satisfy any permission check")
	}
}
