package group

import "testing"

func TestGroupLifecycle(t *testing.T) {
	gm := NewGroupManager()

	if err := gm.Create("g1", "alice"); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// duplicate create should fail
	if err := gm.Create("g1", "bob"); err == nil {
		t.Fatalf("expected duplicate create to fail")
	}

	if err := gm.Join("g1", "bob"); err != nil {
		t.Fatalf("join failed: %v", err)
	}

	if members := gm.Members("g1"); len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}

	if role, ok := gm.RoleOf("g1", "alice"); !ok || role != GroupRoleOwner {
		t.Fatalf("alice should be owner")
	}

	if err := gm.GrantAdmin("g1", "alice", "bob"); err != nil {
		t.Fatalf("grant admin failed: %v", err)
	}

	if role, _ := gm.RoleOf("g1", "bob"); role != GroupRoleAdmin {
		t.Fatalf("bob should be admin")
	}

	// admin kicks a member
	if err := gm.Join("g1", "charlie"); err != nil {
		t.Fatalf("join charlie failed: %v", err)
	}
	if err := gm.Kick("g1", "bob", "charlie"); err != nil {
		t.Fatalf("kick failed: %v", err)
	}

	// owner cannot leave
	if err := gm.Leave("g1", "alice"); err == nil {
		t.Fatalf("owner should not be able to leave")
	}

	// delete by owner
	if err := gm.Delete("g1", "alice"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
}
