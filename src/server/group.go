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

type Group struct {
	ID      string
	Name    string
	Owner   string
	Members map[string]GroupRole
}

type GroupManager struct {
	mu     sync.RWMutex
	groups map[string]*Group
}

func NewGroupManager() *GroupManager {
	return &GroupManager{groups: make(map[string]*Group)}
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
	gm.groups[id] = &Group{
		ID:      id,
		Name:    id,
		Owner:   own,
		Members: map[string]GroupRole{own: GroupRoleOwner},
	}
	return nil
}

func (gm *GroupManager) Delete(groupID, operator string) error {
	id := strings.TrimSpace(groupID)
	op := strings.TrimSpace(operator)
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
	id := strings.TrimSpace(groupID)
	u := strings.TrimSpace(username)
	if id == "" || u == "" {
		return errors.New("groupID/username empty")
	}
	gm.mu.Lock()
	defer gm.mu.Unlock()
	g, ok := gm.groups[id]
	if !ok {
		return errors.New("group not found")
	}
	if _, ok := g.Members[u]; ok {
		return nil
	}
	g.Members[u] = GroupRoleMember
	return nil
}

func (gm *GroupManager) Leave(groupID, username string) error {
	id := strings.TrimSpace(groupID)
	u := strings.TrimSpace(username)
	if id == "" || u == "" {
		return errors.New("groupID/username empty")
	}
	gm.mu.Lock()
	defer gm.mu.Unlock()
	g, ok := gm.groups[id]
	if !ok {
		return errors.New("group not found")
	}
	role, ok := g.Members[u]
	if !ok {
		return nil
	}
	if role == GroupRoleOwner {
		return errors.New("owner cannot leave; delete group instead")
	}
	delete(g.Members, u)
	return nil
}

func (gm *GroupManager) Kick(groupID, operator, target string) error {
	id := strings.TrimSpace(groupID)
	op := strings.TrimSpace(operator)
	t := strings.TrimSpace(target)
	if id == "" || op == "" || t == "" {
		return errors.New("groupID/operator/target empty")
	}
	gm.mu.Lock()
	defer gm.mu.Unlock()
	g, ok := gm.groups[id]
	if !ok {
		return errors.New("group not found")
	}
	opRole, ok := g.Members[op]
	if !ok {
		return errors.New("operator not in group")
	}
	if opRole < GroupRoleAdmin {
		return errors.New("insufficient permission")
	}
	tRole, ok := g.Members[t]
	if !ok {
		return errors.New("target not in group")
	}
	if tRole == GroupRoleOwner {
		return errors.New("cannot kick owner")
	}
	delete(g.Members, t)
	return nil
}

func (gm *GroupManager) GrantAdmin(groupID, operator, target string) error {
	id := strings.TrimSpace(groupID)
	op := strings.TrimSpace(operator)
	t := strings.TrimSpace(target)
	if id == "" || op == "" || t == "" {
		return errors.New("groupID/operator/target empty")
	}
	gm.mu.Lock()
	defer gm.mu.Unlock()
	g, ok := gm.groups[id]
	if !ok {
		return errors.New("group not found")
	}
	if g.Owner != op {
		return errors.New("only owner can grant admin")
	}
	if _, ok := g.Members[t]; !ok {
		return errors.New("target not in group")
	}
	g.Members[t] = GroupRoleAdmin
	return nil
}

func (gm *GroupManager) RevokeAdmin(groupID, operator, target string) error {
	id := strings.TrimSpace(groupID)
	op := strings.TrimSpace(operator)
	t := strings.TrimSpace(target)
	if id == "" || op == "" || t == "" {
		return errors.New("groupID/operator/target empty")
	}
	gm.mu.Lock()
	defer gm.mu.Unlock()
	g, ok := gm.groups[id]
	if !ok {
		return errors.New("group not found")
	}
	if g.Owner != op {
		return errors.New("only owner can revoke admin")
	}
	role, ok := g.Members[t]
	if !ok {
		return errors.New("target not in group")
	}
	if role == GroupRoleOwner {
		return errors.New("cannot revoke owner")
	}
	g.Members[t] = GroupRoleMember
	return nil
}

func (gm *GroupManager) Members(groupID string) []string {
	id := strings.TrimSpace(groupID)
	if id == "" {
		return nil
	}
	gm.mu.RLock()
	defer gm.mu.RUnlock()
	g, ok := gm.groups[id]
	if !ok {
		return nil
	}
	out := make([]string, 0, len(g.Members))
	for user := range g.Members {
		out = append(out, user)
	}
	return out
}

func (gm *GroupManager) RoleOf(groupID, username string) (GroupRole, bool) {
	id := strings.TrimSpace(groupID)
	u := strings.TrimSpace(username)
	if id == "" || u == "" {
		return 0, false
	}
	gm.mu.RLock()
	defer gm.mu.RUnlock()
	g, ok := gm.groups[id]
	if !ok {
		return 0, false
	}
	role, ok := g.Members[u]
	return role, ok
}
