package metrics

import "github.com/prometheus/client_golang/prometheus"

// AdminActionsTotal tracks administrative CRUD operations and their outcomes.
var AdminActionsTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "gateway_admin_actions_total",
		Help: "Total number of admin actions grouped by action and outcome.",
	},
	[]string{"action", "outcome"},
)

func init() {
	prometheus.MustRegister(AdminActionsTotal)
}

// ObserveAdminAction records the outcome of an admin action.
func ObserveAdminAction(action string, success bool) {
	outcome := "success"
	if !success {
		outcome = "error"
	}
	AdminActionsTotal.WithLabelValues(action, outcome).Inc()
}
