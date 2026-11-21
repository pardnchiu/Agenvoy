// * last modified: 26/11/21
package file

import (
	"fmt"
	"slices"
	"strings"

	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

type patchTarget struct {
	index int
	start int
	end   int
	text  string
}

func planTargets(content string, targetContents []patchContent, absPath string) ([]patchTarget, error) {
	var targets []patchTarget

	for _, targetContent := range targetContents {
		oldContent := targetContent.OldString
		newContent := targetContent.NewString

		at := offsetsOf(content, oldContent)
		if len(at) == 0 {
			return nil, utilsmsg.New(utilsmsg.Internal, fmt.Sprintf("patch_contents[%d]", targetContent.index), "%q is not in %s; anchors must pass verifyAnchors first", oldContent, absPath)
		}
		if len(at) > 1 && !targetContent.ReplaceAll {
			return nil, utilsmsg.New(utilsmsg.Invalid, fmt.Sprintf("patch_contents[%d]", targetContent.index), "%q occurs on rows %v of %s; extend old_string until it matches once, or set replace_all", oldContent, rowsAt(content, at), absPath)
		}
		if !targetContent.ReplaceAll {
			at = at[:1]
		}
		for _, i := range at {
			end := i + len(oldContent)
			// NOTE: deleting an anchor directly followed by `\n` removes that newline too
			if newContent == "" && !strings.HasSuffix(oldContent, "\n") && strings.HasPrefix(content[end:], "\n") {
				end++
			}
			targets = append(targets, patchTarget{
				index: targetContent.index,
				start: i,
				end:   end,
				text:  newContent,
			})
		}
	}

	slices.SortFunc(targets, func(a, b patchTarget) int { return a.start - b.start })
	for i := 1; i < len(targets); i++ {
		// NOTE: ensuring replace index wont same index
		if targets[i].start < targets[i-1].end {
			return nil, utilsmsg.New(utilsmsg.Invalid, "patch_contents", "patch_contents[%d] and patch_contents[%d] both cover row %d of %s, so applying one would destroy the other's anchor. Nothing was written — merge them into one entry, or send them in separate calls", targets[i-1].index, targets[i].index, rowAt(content, targets[i].start), absPath)
		}
	}
	return targets, nil
}

// NOTE: find all offsets
func offsetsOf(content, filterText string) []int {
	var at []int
	for idx := 0; ; {
		i := strings.Index(content[idx:], filterText)
		if i < 0 {
			return at
		}

		pos := idx + i
		at = append(at, pos)
		idx = pos + len(filterText)
	}
}

func rowsAt(content string, at []int) []int {
	rows := make([]int, 0, len(at))
	for _, i := range at {
		rows = append(rows, rowAt(content, i))
	}
	return rows
}

func rowAt(content string, i int) int {
	return strings.Count(content[:i], "\n") + 1
}
