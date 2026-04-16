package store

import "goim/internal/protocol"

func dedupeStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, item := range in {
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func reverseMessages(in []*protocol.Message) {
	for i, j := 0, len(in)-1; i < j; i, j = i+1, j-1 {
		in[i], in[j] = in[j], in[i]
	}
}

func cloneMessage(m *protocol.Message) *protocol.Message {
	if m == nil {
		return nil
	}
	cp := *m
	return &cp
}
