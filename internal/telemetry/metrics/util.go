package metrics

import accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"

// getAllNamespaces returns all namespaces referenced by a Breakglass resource.
func getAllNamespaces(bg *accessv1alpha1.Breakglass) []string {
	if bg == nil {
		return nil
	}
	set := make(map[string]struct{})
	// For now, use the breakglass namespace since PolicyRule doesn't contain namespace info
	if bg.Namespace != "" {
		set[bg.Namespace] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for ns := range set {
		out = append(out, ns)
	}
	return out
}
