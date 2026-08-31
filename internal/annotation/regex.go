package annotation

import (
	"bufio"
	"go/ast"
	"go/token"
	"log"
	"os"
	"regexp"
	"strings"
)

// parseRegexAnnotation parses a comment line containing a regex annotation.
func (r *RegexAnnotation) parseRegexAnnotation(comment string) (*regexp.Regexp, mutatorInfo) {
	content := strings.TrimSpace(strings.TrimPrefix(comment, r.Name))
	if content == "" {
		return nil, mutatorInfo{}
	}

	parts := strings.SplitN(content, " ", 2)

	pattern := strings.TrimSpace(parts[0])
	re, err := regexp.Compile(pattern)
	if err != nil {
		log.Printf("Warning: invalid regex in annotation: %q, error: %v\n", pattern, err)
		return nil, mutatorInfo{}
	}

	var mutators []string
	if len(parts) > 1 {
		mutators = parseMutators(parts[1])
	}

	return re, mutatorInfo{
		Names: mutators,
	}
}

// collectMatchNodes processes a "mutator-disable-regexp" annotation comment by:
// 1. Parsing the regex pattern and mutators from the comment
// 2. Finding all lines in the file that match the regex
// 3. Recording nodes from matching lines to be excluded
func (r *RegexAnnotation) collectMatchNodes(comment *ast.Comment, _ *token.FileSet, _ *ast.File, fileAbs string, nodesByLine map[int][]ast.Node) {
	regex, mutators := r.parseRegexAnnotation(comment.Text)

	lines, err := r.findLinesMatchingRegex(fileAbs, regex)
	if err != nil {
		log.Printf("Error scaning a source file: %v", err)
	}

	collectExcludedNodes(nodesByLine, lines, r.Exclusions, r.PositionIndex, mutators)
}

// findLinesMatchingRegex scans a source file and returns line numbers that match the given regex.
func (r *RegexAnnotation) findLinesMatchingRegex(filePath string, regex *regexp.Regexp) ([]int, error) {
	var matchedLineNumbers []int

	if regex == nil {
		return matchedLineNumbers, nil
	}

	f, err := os.Open(filePath)
	if err != nil {
		log.Printf("Error opening file: %v", err)
	}

	reader := bufio.NewReader(f)

	lineNumber := 0
	for {
		line, err := reader.ReadString('\n')
		// On io.EOF the final line is returned together with the error when the
		// file has no trailing newline. Process that partial line before breaking
		// so a regex matching only the last line is not silently ignored.
		if len(line) > 0 {
			if regex.MatchString(line) {
				matchedLineNumbers = append(matchedLineNumbers, lineNumber+1)
			}
			lineNumber++
		}
		if err != nil {
			break
		}
	}

	defer func() {
		err = f.Close()
		if err != nil {
			log.Printf("Error while file closing during processing regex annotation: %v", err.Error())
		}
	}()

	return matchedLineNumbers, nil
}

// filterRegexNodes checks if a given node should be excluded from mutation based on:
// 1. Whether the node appears in the Exclusions map
// 2. Whether the current mutator is in the node's exclusion list
func (r *RegexAnnotation) filterRegexNodes(node ast.Node, mutatorName string) bool {
	mutators, exists := r.PositionIndex[node.Pos()]
	return exists && shouldSkipMutator(mutators, mutatorName)
}
