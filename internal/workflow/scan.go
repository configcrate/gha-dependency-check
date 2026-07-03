package workflow

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/configcrate/gha-dependency-check/internal/model"
	"gopkg.in/yaml.v3"
)

type ScanResult struct {
	Files        []string
	Dependencies []model.Dependency
	Invalid      []model.Result
}

func Scan(path string) (ScanResult, error) {
	files, err := workflowFiles(path)
	if err != nil {
		return ScanResult{}, err
	}

	result := ScanResult{Files: files}
	for _, file := range files {
		deps, invalid, err := scanFile(file)
		if err != nil {
			return ScanResult{}, err
		}
		result.Dependencies = append(result.Dependencies, deps...)
		result.Invalid = append(result.Invalid, invalid...)
	}
	return result, nil
}

func workflowFiles(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("open scan path: %w", err)
	}
	if !info.IsDir() {
		if !isYAML(path) {
			return nil, fmt.Errorf("workflow file must end in .yml or .yaml: %s", path)
		}
		return []string{path}, nil
	}

	root := path
	defaultWorkflows := filepath.Join(path, ".github", "workflows")
	if nested, err := os.Stat(defaultWorkflows); err == nil && nested.IsDir() {
		root = defaultWorkflows
	}

	var files []string
	err = filepath.WalkDir(root, func(current string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if isYAML(current) {
			files = append(files, current)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk workflow directory: %w", err)
	}
	sort.Strings(files)
	return files, nil
}

func isYAML(path string) bool {
	extension := strings.ToLower(filepath.Ext(path))
	return extension == ".yml" || extension == ".yaml"
}

func scanFile(path string) ([]model.Dependency, []model.Result, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open workflow %s: %w", path, err)
	}
	defer file.Close()

	var dependencies []model.Dependency
	var invalid []model.Result
	decoder := yaml.NewDecoder(file)
	for {
		var document yaml.Node
		err = decoder.Decode(&document)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("parse workflow %s: %w", path, err)
		}
		walkUses(&document, func(node *yaml.Node) {
			dependency, skip, parseErr := parseUses(path, node.Line, node.Value)
			if skip {
				return
			}
			if parseErr != nil {
				invalid = append(invalid, model.Result{
					Dependency: dependency,
					Status:     model.StatusInvalid,
					Message:    parseErr.Error(),
				})
				return
			}
			dependencies = append(dependencies, dependency)
		})
	}
	return dependencies, invalid, nil
}

func walkUses(node *yaml.Node, visit func(*yaml.Node)) {
	if node == nil {
		return
	}
	if node.Kind == yaml.MappingNode {
		for index := 0; index+1 < len(node.Content); index += 2 {
			key := node.Content[index]
			value := node.Content[index+1]
			if key.Kind == yaml.ScalarNode && key.Value == "uses" && value.Kind == yaml.ScalarNode {
				visit(value)
			}
			walkUses(value, visit)
		}
		return
	}
	for _, child := range node.Content {
		walkUses(child, visit)
	}
}

func parseUses(file string, line int, raw string) (model.Dependency, bool, error) {
	uses := strings.TrimSpace(raw)
	dependency := model.Dependency{File: file, Line: line, Uses: uses}

	if uses == "" {
		return dependency, false, errors.New("empty uses value")
	}
	if strings.HasPrefix(uses, "./") || strings.HasPrefix(uses, "docker://") {
		return dependency, true, nil
	}
	if strings.Contains(uses, "${{") {
		return dependency, true, nil
	}

	at := strings.LastIndex(uses, "@")
	if at <= 0 || at == len(uses)-1 {
		return dependency, false, errors.New("remote uses value must include a non-empty @ref")
	}
	target := strings.Trim(uses[:at], "/")
	dependency.Ref = strings.TrimSpace(uses[at+1:])
	parts := strings.Split(target, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return dependency, false, errors.New("remote uses value must start with owner/repository")
	}
	dependency.Owner = parts[0]
	dependency.Repo = parts[1]
	if len(parts) > 2 {
		dependency.Path = strings.Join(parts[2:], "/")
	}
	return dependency, false, nil
}
