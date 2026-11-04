package drvshow

import "sort"

// ClosureBFSOrder returns derivation keys in breadth-first order from rootDrvKey.
func ClosureBFSOrder(doc Document, rootDrvKey string) []string {
	if rootDrvKey == "" {
		return nil
	}
	var out []string
	seen := make(map[string]struct{})
	queue := []string{rootDrvKey}
	for len(queue) > 0 {
		k := queue[0]
		queue = queue[1:]
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		e, ok := doc.Derivations[k]
		if !ok {
			continue
		}
		out = append(out, k)
		inputs := make([]string, 0, len(e.Inputs.Drvs))
		for dk := range e.Inputs.Drvs {
			inputs = append(inputs, dk)
		}
		sort.Strings(inputs)
		queue = append(queue, inputs...)
	}
	return out
}

// PickKeyByBFS disambiguates duplicate logical keys by first BFS position from root.
func PickKeyByBFS(doc Document, rootDrvKey string, match func(key string, e Entry) bool) string {
	for _, k := range ClosureBFSOrder(doc, rootDrvKey) {
		e, ok := doc.Derivations[k]
		if !ok {
			continue
		}
		if match(k, e) {
			return k
		}
	}
	return ""
}
