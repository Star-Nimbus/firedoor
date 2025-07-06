package telemetry

import accessv1alpha1 "github.com/cloud-nimbus/firedoor/api/v1alpha1"

// getAllNamespaces returns all namespaces referenced by a Breakglass resource.
func getAllNamespaces(bg *accessv1alpha1.Breakglass) []string {
	if bg == nil {
		return nil
	}
	set := make(map[string]struct{})
	if bg.Spec.AccessPolicy != nil {
		for _, rule := range bg.Spec.AccessPolicy.Rules {
			for _, ns := range rule.Namespaces {
				set[ns] = struct{}{}
			}
		}
	}
	if len(set) == 0 && bg.Namespace != "" {
		set[bg.Namespace] = struct{}{}
	}
	var out []string
	for ns := range set {
		out = append(out, ns)
	}
	return out
}
