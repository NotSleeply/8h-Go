package server

import (
	"errors"
	"strings"
	"sync"
)

type GroupRole int8

const (
	GroupRoleMember GroupRole = 0
	GroupRoleAdmin  GroupRole = 1
	GroupRoleOwner  GroupRole = 2
)

type group struct {
	ID      string
	Owner   string
	Members map[string]GroupRole
}

type GroupManager struct {
	mu     sync.RWMutex
	groups map[string]*group
}

func NewGroupManager() *GroupManager {
	return &GroupManager{groups: make(map[string]*group)}
}

func (gm *GroupManager) Create(groupID, owner string) error {
	id := strings.TrimSpace(groupID)
	own := strings.TrimSpace(owner)
	if id == "" || own == "" {
		return errors.New("groupID/owner empty")
	}
	gm.mu.Lock()
	defer gm.mu.Unlock()
	if _, ok := gm.groups[id]; ok {
		return errors.New("group already exists")
	}
	gm.groups[id] = &group{ID: id, Owner: own, Members: map[string]GroupRole{own: GroupRoleOwner}}
	return nil
}

func (gm *GroupManager) Delete(groupID, operator string) error {
	id, op := strings.TrimSpace(groupID), strings.TrimSpace(operator)
	if id == "" || op == "" {
		return errors.New("groupID/operator empty")
	}
	gm.mu.Lock()
	defer gm.mu.Unlock()
	g, ok := gm.groups[id]
	if !ok {
		return errors.New("group not found")
	}
	if g.Owner != op {
		return errors.New("only owner can delete group")
	}
	delete(gm.groups, id)
	return nil
}

func (gm *GroupManager) Join(groupID, username string) error {
	id, u := strings.TrimSpace(groupID), strings.TrimSpace(username)
	if id == "" || u == "" {
		return errors.New("groupID/username empty")
	}
	gm.mu.Lock()
	defer gm.mu.Unlock()
	g, ok := gm.groups[id]
	if !ok {
		return errors.New("group not found")
	}
	if _, exists := g.Members[u]; exists {
		return errors.New("already a member")
	}
	g.Members[u] = GroupRoleMember
	return nil
}

func (gm *GroupManager) Leave(groupID, username string) error {
	id, u := strings.TrimSpace(groupID), strings.TrimSpace(username)
	if id == "" || u == "" {
		return errors.New("groupID/username empty")
	}
	gm.mu.Lock()
	defer gm.mu.Unlock()
	g, ok := gm.groups[id]
	if !ok {
		return errors.New("group not found")
	}
	if g.Owner == u {
		return errors.New("owner cannot leave, use delete instead")
	}
	delete(g.Members, u)
	return nil
}

func (gm *GroupManager) Kick(groupID, by, target string) error {
	id := strings.TrimSpace(groupID)
	gm.mu.Lock()
	defer gm.mu.Unlock()
	g, ok := gm.groups[id]
	if !ok {
		return errors.New("group not found")
	}
	byRole, exists := g.Members[by]
	if !exists {
		return errors.New("operator not in group")
	}
	if byRole < GroupRoleAdmin {
		return errors.New("insufficient permission")
	}
	targetRole, exists := g.Members[target]
	if !exists {
		return errors.New("target not in group")
	}
	if targetRole >= byRole {
		return errors.New("cannot kick member with equal or higher role")
	}
	delete(g.Members, target)
	return nil
}

func (gm *GroupManager) GrantAdmin(groupID, by, target string) error {
	id := strings.TrimSpace(groupID)
	gm.mu.Lock()
	defer gm.mu.Unlock()
	g, ok := gm.groups[id]
	if !ok {
		return errors.New("group not found")
	}
	if g.Owner != by {
		return errors.New("only owner can grant admin")
	}
	if _, exists := g.Members[target]; !exists {
		return errors.New("target not in group")
	}
	g.Members[target] = GroupRoleAdmin
	return nil
}

func (gm *GroupManager) RevokeAdmin(groupID, by, target string) error {
	id := strings.TrimSpace(groupID)
	gm.mu.Lock()
	defer gm.mu.Unlock()
	g, ok := gm.groups[id]
	if !ok {
		return errors.New("group not found")
	}
	if g.Owner != by {
		return errors.New("only owner can revoke admin")
	}
	if _, exists := g.Members[target]; !exists {
		return errors.New("target not in group")
	}
	g.Members[target] = GroupRoleMember
	return nil
}

func (gm *GroupManager) Members(groupID string) []string {
	id := strings.TrimSpace(groupID)
	gm.mu.RLock()
	defer gm.mu.RUnlock()
	g, ok := gm.groups[id]
	if !ok {
		return nil
	}
	out := make([]string, 0, len(g.Members))
	for name := range g.Members {
		out = append(out, name)
	}
	return out
}

func (gm *GroupManager) RoleOf(groupID, username string) (string, bool) {
	id := strings.TrimSpace(groupID)
	gm.mu.RLock()
	defer gm.mu.RUnlock()
	g, ok := gm.groups[id]
	if !ok {
		return "", false
	}
	role, exists := g.Members[username]
	if !exists {
		return "", false
	}
	switch role {
	case GroupRoleOwner:
		return "owner", true
	case GroupRoleAdmin:
		return "admin", true
	default:
		return "member", true
	}
}
