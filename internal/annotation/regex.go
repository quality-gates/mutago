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

	pattern, mutatorList := splitRegexAnnotation(content)
	pattern = unquotePattern(pattern)
	if pattern == "" {
		return nil, mutatorInfo{}
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		log.Printf("Warning: invalid regex in annotation: %q, error: %v\n", pattern, err)
		return nil, mutatorInfo{}
	}

	var mutators []string
	if mutatorList != "" {
		mutators = parseMutators(mutatorList)
	}

	return re, mutatorInfo{
		Names: mutators,
	}
}

func unquotePattern(pattern string) string {
	pattern = strings.TrimSpace(pattern)
	if len(pattern) >= 2 {
		if (pattern[0] == '"' && pattern[len(pattern)-1] == '"') ||
			(pattern[0] == '`' && pattern[len(pattern)-1] == '`') {
			return pattern[1 : len(pattern)-1]
		}
	}
	return pattern
}

var validMutatorRune = [256]bool{
	'*': true,
	'-': true,
	'/': true,
	'_': true,
}

func init() {
	for c := 'a'; c <= 'z'; c++ {
		validMutatorRune[c] = true
	}
	for c := 'A'; c <= 'Z'; c++ {
		validMutatorRune[c] = true
	}
	for c := '0'; c <= '9'; c++ {
		validMutatorRune[c] = true
	}
}

func isValidMutatorName(name string) bool {
	if name == "*" {
		return true
	}
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		b := name[i]
		if b >= 128 || !validMutatorRune[b] {
			return false
		}
	}
	return true
}

func splitWildcard(content string) (string, string, bool) {
	lastSpace := strings.LastIndexAny(content, " \t")
	if lastSpace == -1 {
		return "", "", false
	}
	lastToken := strings.TrimSpace(content[lastSpace:])
	if lastToken == "*" {
		return strings.TrimSpace(content[:lastSpace]), "*", true
	}
	return "", "", false
}

func splitSingleMutator(content string) (string, string, bool) {
	lastSpace := strings.LastIndexAny(content, " \t")
	if lastSpace == -1 {
		return "", "", false
	}
	candidateMutator := strings.TrimSpace(content[lastSpace:])
	candidatePattern := strings.TrimSpace(content[:lastSpace])
	if candidatePattern != "" && isValidMutatorName(candidateMutator) {
		return candidatePattern, candidateMutator, true
	}
	return "", "", false
}

func findCommaMutatorBoundary(commaParts []string) (int, int, bool) {
	for i := len(commaParts) - 2; i >= 0; i-- {
		trimmed := strings.TrimSpace(commaParts[i])
		if isValidMutatorName(trimmed) {
			continue
		}
		idx := strings.LastIndexAny(commaParts[i], " \t")
		if idx != -1 && isValidMutatorName(strings.TrimSpace(commaParts[i][idx:])) {
			return i, idx, true
		}
		return -1, -1, false
	}
	return -1, -1, false
}

func splitCommaMutators(content string) (string, string, bool) {
	if !strings.Contains(content, ",") {
		return "", "", false
	}
	commaParts := strings.Split(content, ",")
	lastPart := strings.TrimSpace(commaParts[len(commaParts)-1])
	if !isValidMutatorName(lastPart) {
		return "", "", false
	}

	targetPartIdx, firstMutIdxInPart, ok := findCommaMutatorBoundary(commaParts)
	if !ok {
		return "", "", false
	}

	patternParts := append([]string{}, commaParts[:targetPartIdx]...)
	patternParts = append(patternParts, commaParts[targetPartIdx][:firstMutIdxInPart])
	pattern := strings.TrimSpace(strings.Join(patternParts, ","))

	mutatorParts := append([]string{strings.TrimSpace(commaParts[targetPartIdx][firstMutIdxInPart:])}, commaParts[targetPartIdx+1:]...)
	mutatorList := strings.Join(mutatorParts, ", ")
	return pattern, mutatorList, true
}

func splitRegexAnnotation(content string) (string, string) {
	content = strings.TrimSpace(content)
	if content == "" || !strings.ContainsAny(content, " \t") {
		return content, ""
	}

	if pattern, mutators, ok := splitWildcard(content); ok {
		return pattern, mutators
	}
	if pattern, mutators, ok := splitCommaMutators(content); ok {
		return pattern, mutators
	}
	if pattern, mutators, ok := splitSingleMutator(content); ok {
		return pattern, mutators
	}

	return content, ""
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
