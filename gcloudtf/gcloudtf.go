// Package gcloudtf handles parsing Terraform files to extract structured
// information out of it, to support a few tools to speed up the DeployStack
// authoring process
package gcloudtf

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-config-inspect/tfconfig"
)

// Extract points to a path that includes Terraform files and extracts all of
// the information out of it for use with DeployStack Tools
func Extract(path string) (*Blocks, error) {
	mod, dia := tfconfig.LoadModule(path)
	if dia.Err() != nil {
		return nil, fmt.Errorf("terraform config problem %+v", dia)
	}

	b, err := NewBlocks(mod)
	if dia.Err() != nil {
		return nil, fmt.Errorf("could not properly parse blocks %+v", err)
	}

	return b, nil
}

// Block represents one of several kinds of Terraform constructs: resources,
// variables, module
type Block struct {
	Name  string
	Text  string
	Kind  string
	Type  string
	Attr  map[string]string
	file  string
	start int
}

// NewResourceBlock converts a parsed Terraform Resource to a Block
func NewResourceBlock(t *tfconfig.Resource) (Block, error) {
	b := Block{}
	var err error
	b.Name = t.Name
	b.Type = t.Type
	b.Kind = t.Mode.String()
	b.start = t.Pos.Line
	b.file = t.Pos.Filename
	b.Text, err = getResourceText(t.Pos.Filename, t.Pos.Line)
	if err != nil {
		return b, fmt.Errorf("could not extract text from Resource: %s", err)
	}

	b.generateMap(List{})

	return b, nil
}

// NewVariableBlock converts a parsed Terraform Variable to a Block
func NewVariableBlock(t *tfconfig.Variable) (Block, error) {
	b := Block{}
	var err error
	b.Name = t.Name
	b.Type = t.Type
	b.Kind = "variable"
	b.start = t.Pos.Line
	b.file = t.Pos.Filename
	b.Text, err = getResourceText(t.Pos.Filename, t.Pos.Line)
	if err != nil {
		return b, fmt.Errorf("could not extract text from Variable: %s", err)
	}
	return b, nil
}

// NewModuleBlock converts a parsed Terraform Module to a Block
func NewModuleBlock(t *tfconfig.ModuleCall) (Block, error) {
	b := Block{}
	var err error
	b.Name = t.Name
	b.Type = t.Source
	b.Kind = "module"
	b.start = t.Pos.Line
	b.file = t.Pos.Filename
	b.Text, err = getResourceText(t.Pos.Filename, t.Pos.Line)
	if err != nil {
		return b, fmt.Errorf("could not extract text from Module: %s", err)
	}
	return b, nil
}

func (b *Block) generateMap(terms List) {
	sl := strings.Split(b.Text, "\n")
	m := map[string]string{}
	terms = append(terms, "name", "region", "zone")

	for _, t := range terms {
		for _, v := range sl {

			if strings.Contains(v, "#") {
				continue
			}

			lsl := strings.Split(v, "=")

			if strings.Contains(lsl[0], t) {
				if len(lsl) > 1 {
					m[t] = strings.TrimSpace(lsl[1])
					break
				}
			}
		}
	}

	b.Attr = m
}

func getResourceText(file string, start int) (string, error) {
	dat, _ := os.ReadFile(file)
	sl := strings.Split(string(dat), "\n")

	resultSl := []string{}

	startpos := start - 1

	end := len(sl) - 1

	end = findClosingBracket(start, sl) + 1

	if startpos == 0 {
		startpos = 1
	}

	for i := startpos - 1; i < end; i++ {
		resultSl = append(resultSl, sl[i])
	}

	result := strings.Join(resultSl, "\n")

	return result, nil
}

func findClosingBracket(start int, sl []string) int {
	count := 0
	for i := start - 1; i < len(sl); i++ {
		if strings.Contains(sl[i], "{") {
			count++
		}
		if strings.Contains(sl[i], "}") {
			count--
		}
		if count == 0 {
			return i
		}
	}

	return len(sl)
}

// Blocks is a slice of type Block
type Blocks []Block

// NewBlocks converts the results from a Terraform parse operation to Blocks.
func NewBlocks(mod *tfconfig.Module) (*Blocks, error) {
	result := Blocks{}

	for _, v := range mod.ModuleCalls {
		b, err := NewModuleBlock(v)
		if err != nil {
			return nil, fmt.Errorf("could not parse Module Calls: %s", err)
		}
		result = append(result, b)
	}

	for _, v := range mod.ManagedResources {
		b, err := NewResourceBlock(v)
		if err != nil {
			return nil, fmt.Errorf("could not parse ManagedResources: %s", err)
		}
		result = append(result, b)
	}

	for _, v := range mod.DataResources {
		b, err := NewResourceBlock(v)
		if err != nil {
			return nil, fmt.Errorf("could not parse DataResources: %s", err)
		}
		result = append(result, b)
	}

	for _, v := range mod.Variables {
		b, err := NewVariableBlock(v)
		if err != nil {
			return nil, fmt.Errorf("could not parse Variables: %s", err)
		}
		result = append(result, b)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].start < result[j].start
	})

	return &result, nil
}

// List is a slice of strings that we add extra functionality to
type List []string

// Matches determines if a given string is an entry in the list.
func (l List) Matches(s string) bool {
	tmp := strings.ToLower(s)
	for _, v := range l {
		if strings.Contains(tmp, strings.ToLower(v)) {
			return true
		}
	}
	return false
}
