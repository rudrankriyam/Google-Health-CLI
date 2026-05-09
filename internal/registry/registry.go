package registry

import (
	"fmt"
	"sort"
	"strings"
)

const (
	ScopeProfileRead    = "https://www.googleapis.com/auth/googlehealth.profile.readonly"
	ScopeProfileWrite   = "https://www.googleapis.com/auth/googlehealth.profile"
	ScopeSettingsRead   = "https://www.googleapis.com/auth/googlehealth.settings.readonly"
	ScopeSettingsWrite  = "https://www.googleapis.com/auth/googlehealth.settings"
	ScopeActivityRead   = "https://www.googleapis.com/auth/googlehealth.activity_and_fitness.readonly"
	ScopeActivityWrite  = "https://www.googleapis.com/auth/googlehealth.activity_and_fitness"
	ScopeMetricsRead    = "https://www.googleapis.com/auth/googlehealth.health_metrics_and_measurements.readonly"
	ScopeMetricsWrite   = "https://www.googleapis.com/auth/googlehealth.health_metrics_and_measurements"
	ScopeSleepRead      = "https://www.googleapis.com/auth/googlehealth.sleep.readonly"
	ScopeSleepWrite     = "https://www.googleapis.com/auth/googlehealth.sleep"
	ScopeNutritionRead  = "https://www.googleapis.com/auth/googlehealth.nutrition.readonly"
	ScopeNutritionWrite = "https://www.googleapis.com/auth/googlehealth.nutrition"
)

type DataType struct {
	Name            string   `json:"name"`
	EndpointName    string   `json:"endpointName"`
	FilterName      string   `json:"filterName"`
	RecordType      string   `json:"recordType"`
	Operations      []string `json:"operations"`
	Scope           string   `json:"scope"`
	DefaultTimePath string   `json:"defaultTimePath,omitempty"`
	Source          string   `json:"source"`
}

type Operation struct {
	Name     string `json:"name"`
	Method   string `json:"method"`
	Path     string `json:"path"`
	Category string `json:"category"`
}

func ReadOnlyScopes() []string {
	return []string{ScopeProfileRead, ScopeSettingsRead, ScopeActivityRead, ScopeMetricsRead, ScopeSleepRead, ScopeNutritionRead}
}

func WriteScopes() []string {
	return []string{ScopeProfileWrite, ScopeSettingsWrite, ScopeActivityWrite, ScopeMetricsWrite, ScopeSleepWrite, ScopeNutritionWrite}
}

func AllScopes() []string {
	return append(ReadOnlyScopes(), WriteScopes()...)
}

func Types() []DataType {
	values := append([]DataType(nil), dataTypes...)
	sort.Slice(values, func(i, j int) bool { return values[i].EndpointName < values[j].EndpointName })
	return values
}

func Lookup(name string) (DataType, bool) {
	normalized := strings.TrimSpace(strings.ToLower(name))
	for _, dataType := range dataTypes {
		if dataType.EndpointName == normalized || dataType.FilterName == normalized || strings.EqualFold(dataType.Name, normalized) {
			return dataType, true
		}
	}
	return DataType{}, false
}

func MustLookup(name string) (DataType, error) {
	dataType, ok := Lookup(name)
	if !ok {
		return DataType{}, fmt.Errorf("unknown data type %q; run `ghealth types list`", name)
	}
	return dataType, nil
}

func RESTOperations() []Operation {
	return []Operation{
		{Name: "projects.subscribers.create", Method: "POST", Path: "/v4/{parent=projects/*}/subscribers", Category: "subscribers"},
		{Name: "projects.subscribers.delete", Method: "DELETE", Path: "/v4/{name=projects/*/subscribers/*}", Category: "subscribers"},
		{Name: "projects.subscribers.list", Method: "GET", Path: "/v4/{parent=projects/*}/subscribers", Category: "subscribers"},
		{Name: "projects.subscribers.patch", Method: "PATCH", Path: "/v4/{subscriber.name=projects/*/subscribers/*}", Category: "subscribers"},
		{Name: "users.getIdentity", Method: "GET", Path: "/v4/{name=users/*/identity}", Category: "users"},
		{Name: "users.getProfile", Method: "GET", Path: "/v4/{name=users/*/profile}", Category: "users"},
		{Name: "users.getSettings", Method: "GET", Path: "/v4/{name=users/*/settings}", Category: "users"},
		{Name: "users.updateProfile", Method: "PATCH", Path: "/v4/{profile.name=users/*/profile}", Category: "users"},
		{Name: "users.updateSettings", Method: "PATCH", Path: "/v4/{settings.name=users/*/settings}", Category: "users"},
		{Name: "users.dataTypes.dataPoints.batchDelete", Method: "POST", Path: "/v4/{parent=users/*/dataTypes/*}/dataPoints:batchDelete", Category: "dataPoints"},
		{Name: "users.dataTypes.dataPoints.create", Method: "POST", Path: "/v4/{parent=users/*/dataTypes/*}/dataPoints", Category: "dataPoints"},
		{Name: "users.dataTypes.dataPoints.dailyRollUp", Method: "POST", Path: "/v4/{parent=users/*/dataTypes/*}/dataPoints:dailyRollUp", Category: "dataPoints"},
		{Name: "users.dataTypes.dataPoints.exportExerciseTcx", Method: "GET", Path: "/v4/{name=users/*/dataTypes/*/dataPoints/*}:exportExerciseTcx", Category: "dataPoints"},
		{Name: "users.dataTypes.dataPoints.get", Method: "GET", Path: "/v4/{name=users/*/dataTypes/*/dataPoints/*}", Category: "dataPoints"},
		{Name: "users.dataTypes.dataPoints.list", Method: "GET", Path: "/v4/{parent=users/*/dataTypes/*}/dataPoints", Category: "dataPoints"},
		{Name: "users.dataTypes.dataPoints.patch", Method: "PATCH", Path: "/v4/{dataPoint.name=users/*/dataTypes/*/dataPoints/*}", Category: "dataPoints"},
		{Name: "users.dataTypes.dataPoints.reconcile", Method: "GET", Path: "/v4/{parent=users/*/dataTypes/*}/dataPoints:reconcile", Category: "dataPoints"},
		{Name: "users.dataTypes.dataPoints.rollUp", Method: "POST", Path: "/v4/{parent=users/*/dataTypes/*}/dataPoints:rollUp", Category: "dataPoints"},
	}
}

func HasOperation(dataType DataType, operation string) bool {
	for _, item := range dataType.Operations {
		if item == operation {
			return true
		}
	}
	return false
}

func FilterFromRange(dataType DataType, from, to string) string {
	field := dataType.DefaultTimePath
	if field == "" || strings.TrimSpace(from) == "" {
		return ""
	}
	parts := []string{fmt.Sprintf(`%s >= "%s"`, field, from)}
	if strings.TrimSpace(to) != "" {
		parts = append(parts, fmt.Sprintf(`%s < "%s"`, field, to))
	}
	return strings.Join(parts, " AND ")
}

func ops(values ...string) []string {
	return values
}

func intervalPath(filter string) string {
	return filter + ".interval.civil_start_time"
}

func samplePath(filter string) string {
	return filter + ".sample_time.physical_time"
}

func dailyPath(filter string) string {
	return filter + ".date"
}

var dataTypes = []DataType{
	{Name: "Active Minutes", EndpointName: "active-minutes", FilterName: "active_minutes", RecordType: "Interval", Operations: ops("reconcile", "rollup", "dailyRollUp"), Scope: "activity_and_fitness", DefaultTimePath: intervalPath("active_minutes"), Source: "Google Health data types docs"},
	{Name: "Active Zone Minutes", EndpointName: "active-zone-minutes", FilterName: "active_zone_minutes", RecordType: "Interval", Operations: ops("list", "reconcile", "rollup", "dailyRollUp"), Scope: "activity_and_fitness", DefaultTimePath: intervalPath("active_zone_minutes"), Source: "Google Health data types docs"},
	{Name: "Activity Level", EndpointName: "activity-level", FilterName: "activity_level", RecordType: "Interval", Operations: ops("list", "reconcile"), Scope: "activity_and_fitness", DefaultTimePath: intervalPath("activity_level"), Source: "Google Health data types docs"},
	{Name: "Altitude", EndpointName: "altitude", FilterName: "altitude", RecordType: "Interval", Operations: ops("list", "reconcile", "rollup", "dailyRollUp"), Scope: "activity_and_fitness", DefaultTimePath: intervalPath("altitude"), Source: "Google Health data types docs"},
	{Name: "Body Fat", EndpointName: "body-fat", FilterName: "body_fat", RecordType: "Sample", Operations: ops("list", "get", "reconcile", "rollup", "dailyRollUp", "create", "update", "batchDelete"), Scope: "health_metrics_and_measurements", DefaultTimePath: samplePath("body_fat"), Source: "Google Health data types docs"},
	{Name: "Calories In Heart Rate Zone", EndpointName: "calories-in-heart-rate-zone", FilterName: "calories_in_heart_rate_zone", RecordType: "Interval", Operations: ops("rollup", "dailyRollUp"), Scope: "activity_and_fitness", DefaultTimePath: intervalPath("calories_in_heart_rate_zone"), Source: "Google Health data types docs"},
	{Name: "Daily Heart Rate Variability", EndpointName: "daily-heart-rate-variability", FilterName: "daily_heart_rate_variability", RecordType: "Daily", Operations: ops("list", "reconcile"), Scope: "health_metrics_and_measurements", DefaultTimePath: dailyPath("daily_heart_rate_variability"), Source: "Google Health data types docs"},
	{Name: "Daily Heart Rate Zones", EndpointName: "daily-heart-rate-zones", FilterName: "daily_heart_rate_zones", RecordType: "Daily", Operations: ops("reconcile"), Scope: "health_metrics_and_measurements", DefaultTimePath: dailyPath("daily_heart_rate_zones"), Source: "Google Health data types docs"},
	{Name: "Daily Oxygen Saturation", EndpointName: "daily-oxygen-saturation", FilterName: "daily_oxygen_saturation", RecordType: "Daily", Operations: ops("list", "reconcile"), Scope: "health_metrics_and_measurements", DefaultTimePath: dailyPath("daily_oxygen_saturation"), Source: "Google Health data types docs"},
	{Name: "Daily Respiratory Rate", EndpointName: "daily-respiratory-rate", FilterName: "daily_respiratory_rate", RecordType: "Daily", Operations: ops("list", "reconcile"), Scope: "health_metrics_and_measurements", DefaultTimePath: dailyPath("daily_respiratory_rate"), Source: "Google Health data types docs"},
	{Name: "Daily Resting Heart Rate", EndpointName: "daily-resting-heart-rate", FilterName: "daily_resting_heart_rate", RecordType: "Daily", Operations: ops("list", "reconcile"), Scope: "health_metrics_and_measurements", DefaultTimePath: dailyPath("daily_resting_heart_rate"), Source: "Google Health data types docs"},
	{Name: "Daily Sleep Temperature Derivations", EndpointName: "daily-sleep-temperature-derivations", FilterName: "daily_sleep_temperature_derivations", RecordType: "Daily", Operations: ops("list", "reconcile"), Scope: "health_metrics_and_measurements", DefaultTimePath: dailyPath("daily_sleep_temperature_derivations"), Source: "Google Health data types docs"},
	{Name: "Daily VO2 Max", EndpointName: "daily-vo2-max", FilterName: "daily_vo2_max", RecordType: "Daily", Operations: ops("list", "reconcile"), Scope: "activity_and_fitness", DefaultTimePath: dailyPath("daily_vo2_max"), Source: "Google Health data types docs"},
	{Name: "Distance", EndpointName: "distance", FilterName: "distance", RecordType: "Interval", Operations: ops("list", "reconcile", "rollup", "dailyRollUp"), Scope: "activity_and_fitness", DefaultTimePath: intervalPath("distance"), Source: "Google Health data types docs"},
	{Name: "Exercise", EndpointName: "exercise", FilterName: "exercise", RecordType: "Session", Operations: ops("list", "get", "reconcile", "create", "update", "batchDelete"), Scope: "activity_and_fitness", DefaultTimePath: intervalPath("exercise"), Source: "Google Health data types docs"},
	{Name: "Floors", EndpointName: "floors", FilterName: "floors", RecordType: "Interval", Operations: ops("reconcile", "rollup", "dailyRollUp"), Scope: "activity_and_fitness", DefaultTimePath: intervalPath("floors"), Source: "Google Health data types docs"},
	{Name: "Heart Rate", EndpointName: "heart-rate", FilterName: "heart_rate", RecordType: "Sample", Operations: ops("list", "reconcile", "rollup", "dailyRollUp"), Scope: "health_metrics_and_measurements", DefaultTimePath: samplePath("heart_rate"), Source: "Google Health data types docs"},
	{Name: "Heart Rate Variability", EndpointName: "heart-rate-variability", FilterName: "heart_rate_variability", RecordType: "Sample", Operations: ops("list", "reconcile"), Scope: "health_metrics_and_measurements", DefaultTimePath: samplePath("heart_rate_variability"), Source: "Google Health data types docs"},
	{Name: "Height", EndpointName: "height", FilterName: "height", RecordType: "Sample", Operations: ops("list", "get", "reconcile", "create", "update", "batchDelete"), Scope: "health_metrics_and_measurements", DefaultTimePath: samplePath("height"), Source: "Google Health data types docs"},
	{Name: "Hydration Log", EndpointName: "hydration-log", FilterName: "hydration_log", RecordType: "Session", Operations: ops("list", "get", "reconcile", "rollup", "dailyRollUp", "create", "update", "batchDelete"), Scope: "nutrition", DefaultTimePath: intervalPath("hydration_log"), Source: "Google Health data types docs"},
	{Name: "Oxygen Saturation", EndpointName: "oxygen-saturation", FilterName: "oxygen_saturation", RecordType: "Sample", Operations: ops("list", "reconcile"), Scope: "health_metrics_and_measurements", DefaultTimePath: samplePath("oxygen_saturation"), Source: "Google Health data types docs"},
	{Name: "Respiratory Rate Sleep Summary", EndpointName: "respiratory-rate-sleep-summary", FilterName: "respiratory_rate_sleep_summary", RecordType: "Sample", Operations: ops("list", "reconcile"), Scope: "health_metrics_and_measurements", DefaultTimePath: samplePath("respiratory_rate_sleep_summary"), Source: "Google Health data types docs"},
	{Name: "Run VO2 Max", EndpointName: "run-vo2-max", FilterName: "run_vo2_max", RecordType: "Sample", Operations: ops("list", "reconcile", "rollup", "dailyRollUp"), Scope: "activity_and_fitness", DefaultTimePath: samplePath("run_vo2_max"), Source: "Google Health data types docs"},
	{Name: "Sedentary Period", EndpointName: "sedentary-period", FilterName: "sedentary_period", RecordType: "Interval", Operations: ops("list", "reconcile", "rollup", "dailyRollUp"), Scope: "activity_and_fitness", DefaultTimePath: intervalPath("sedentary_period"), Source: "Google Health data types docs"},
	{Name: "Sleep", EndpointName: "sleep", FilterName: "sleep", RecordType: "Session", Operations: ops("list", "get", "reconcile", "create", "update", "batchDelete"), Scope: "sleep", DefaultTimePath: intervalPath("sleep"), Source: "Google Health data types docs"},
	{Name: "Steps", EndpointName: "steps", FilterName: "steps", RecordType: "Interval", Operations: ops("list", "reconcile", "rollup", "dailyRollUp"), Scope: "activity_and_fitness", DefaultTimePath: intervalPath("steps"), Source: "Google Health data types docs"},
	{Name: "Swim Lengths Data", EndpointName: "swim-lengths-data", FilterName: "swim_lengths_data", RecordType: "Interval", Operations: ops("list", "reconcile", "rollup", "dailyRollUp"), Scope: "activity_and_fitness", DefaultTimePath: intervalPath("swim_lengths_data"), Source: "Google Health data types docs"},
	{Name: "Time in Heart Rate Zone", EndpointName: "time-in-heart-rate-zone", FilterName: "time_in_heart_rate_zone", RecordType: "Interval", Operations: ops("reconcile", "rollup", "dailyRollUp"), Scope: "activity_and_fitness", DefaultTimePath: intervalPath("time_in_heart_rate_zone"), Source: "Google Health data types docs"},
	{Name: "Total Calories", EndpointName: "total-calories", FilterName: "total_calories", RecordType: "Interval", Operations: ops("rollup", "dailyRollUp"), Scope: "activity_and_fitness", DefaultTimePath: intervalPath("total_calories"), Source: "Google Health data types docs"},
	{Name: "VO2 Max", EndpointName: "vo2-max", FilterName: "vo2_max", RecordType: "Sample", Operations: ops("list", "reconcile"), Scope: "activity_and_fitness", DefaultTimePath: samplePath("vo2_max"), Source: "Google Health data types docs"},
	{Name: "Weight", EndpointName: "weight", FilterName: "weight", RecordType: "Sample", Operations: ops("list", "get", "reconcile", "rollup", "dailyRollUp", "create", "update", "batchDelete"), Scope: "health_metrics_and_measurements", DefaultTimePath: samplePath("weight"), Source: "Google Health data types docs"},
}
