package registry

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	ScopeProfileRead    = "https://www.googleapis.com/auth/googlehealth.profile.readonly"
	ScopeProfileWrite   = "https://www.googleapis.com/auth/googlehealth.profile.writeonly"
	ScopeSettingsRead   = "https://www.googleapis.com/auth/googlehealth.settings.readonly"
	ScopeSettingsWrite  = "https://www.googleapis.com/auth/googlehealth.settings.writeonly"
	ScopeActivityRead   = "https://www.googleapis.com/auth/googlehealth.activity_and_fitness.readonly"
	ScopeActivityWrite  = "https://www.googleapis.com/auth/googlehealth.activity_and_fitness.writeonly"
	ScopeMetricsRead    = "https://www.googleapis.com/auth/googlehealth.health_metrics_and_measurements.readonly"
	ScopeMetricsWrite   = "https://www.googleapis.com/auth/googlehealth.health_metrics_and_measurements.writeonly"
	ScopeSleepRead      = "https://www.googleapis.com/auth/googlehealth.sleep.readonly"
	ScopeSleepWrite     = "https://www.googleapis.com/auth/googlehealth.sleep.writeonly"
	ScopeNutritionRead  = "https://www.googleapis.com/auth/googlehealth.nutrition.readonly"
	ScopeNutritionWrite = "https://www.googleapis.com/auth/googlehealth.nutrition.writeonly"
	ScopeECGRead        = "https://www.googleapis.com/auth/googlehealth.ecg.readonly"
	ScopeIRNRead        = "https://www.googleapis.com/auth/googlehealth.irn.readonly"
	ScopeLocationRead   = "https://www.googleapis.com/auth/googlehealth.location.readonly"
	ScopeCloudPlatform  = "https://www.googleapis.com/auth/cloud-platform"

	dataTypesSource    = "Google Health data types docs"
	dataTypesSourceURL = "https://developers.google.com/health/data-types"
)

type TimeLiteral string

const (
	TimeLiteralRFC3339  TimeLiteral = "rfc3339"
	TimeLiteralCivil    TimeLiteral = "civilDateTime"
	TimeLiteralDateOnly TimeLiteral = "date"
)

type TimeFilter struct {
	Path               string      `json:"path"`
	Literal            TimeLiteral `json:"literal"`
	SupportsUpperBound bool        `json:"supportsUpperBound"`
}

type DataType struct {
	Name               string      `json:"name"`
	EndpointName       string      `json:"endpointName"`
	FilterName         string      `json:"filterName"`
	RecordType         string      `json:"recordType"`
	Operations         []string    `json:"operations"`
	Scope              string      `json:"scope"`
	ReadScope          string      `json:"readScope"`
	WriteScope         string      `json:"writeScope,omitempty"`
	DefaultTimePath    string      `json:"defaultTimePath,omitempty"`
	TimeFilter         *TimeFilter `json:"timeFilter,omitempty"`
	MaxQueryDays       int         `json:"maxQueryDays,omitempty"`
	FilterRequired     bool        `json:"filterRequired"`
	WebhookSupported   bool        `json:"webhookSupported"`
	TrueZerosSupported bool        `json:"trueZerosSupported"`
	Source             string      `json:"source"`
	SourceURL          string      `json:"sourceURL"`
}

type Operation struct {
	Name     string `json:"name"`
	Method   string `json:"method"`
	Path     string `json:"path"`
	Category string `json:"category"`
}

func ReadOnlyScopes() []string {
	return []string{
		ScopeProfileRead,
		ScopeSettingsRead,
		ScopeActivityRead,
		ScopeMetricsRead,
		ScopeSleepRead,
		ScopeNutritionRead,
		ScopeECGRead,
		ScopeIRNRead,
		ScopeLocationRead,
	}
}

func WriteScopes() []string {
	return []string{ScopeProfileWrite, ScopeSettingsWrite, ScopeActivityWrite, ScopeMetricsWrite, ScopeSleepWrite, ScopeNutritionWrite}
}

func AllScopes() []string {
	return append(ReadOnlyScopes(), WriteScopes()...)
}

func SubscriberScopes() []string {
	return ManagementScopes()
}

func ManagementScopes() []string {
	return []string{ScopeCloudPlatform}
}

func Types() []DataType {
	values := make([]DataType, len(dataTypes))
	for index, dataType := range dataTypes {
		values[index] = cloneDataType(dataType)
	}
	sort.Slice(values, func(i, j int) bool { return values[i].EndpointName < values[j].EndpointName })
	return values
}

func Lookup(name string) (DataType, bool) {
	normalized := strings.TrimSpace(strings.ToLower(name))
	for _, dataType := range dataTypes {
		if dataType.EndpointName == normalized || dataType.FilterName == normalized || strings.EqualFold(dataType.Name, normalized) {
			return cloneDataType(dataType), true
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
		{Name: "projects.subscribers.subscriptions.create", Method: "POST", Path: "/v4/{parent=projects/*/subscribers/*}/subscriptions", Category: "subscriptions"},
		{Name: "projects.subscribers.subscriptions.delete", Method: "DELETE", Path: "/v4/{name=projects/*/subscribers/*/subscriptions/*}", Category: "subscriptions"},
		{Name: "projects.subscribers.subscriptions.list", Method: "GET", Path: "/v4/{parent=projects/*/subscribers/*}/subscriptions", Category: "subscriptions"},
		{Name: "projects.subscribers.subscriptions.patch", Method: "PATCH", Path: "/v4/{subscription.name=projects/*/subscribers/*/subscriptions/*}", Category: "subscriptions"},
		{Name: "users.getIdentity", Method: "GET", Path: "/v4/{name=users/*/identity}", Category: "users"},
		{Name: "users.getIrnProfile", Method: "GET", Path: "/v4/{name=users/*/irnProfile}", Category: "users"},
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
		{Name: "users.pairedDevices.get", Method: "GET", Path: "/v4/{name=users/*/pairedDevices/*}", Category: "pairedDevices"},
		{Name: "users.pairedDevices.list", Method: "GET", Path: "/v4/{parent=users/*}/pairedDevices", Category: "pairedDevices"},
	}
}

func HasOperation(dataType DataType, operation string) bool {
	operation = canonicalOperation(operation)
	for _, item := range dataType.Operations {
		if canonicalOperation(item) == operation {
			return true
		}
	}
	return false
}

func FilterFromRange(dataType DataType, from, to string) string {
	filter, err := BuildRangeFilter(dataType, from, to)
	if err != nil {
		return ""
	}
	return filter
}

func BuildRangeFilter(dataType DataType, from, to string) (string, error) {
	from = strings.TrimSpace(from)
	to = strings.TrimSpace(to)
	if from == "" && to == "" {
		if dataType.FilterRequired {
			return "", fmt.Errorf("%s requires a time filter", dataType.EndpointName)
		}
		return "", nil
	}
	if dataType.TimeFilter == nil {
		return "", fmt.Errorf("%s does not support time-range filters", dataType.EndpointName)
	}
	if to != "" && !dataType.TimeFilter.SupportsUpperBound {
		return "", fmt.Errorf("%s supports only a lower time bound", dataType.EndpointName)
	}

	fromTime, err := validateTimeLiteral(dataType.TimeFilter.Literal, from)
	if err != nil {
		return "", fmt.Errorf("invalid --from for %s: %w", dataType.EndpointName, err)
	}
	toTime, err := validateTimeLiteral(dataType.TimeFilter.Literal, to)
	if err != nil {
		return "", fmt.Errorf("invalid --to for %s: %w", dataType.EndpointName, err)
	}
	if from != "" && to != "" && !fromTime.Before(toTime) {
		return "", fmt.Errorf("time range for %s must end after it starts", dataType.EndpointName)
	}
	if from != "" && to != "" && dataType.MaxQueryDays > 0 {
		maximumRange := time.Duration(dataType.MaxQueryDays) * 24 * time.Hour
		if toTime.Sub(fromTime) > maximumRange {
			return "", fmt.Errorf("time range for %s must not exceed %d days", dataType.EndpointName, dataType.MaxQueryDays)
		}
	}

	parts := make([]string, 0, 2)
	if from != "" {
		parts = append(parts, fmt.Sprintf(`%s >= "%s"`, dataType.TimeFilter.Path, from))
	}
	if to != "" {
		parts = append(parts, fmt.Sprintf(`%s < "%s"`, dataType.TimeFilter.Path, to))
	}
	return strings.Join(parts, " AND "), nil
}

func cloneDataType(dataType DataType) DataType {
	dataType.Operations = append([]string(nil), dataType.Operations...)
	if dataType.TimeFilter != nil {
		filter := *dataType.TimeFilter
		dataType.TimeFilter = &filter
	}
	return dataType
}

func canonicalOperation(operation string) string {
	switch strings.ToLower(strings.TrimSpace(operation)) {
	case "rollup":
		return "rollup"
	case "dailyrollup":
		return "dailyrollup"
	case "patch", "update":
		return "update"
	case "delete", "batchdelete":
		return "batchdelete"
	default:
		return strings.ToLower(strings.TrimSpace(operation))
	}
}

func validateTimeLiteral(literal TimeLiteral, value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	var layouts []string
	switch literal {
	case TimeLiteralRFC3339:
		layouts = []string{time.RFC3339Nano}
	case TimeLiteralCivil:
		layouts = []string{"2006-01-02", "2006-01-02T15:04:05"}
	case TimeLiteralDateOnly:
		layouts = []string{"2006-01-02"}
	default:
		return time.Time{}, fmt.Errorf("unsupported time literal %q", literal)
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("expected %s", literal)
}

func ops(values ...string) []string {
	return values
}

func intervalFilter(filter string) *TimeFilter {
	return &TimeFilter{Path: filter + ".interval.civil_start_time", Literal: TimeLiteralCivil, SupportsUpperBound: true}
}

func sampleFilter(filter string) *TimeFilter {
	return &TimeFilter{Path: filter + ".sample_time.physical_time", Literal: TimeLiteralRFC3339, SupportsUpperBound: true}
}

func dailyFilter(filter string) *TimeFilter {
	return &TimeFilter{Path: filter + ".date", Literal: TimeLiteralDateOnly, SupportsUpperBound: true}
}

func sessionFilter(filter string) *TimeFilter {
	return intervalFilter(filter)
}

func sleepFilter() *TimeFilter {
	return &TimeFilter{Path: "sleep.interval.end_time", Literal: TimeLiteralRFC3339, SupportsUpperBound: true}
}

func electrocardiogramFilter() *TimeFilter {
	return &TimeFilter{Path: "electrocardiogram.interval.start_time", Literal: TimeLiteralRFC3339, SupportsUpperBound: false}
}

func makeDataType(name, endpoint, filter, recordType, scope string, timeFilter *TimeFilter, operations ...string) DataType {
	readScope, writeScope := scopesFor(scope)
	maxQueryDays := 0
	if timeFilter != nil {
		maxQueryDays = 90
		if _, ok := shortQueryRangeDataTypes[endpoint]; ok {
			maxQueryDays = 14
		}
	}
	defaultTimePath := ""
	if timeFilter != nil {
		defaultTimePath = timeFilter.Path
	}
	_, filterRequired := filterRequiredDataTypes[endpoint]
	_, webhookSupported := webhookDataTypes[endpoint]
	_, trueZerosSupported := trueZeroDataTypes[endpoint]
	return DataType{
		Name:               name,
		EndpointName:       endpoint,
		FilterName:         filter,
		RecordType:         recordType,
		Operations:         operations,
		Scope:              scope,
		ReadScope:          readScope,
		WriteScope:         writeScope,
		DefaultTimePath:    defaultTimePath,
		TimeFilter:         timeFilter,
		MaxQueryDays:       maxQueryDays,
		FilterRequired:     filterRequired,
		WebhookSupported:   webhookSupported,
		TrueZerosSupported: trueZerosSupported,
		Source:             dataTypesSource,
		SourceURL:          dataTypesSourceURL,
	}
}

func scopesFor(scope string) (string, string) {
	switch scope {
	case "activity_and_fitness":
		return ScopeActivityRead, ScopeActivityWrite
	case "health_metrics_and_measurements":
		return ScopeMetricsRead, ScopeMetricsWrite
	case "nutrition":
		return ScopeNutritionRead, ScopeNutritionWrite
	case "sleep":
		return ScopeSleepRead, ScopeSleepWrite
	case "ecg":
		return ScopeECGRead, ""
	case "irn":
		return ScopeIRNRead, ""
	default:
		panic("unknown Google Health data type scope: " + scope)
	}
}

var shortQueryRangeDataTypes = map[string]struct{}{
	"active-minutes":              {},
	"calories-in-heart-rate-zone": {},
	"heart-rate":                  {},
	"total-calories":              {},
}

var filterRequiredDataTypes = map[string]struct{}{
	"total-calories": {},
}

var webhookDataTypes = map[string]struct{}{
	"active-zone-minutes":                 {},
	"activity-level":                      {},
	"altitude":                            {},
	"blood-glucose":                       {},
	"body-fat":                            {},
	"calories-in-heart-rate-zone":         {},
	"daily-heart-rate-variability":        {},
	"daily-heart-rate-zones":              {},
	"daily-oxygen-saturation":             {},
	"daily-respiratory-rate":              {},
	"daily-resting-heart-rate":            {},
	"daily-sleep-temperature-derivations": {},
	"distance":                            {},
	"exercise":                            {},
	"floors":                              {},
	"heart-rate":                          {},
	"heart-rate-variability":              {},
	"height":                              {},
	"hydration-log":                       {},
	"nutrition-log":                       {},
	"respiratory-rate-sleep-summary":      {},
	"run-vo2-max":                         {},
	"sedentary-period":                    {},
	"sleep":                               {},
	"steps":                               {},
	"time-in-heart-rate-zone":             {},
	"weight":                              {},
}

var trueZeroDataTypes = map[string]struct{}{
	"altitude":       {},
	"distance":       {},
	"floors":         {},
	"steps":          {},
	"total-calories": {},
}

var dataTypes = []DataType{
	makeDataType("Active Energy Burned", "active-energy-burned", "active_energy_burned", "Interval", "activity_and_fitness", intervalFilter("active_energy_burned"), "list", "reconcile", "rollup", "dailyRollUp"),
	makeDataType("Active Minutes", "active-minutes", "active_minutes", "Interval", "activity_and_fitness", intervalFilter("active_minutes"), "list", "reconcile", "rollup", "dailyRollUp"),
	makeDataType("Active Zone Minutes", "active-zone-minutes", "active_zone_minutes", "Interval", "activity_and_fitness", intervalFilter("active_zone_minutes"), "list", "reconcile", "rollup", "dailyRollUp"),
	makeDataType("Activity Level", "activity-level", "activity_level", "Interval", "activity_and_fitness", intervalFilter("activity_level"), "list", "reconcile"),
	makeDataType("Altitude", "altitude", "altitude", "Interval", "activity_and_fitness", intervalFilter("altitude"), "list", "reconcile", "rollup", "dailyRollUp"),
	makeDataType("Blood Glucose", "blood-glucose", "blood_glucose", "Sample", "health_metrics_and_measurements", sampleFilter("blood_glucose"), "list", "get", "reconcile", "rollup", "dailyRollUp"),
	makeDataType("Body Fat", "body-fat", "body_fat", "Sample", "health_metrics_and_measurements", sampleFilter("body_fat"), "list", "get", "reconcile", "rollup", "dailyRollUp", "create", "update", "batchDelete"),
	makeDataType("Calories In Heart Rate Zone", "calories-in-heart-rate-zone", "calories_in_heart_rate_zone", "Interval", "activity_and_fitness", intervalFilter("calories_in_heart_rate_zone"), "rollup", "dailyRollUp"),
	makeDataType("Core Body Temperature", "core-body-temperature", "core_body_temperature", "Sample", "health_metrics_and_measurements", sampleFilter("core_body_temperature"), "list", "get", "reconcile", "rollup", "dailyRollUp"),
	makeDataType("Daily Heart Rate Variability", "daily-heart-rate-variability", "daily_heart_rate_variability", "Daily", "health_metrics_and_measurements", dailyFilter("daily_heart_rate_variability"), "list", "reconcile"),
	makeDataType("Daily Heart Rate Zones", "daily-heart-rate-zones", "daily_heart_rate_zones", "Daily", "health_metrics_and_measurements", dailyFilter("daily_heart_rate_zones"), "list", "reconcile"),
	makeDataType("Daily Oxygen Saturation", "daily-oxygen-saturation", "daily_oxygen_saturation", "Daily", "health_metrics_and_measurements", dailyFilter("daily_oxygen_saturation"), "list", "reconcile"),
	makeDataType("Daily Respiratory Rate", "daily-respiratory-rate", "daily_respiratory_rate", "Daily", "health_metrics_and_measurements", dailyFilter("daily_respiratory_rate"), "list", "reconcile"),
	makeDataType("Daily Resting Heart Rate", "daily-resting-heart-rate", "daily_resting_heart_rate", "Daily", "health_metrics_and_measurements", dailyFilter("daily_resting_heart_rate"), "list", "reconcile"),
	makeDataType("Daily Sleep Temperature Derivations", "daily-sleep-temperature-derivations", "daily_sleep_temperature_derivations", "Daily", "health_metrics_and_measurements", dailyFilter("daily_sleep_temperature_derivations"), "list", "reconcile"),
	makeDataType("Daily VO2 Max", "daily-vo2-max", "daily_vo2_max", "Daily", "activity_and_fitness", dailyFilter("daily_vo2_max"), "list", "reconcile"),
	makeDataType("Distance", "distance", "distance", "Interval", "activity_and_fitness", intervalFilter("distance"), "list", "reconcile", "rollup", "dailyRollUp"),
	makeDataType("Electrocardiogram (ECG)", "electrocardiogram", "electrocardiogram", "Session", "ecg", electrocardiogramFilter(), "list"),
	makeDataType("Exercise", "exercise", "exercise", "Session", "activity_and_fitness", sessionFilter("exercise"), "list", "get", "reconcile", "create", "update", "batchDelete"),
	makeDataType("Floors", "floors", "floors", "Interval", "activity_and_fitness", intervalFilter("floors"), "reconcile", "rollup", "dailyRollUp"),
	makeDataType("Food", "food", "food", "Food", "nutrition", nil, "list", "get"),
	makeDataType("Food Measurement Unit", "food-measurement-unit", "food_measurement_unit", "Food", "nutrition", nil, "list", "get"),
	makeDataType("Heart Rate", "heart-rate", "heart_rate", "Sample", "health_metrics_and_measurements", sampleFilter("heart_rate"), "list", "reconcile", "rollup", "dailyRollUp"),
	makeDataType("Heart Rate Variability", "heart-rate-variability", "heart_rate_variability", "Sample", "health_metrics_and_measurements", sampleFilter("heart_rate_variability"), "list", "reconcile"),
	makeDataType("Height", "height", "height", "Sample", "health_metrics_and_measurements", sampleFilter("height"), "list", "get", "reconcile", "create", "update", "batchDelete"),
	makeDataType("Hydration Log", "hydration-log", "hydration_log", "Session", "nutrition", sessionFilter("hydration_log"), "list", "get", "reconcile", "rollup", "dailyRollUp", "create", "update", "batchDelete"),
	makeDataType("Irregular Rhythm Notification", "irregular-rhythm-notification", "irregular_rhythm_notification", "Session", "irn", sessionFilter("irregular_rhythm_notification"), "list"),
	makeDataType("Nutrition Log", "nutrition-log", "nutrition_log", "Sample", "nutrition", sessionFilter("nutrition_log"), "list", "get", "reconcile", "rollup", "dailyRollUp", "create", "update", "batchDelete"),
	makeDataType("Oxygen Saturation", "oxygen-saturation", "oxygen_saturation", "Sample", "health_metrics_and_measurements", sampleFilter("oxygen_saturation"), "list", "reconcile"),
	makeDataType("Respiratory Rate Sleep Summary", "respiratory-rate-sleep-summary", "respiratory_rate_sleep_summary", "Sample", "health_metrics_and_measurements", sampleFilter("respiratory_rate_sleep_summary"), "list", "reconcile"),
	makeDataType("Run VO2 Max", "run-vo2-max", "run_vo2_max", "Sample", "activity_and_fitness", sampleFilter("run_vo2_max"), "list", "reconcile", "rollup", "dailyRollUp"),
	makeDataType("Sedentary Period", "sedentary-period", "sedentary_period", "Interval", "activity_and_fitness", intervalFilter("sedentary_period"), "list", "reconcile", "rollup", "dailyRollUp"),
	makeDataType("Sleep", "sleep", "sleep", "Session", "sleep", sleepFilter(), "list", "get", "reconcile", "create", "update", "batchDelete"),
	makeDataType("Steps", "steps", "steps", "Interval", "activity_and_fitness", intervalFilter("steps"), "list", "reconcile", "rollup", "dailyRollUp"),
	makeDataType("Swim Lengths Data", "swim-lengths-data", "swim_lengths_data", "Interval", "activity_and_fitness", intervalFilter("swim_lengths_data"), "list", "reconcile", "rollup", "dailyRollUp"),
	makeDataType("Time in Heart Rate Zone", "time-in-heart-rate-zone", "time_in_heart_rate_zone", "Interval", "activity_and_fitness", intervalFilter("time_in_heart_rate_zone"), "list", "reconcile", "rollup", "dailyRollUp"),
	makeDataType("Total Calories", "total-calories", "total_calories", "Interval", "activity_and_fitness", intervalFilter("total_calories"), "rollup", "dailyRollUp"),
	makeDataType("VO2 Max", "vo2-max", "vo2_max", "Sample", "activity_and_fitness", sampleFilter("vo2_max"), "list", "reconcile"),
	makeDataType("Weight", "weight", "weight", "Sample", "health_metrics_and_measurements", sampleFilter("weight"), "list", "get", "reconcile", "rollup", "dailyRollUp", "create", "update", "batchDelete"),
}
