package annotation

import (
	"go/ast"
	"go/token"
)

// ChainCollector defines the interface for handlers in the annotation processing chain.
// Implementations should handle specific annotation types and pass unhandled cases to the next handler in the chain.
type ChainCollector interface {
	// Handle processes an annotation if it matches the handler's type,
	// otherwise delegates to the next handler in the chain.
	Handle(name string, comment *ast.Comment, fset *token.FileSet, file *ast.File, fileAbs string)
	// SetNext establishes the next handler in the chain of responsibility.
	SetNext(next ChainCollector)
}

// BaseCollector provides default chain handling behavior
type BaseCollector struct {
	next ChainCollector
}

// SetNext sets the next handler in the chain of responsibility.
func (h *BaseCollector) SetNext(next ChainCollector) {
	h.next = next
}

// Handle implements the default chain behavior by delegating to the next handler.
func (h *BaseCollector) Handle(name string, comment *ast.Comment, fset *token.FileSet, file *ast.File, fileAbs string) {
	if h.next != nil {
		h.next.Handle(name, comment, fset, file, fileAbs)
	}
}

// RegexAnnotationCollector implements the ChainCollector interface for "mutator-disable-regexp" annotations.
type RegexAnnotationCollector struct {
	BaseCollector
	Processor   RegexAnnotation
	nodesByLine map[int][]ast.Node
}

// NextLineAnnotationCollector implements the ChainCollector interface for "mutator-disable-next-line" annotations.
type NextLineAnnotationCollector struct {
	BaseCollector
	Processor   LineAnnotation
	nodesByLine map[int][]ast.Node
}

// Handle processes regex pattern annotations, delegating other types to the next handler.
func (r *RegexAnnotationCollector) Handle(name string, comment *ast.Comment, fset *token.FileSet, file *ast.File, fileAbs string) {
	if name != RegexpAnnotation {
		r.BaseCollector.Handle(name, comment, fset, file, fileAbs)

		return
	}

	r.Processor.collectMatchNodes(comment, fset, file, fileAbs, r.nodesByLine)
}

// Handle processes regex pattern annotations, delegating other types to the next handler.
func (n *NextLineAnnotationCollector) Handle(name string, comment *ast.Comment, fset *token.FileSet, file *ast.File, fileAbs string) {
	if name != NextLineAnnotation {
		n.BaseCollector.Handle(name, comment, fset, file, fileAbs)

		return
	}

	n.Processor.collectNodesOnNextLine(comment, fset, file, n.nodesByLine)
}

func (p *Processor) buildChain(nodesByLine map[int][]ast.Node) ChainCollector {
	regexHandler := &RegexAnnotationCollector{Processor: p.RegexAnnotation, nodesByLine: nodesByLine}
	nextLineHandler := &NextLineAnnotationCollector{Processor: p.LineAnnotation, nodesByLine: nodesByLine}
	regexHandler.SetNext(nextLineHandler)

	return regexHandler
}
