package astutil

import (
	"go/ast"
	"go/types"
	"sync"
)

type indexRegistry struct {
	mu          sync.Mutex
	identifiers map[*types.Info]*identifierIndex
	safety      map[*types.Info]*safetyIndex
}

var indexes indexRegistry

func (r *indexRegistry) Identifier(info *types.Info) *identifierIndex {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.identifiers == nil {
		r.identifiers = make(map[*types.Info]*identifierIndex)
	}
	if index := r.identifiers[info]; index != nil {
		return index
	}
	index := &identifierIndex{seenNodes: make(map[ast.Node]struct{}), seenDefs: make(map[*ast.Ident]struct{})}
	r.identifiers[info] = index
	return index
}

func (r *indexRegistry) SafetyFor(info *types.Info) *safetyIndex {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.safety == nil {
		r.safety = make(map[*types.Info]*safetyIndex)
	}
	if index := r.safety[info]; index != nil {
		return index
	}
	index := buildSafetyIndex(info)
	r.safety[info] = index
	return index
}

func (r *indexRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.identifiers = nil
	r.safety = nil
}

func (r *indexRegistry) ClearSafety() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.safety = nil
}
