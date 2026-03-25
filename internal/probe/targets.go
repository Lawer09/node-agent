package probe

import "singbox-node-agent/internal/config"

func ResolveTargets(dp config.DefaultProbeConfig) []ProbeTarget {
	targets := make([]ProbeTarget, 0)

	switch dp.ProbeMode {
	case "business":
		for _, u := range dp.ProbeTargets.Business {
			if u == "" {
				continue
			}
			targets = append(targets, ProbeTarget{
				Class: "business",
				URL:   u,
			})
		}
	case "both":
		for _, u := range dp.ProbeTargets.Standard {
			if u == "" {
				continue
			}
			targets = append(targets, ProbeTarget{
				Class: "standard",
				URL:   u,
			})
		}
		for _, u := range dp.ProbeTargets.Business {
			if u == "" {
				continue
			}
			targets = append(targets, ProbeTarget{
				Class: "business",
				URL:   u,
			})
		}
	default:
		for _, u := range dp.ProbeTargets.Standard {
			if u == "" {
				continue
			}
			targets = append(targets, ProbeTarget{
				Class: "standard",
				URL:   u,
			})
		}
	}

	return targets
}

type ProbeTarget struct {
	Class string
	URL   string
}
